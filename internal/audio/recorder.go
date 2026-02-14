package audio

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

// Recorder captures microphone audio using arecord.
// Output format: 16-bit signed little-endian PCM, 16000 Hz, mono.
type Recorder struct {
	cmd    *exec.Cmd
	cancel context.CancelFunc
}

// Start begins recording and returns a channel of raw PCM audio chunks.
// The channel is closed when recording stops.
func (r *Recorder) Start(ctx context.Context) (<-chan []byte, error) {
	ctx, cancel := context.WithCancel(ctx)
	r.cancel = cancel

	r.cmd = exec.CommandContext(ctx, "arecord",
		"-r", "16000",
		"-f", "S16_LE",
		"-c", "1",
		"-t", "raw",
		"-q",
	)

	stdout, err := r.cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("creating arecord pipe: %w", err)
	}

	if err := r.cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("starting arecord: %w", err)
	}

	ch := make(chan []byte, 16)
	go func() {
		defer close(ch)
		buf := make([]byte, 4096)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				chunk := make([]byte, n)
				copy(chunk, buf[:n])
				select {
				case ch <- chunk:
				case <-ctx.Done():
					return
				}
			}
			if err != nil {
				if err != io.EOF {
					// arecord exited or context canceled
				}
				return
			}
		}
	}()

	return ch, nil
}

// Stop terminates the recording.
func (r *Recorder) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
}
