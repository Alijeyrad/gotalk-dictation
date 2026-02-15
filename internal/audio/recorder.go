package audio

import (
	"context"
	"fmt"
	"os/exec"
)

type Recorder struct {
	cmd    *exec.Cmd
	cancel context.CancelFunc
}

func (r *Recorder) Start(ctx context.Context) (<-chan []byte, error) {
	// Stop any previous session before starting a new one so its arecord
	// process and goroutine are not orphaned.
	r.Stop()

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

	cmd := r.cmd // capture before r.cmd may be overwritten
	ch := make(chan []byte, 16)
	go func() {
		defer close(ch)
		defer cmd.Wait() //nolint:errcheck // reap the process to avoid zombies
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
				return
			}
		}
	}()

	return ch, nil
}

func (r *Recorder) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
}
