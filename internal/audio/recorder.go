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
	chunk := make([]byte, len(buf))
	copy(chunk, buf)
	select {
	case w.ch <- chunk:
		return len(buf), nil
	case <-w.ctx.Done():
		return 0, w.ctx.Err()
	}
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
		stream.Stop()
		stream.Close()
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
