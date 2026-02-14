package main

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/Alijeyrad/gotalk-dictation/internal/audio"
	"github.com/Alijeyrad/gotalk-dictation/internal/config"
	"github.com/Alijeyrad/gotalk-dictation/internal/hotkey"
	"github.com/Alijeyrad/gotalk-dictation/internal/speech"
	"github.com/Alijeyrad/gotalk-dictation/internal/typing"
	"github.com/Alijeyrad/gotalk-dictation/internal/ui"
)

type app struct {
	cfg        *config.Config
	recorder   *audio.Recorder
	recognizer *speech.Recognizer
	typer      *typing.Typer
	tray       *ui.Tray

	mu          sync.Mutex
	isListening bool
	cancelDicta context.CancelFunc
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Printf("warning: failed to load config (%v), using defaults", err)
		cfg = config.Default()
	}

	if speech.HasCloudCredentials() {
		log.Println("speech: using Google Cloud Speech API (credentials found)")
	} else {
		log.Println("speech: using free Google Speech API (no credentials needed)")
	}

	a := &app{
		cfg:        cfg,
		recorder:   &audio.Recorder{},
		recognizer: &speech.Recognizer{Language: cfg.Language},
		typer:      &typing.Typer{EnablePunctuation: cfg.EnablePunctuation},
		tray:       &ui.Tray{},
	}

	// Register global hotkey (non-fatal). Done before tray.Run so we know the
	// status; the tray will surface any error once it's initialized.
	var hotkeyErr error
	hkm, err := hotkey.New(cfg.Hotkey)
	if err != nil {
		hotkeyErr = err
		log.Printf("WARNING: hotkey init failed: %v", err)
	} else {
		if err := hkm.Register(a.toggleDictation); err != nil {
			hotkeyErr = err
			log.Printf("WARNING: hotkey %q is already grabbed by another app.\n"+
				"  → Open your DE keyboard settings, find the Alt+D shortcut\n"+
				"    (probably your old dictation script), remove it, then restart.", cfg.Hotkey)
		}
		defer hkm.Stop()
	}

	// Run system tray + popup — blocks until quit (must be on main goroutine).
	// Pass the hotkey error so the tray can surface it after initializing.
	a.tray.Run(a.toggleDictation, func() {
		a.mu.Lock()
		cancel := a.cancelDicta
		a.mu.Unlock()
		if cancel != nil {
			cancel()
		}
	}, hotkeyErr)
}

func (a *app) toggleDictation() {
	a.mu.Lock()
	listening := a.isListening
	a.mu.Unlock()

	if listening {
		a.stopDictation()
	} else {
		go a.startDictation()
	}
}

func (a *app) startDictation() {
	a.mu.Lock()
	if a.isListening {
		a.mu.Unlock()
		return
	}
	a.isListening = true
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(a.cfg.Timeout)*time.Second)
	a.cancelDicta = cancel
	a.mu.Unlock()

	defer func() {
		cancel()
		a.mu.Lock()
		a.isListening = false
		a.cancelDicta = nil
		a.mu.Unlock()
	}()

	a.tray.SetListening()

	audioCh, err := a.recorder.Start(ctx)
	if err != nil {
		a.tray.SetError("Mic error: " + err.Error())
		log.Printf("recorder error: %v", err)
		return
	}

	a.tray.SetProcessing()
	text, err := a.recognizer.Recognize(ctx, audioCh)
	a.recorder.Stop()

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			a.tray.SetError("Timeout — no speech detected")
		} else if ctx.Err() == context.Canceled {
			a.tray.SetIdle()
		} else {
			a.tray.SetError(err.Error())
			log.Printf("recognizer error: %v", err)
		}
		return
	}

	if text == "" {
		a.tray.SetError("Could not understand speech")
		return
	}

	if err := a.typer.Type(text); err != nil {
		a.tray.SetError("Type error: " + err.Error())
		log.Printf("typer error: %v", err)
		return
	}

	a.tray.SetDone(text)
}

func (a *app) stopDictation() {
	a.mu.Lock()
	cancel := a.cancelDicta
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}
