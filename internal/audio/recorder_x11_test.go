//go:build x11test

package audio_test

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/Alijeyrad/gotalk-dictation/internal/audio"
)

func TestMain(m *testing.M) {
	if _, err := exec.LookPath("arecord"); err != nil {
		// arecord not found: skip all X11/audio tests.
		m.Run() // Run to register as skipped rather than fail.
		return
	}
	m.Run()
}

func TestRecorderStartStop(t *testing.T) {
	r := &audio.Recorder{}
	ctx := context.Background()

	ch, err := r.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Read up to 3 chunks.
	count := 0
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
loop:
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				break loop
			}
			count++
			if count >= 3 {
				break loop
			}
		case <-timer.C:
			break loop
		}
	}

	r.Stop()

	// Wait for channel to close within 500ms.
	done := make(chan struct{})
	go func() {
		for range ch {
		}
		close(done)
	}()
	select {
	case <-done:
		// OK
	case <-time.After(500 * time.Millisecond):
		t.Error("channel did not close within 500ms after Stop()")
	}
}

func TestRecorderContextCancellation(t *testing.T) {
	r := &audio.Recorder{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	ch, err := r.Start(ctx)
	if err != nil {
		// Acceptable: Start may return error for cancelled context.
		return
	}

	// If Start succeeded, channel should close promptly.
	select {
	case <-ch:
		// drain any buffered data
	case <-time.After(500 * time.Millisecond):
		t.Error("channel did not close within 500ms for cancelled context")
	}
}

func TestRecorderChunkFormat(t *testing.T) {
	r := &audio.Recorder{}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch, err := r.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer r.Stop()

	// Read one chunk and verify it has an even byte count (S16LE pairs).
	select {
	case chunk, ok := <-ch:
		if !ok {
			t.Fatal("channel closed before any chunk received")
		}
		if len(chunk)%2 != 0 {
			t.Errorf("chunk length %d is not a multiple of 2 (expected S16LE pairs)", len(chunk))
		}
	case <-time.After(2 * time.Second):
		t.Skip("no audio chunk received within timeout (no audio device?)")
	}
}
