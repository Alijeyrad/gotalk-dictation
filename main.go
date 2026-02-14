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
	cfgMu      sync.RWMutex
	cfg        *config.Config
	recorder   *audio.Recorder
	recognizer *speech.Recognizer
	typer      *typing.Typer
	tray       *ui.Tray

	hkmMu sync.Mutex
	hkm   *hotkey.Manager

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

	if speech.HasCloudCredentials() || cfg.UseAdvancedAPI {
		log.Println("speech: using Google Cloud Speech API")
	} else {
		log.Println("speech: using free Google Speech API (no credentials needed)")
	}

	a := &app{
		cfg:      cfg,
		recorder: &audio.Recorder{},
		typer:    &typing.Typer{EnablePunctuation: cfg.EnablePunctuation},
		tray:     &ui.Tray{},
	}
	a.recognizer = buildRecognizer(cfg)

	var startupErr error
	hkm, err := hotkey.New(cfg.Hotkey)
	if err != nil {
		startupErr = err
		log.Printf("WARNING: hotkey init failed: %v", err)
	} else {
		if err := hkm.Register(a.toggleDictation); err != nil {
			startupErr = err
			log.Printf("WARNING: hotkey %q already grabbed — remove old DE shortcut", cfg.Hotkey)
		} else {
			a.hkmMu.Lock()
			a.hkm = hkm
			a.hkmMu.Unlock()
		}
	}

	a.tray.OnSettingsSave = func(newCfg *config.Config) {
		log.Printf("settings save: hotkey=%q language=%q timeout=%d silenceChunks=%d sensitivity=%.1f punctuation=%v advancedAPI=%v",
			newCfg.Hotkey, newCfg.Language, newCfg.Timeout, newCfg.SilenceChunks, newCfg.Sensitivity, newCfg.EnablePunctuation, newCfg.UseAdvancedAPI)
		if err := newCfg.Save(); err != nil {
			log.Printf("warning: failed to save config: %v", err)
		}

		a.cfgMu.RLock()
		oldHotkey := a.cfg.Hotkey
		a.cfgMu.RUnlock()

		a.cfgMu.Lock()
		a.cfg = newCfg
		a.recognizer = buildRecognizer(newCfg)
		a.typer = &typing.Typer{EnablePunctuation: newCfg.EnablePunctuation}
		a.cfgMu.Unlock()
		a.tray.UpdateConfig(newCfg)

		if newCfg.Hotkey != oldHotkey {
			a.rebindHotkey(newCfg.Hotkey)
		}

		log.Println("settings saved and applied")
	}

	a.tray.Run(cfg, a.toggleDictation, func() {
		a.mu.Lock()
		cancel := a.cancelDicta
		a.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		a.hkmMu.Lock()
		if a.hkm != nil {
			a.hkm.Stop()
		}
		a.hkmMu.Unlock()
	}, startupErr)
}

// rebindHotkey stops the current hotkey listener and registers a new one.
func (a *app) rebindHotkey(newHotkey string) {
	a.hkmMu.Lock()
	defer a.hkmMu.Unlock()

	if a.hkm != nil {
		a.hkm.Stop()
		a.hkm = nil
	}

	hkm, err := hotkey.New(newHotkey)
	if err != nil {
		log.Printf("hotkey: failed to create %q: %v", newHotkey, err)
		a.tray.SetError("Hotkey invalid: " + newHotkey)
		return
	}
	if err := hkm.Register(a.toggleDictation); err != nil {
		log.Printf("hotkey: failed to grab %q: %v", newHotkey, err)
		a.tray.SetError("Hotkey taken: " + newHotkey)
		return
	}
	a.hkm = hkm
	log.Printf("hotkey: rebound to %q", newHotkey)
}

// buildRecognizer constructs a Recognizer from the current config.
func buildRecognizer(cfg *config.Config) *speech.Recognizer {
	return &speech.Recognizer{
		Language:       cfg.Language,
		APIKey:         cfg.APIKey,
		UseAdvancedAPI: cfg.UseAdvancedAPI,
		SilenceChunks:  cfg.SilenceChunks,
		Sensitivity:    cfg.Sensitivity,
	}
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
	a.cfgMu.RLock()
	timeout := a.cfg.Timeout
	a.cfgMu.RUnlock()
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	a.cancelDicta = cancel
	a.mu.Unlock()

	defer func() {
		cancel()
		a.mu.Lock()
		a.isListening = false
		a.cancelDicta = nil
		a.mu.Unlock()
	}()

	// Show "Listening" immediately so the user sees feedback before VAD runs.
	a.tray.SetListening()

	audioCh, err := a.recorder.Start(ctx)
	if err != nil {
		a.tray.SetError("Mic error: " + err.Error())
		log.Printf("recorder error: %v", err)
		return
	}

	a.cfgMu.RLock()
	rec := a.recognizer
	typer := a.typer
	a.cfgMu.RUnlock()

	rec.OnProcessing = func() {
		a.tray.SetProcessing()
	}

	text, err := rec.Recognize(ctx, audioCh)
	a.recorder.Stop()

	if err != nil {
		switch ctx.Err() {
		case context.DeadlineExceeded:
			a.tray.SetError("Timeout")
		case context.Canceled:
			a.tray.SetIdle()
		default:
			a.tray.SetError(err.Error())
			log.Printf("recognizer error: %v", err)
		}
		return
	}

	if text == "" {
		a.tray.SetError("Could not understand speech")
		return
	}

	if err := typer.Type(text); err != nil {
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
