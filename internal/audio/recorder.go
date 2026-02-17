package audio

import (
	"context"
	"fmt"

	"github.com/jfreymuth/pulse"
	"github.com/jfreymuth/pulse/proto"
)

type Recorder struct {
	client *pulse.Client
	stream *pulse.RecordStream
	cancel context.CancelFunc
}

// rawWriter implements pulse.Writer, forwarding raw S16_LE bytes to a channel.
type rawWriter struct {
	ch  chan<- []byte
	ctx context.Context
}

func (w *rawWriter) Write(buf []byte) (int, error) {
	// Check cancellation first without blocking.
	select {
	case <-w.ctx.Done():
		return 0, w.ctx.Err()
	default:
	}
	chunk := make([]byte, len(buf))
	copy(chunk, buf)
	// Non-blocking send: drop the chunk if the consumer is busy (e.g. during
	// the API call). This must never block because Write is called from the
	// pulse dispatch goroutine — blocking it prevents Request() replies and
	// causes a deadlock when Stop/Close are called.
	select {
	case w.ch <- chunk:
	default:
	}
	return len(buf), nil
}

func (w *rawWriter) Format() byte { return proto.FormatInt16LE }

func (r *Recorder) Start(ctx context.Context) (<-chan []byte, error) {
	r.Stop()

	ctx, cancel := context.WithCancel(ctx)
	r.cancel = cancel

	client, err := pulse.NewClient(
		pulse.ClientApplicationName("GoTalk Dictation"),
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("connecting to PulseAudio: %w", err)
	}

	ch := make(chan []byte, 16)
	w := &rawWriter{ch: ch, ctx: ctx}

	stream, err := client.NewRecord(w,
		pulse.RecordMono,
		pulse.RecordSampleRate(16000),
		// ~62 ms chunks (2000 bytes at 16 kHz/16-bit mono) — keeps VAD
		// responsive and matches the old arecord behaviour.
		pulse.RecordBufferFragmentSize(2000),
		pulse.RecordMediaName("GoTalk Dictation"),
	)
	if err != nil {
		client.Close()
		cancel()
		return nil, fmt.Errorf("creating record stream: %w", err)
	}

	stream.Start()
	r.client = client
	r.stream = stream

	go func() {
		defer close(ch)
		<-ctx.Done()
		// Close the socket directly. Calling stream.Stop() or stream.Close()
		// sends PA requests via Request(), which blocks waiting for a reply
		// from the dispatch goroutine. If Write() ever stalled the dispatch
		// goroutine (even transiently), that would deadlock. client.Close()
		// closes the socket, causing the dispatch goroutine to exit cleanly.
		client.Close()
	}()

	return ch, nil
}

func (r *Recorder) Stop() {
	if r.cancel != nil {
		r.cancel()
		r.cancel = nil
	}
}
