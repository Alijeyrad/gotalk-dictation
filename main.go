package main

import (
	"context"
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

	hkmMu   sync.Mutex
	hkm     *hotkey.Manager // toggle hotkey
	pttHkm  *hotkey.Manager // push-to-talk hotkey (independent)
	undoHkm *hotkey.Manager

	mu          sync.Mutex
	isListening bool
	cancelDicta context.CancelFunc
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		cfg = config.Default()
	}

	a := &app{
		cfg:      cfg,
		recorder: &audio.Recorder{},
		typer:    &typing.Typer{EnablePunctuation: cfg.EnablePunctuation},
		tray:     &ui.Tray{},
	}
	a.recognizer = buildRecognizer(cfg)

	var startupErr error

	// Register toggle hotkey.
	hkm, err := hotkey.New(cfg.Hotkey)
	if err != nil {
		startupErr = err
	} else {
		if err := hkm.Register(a.toggleDictation); err != nil {
			hkm.Stop() // close the X11 connection; grab failed
			startupErr = err
		} else {
			a.hkmMu.Lock()
			a.hkm = hkm
			a.hkmMu.Unlock()
		}
	}

	// Register push-to-talk hotkey (independent from toggle).
	if cfg.PTTHotkey != "" {
		if pttHkm, err := hotkey.New(cfg.PTTHotkey); err == nil {
			if err := pttHkm.RegisterPushToTalk(a.startDictation, a.recorder.Stop); err == nil {
				a.hkmMu.Lock()
				a.pttHkm = pttHkm
				a.hkmMu.Unlock()
			} else {
				pttHkm.Stop() // close the X11 connection; grab failed
			}
		}
	}

	if cfg.UndoHotkey != "" {
		if uhkm, err := hotkey.New(cfg.UndoHotkey); err == nil {
			if err := uhkm.Register(a.undoLastDictation); err != nil {
				uhkm.Stop() // close the X11 connection; grab failed
			} else {
				a.hkmMu.Lock()
				a.undoHkm = uhkm
				a.hkmMu.Unlock()
			}
		}
	}

	a.tray.OnSettingsSave = func(newCfg *config.Config) {
		newCfg.Save() //nolint:errcheck

		a.cfgMu.RLock()
		oldHotkey := a.cfg.Hotkey
		oldPTTHotkey := a.cfg.PTTHotkey
		oldUndoHotkey := a.cfg.UndoHotkey
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
		if newCfg.PTTHotkey != oldPTTHotkey {
			a.rebindPTTHotkey(newCfg.PTTHotkey)
		}
		if newCfg.UndoHotkey != oldUndoHotkey {
			a.rebindUndoHotkey(newCfg.UndoHotkey)
		}
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
		if a.pttHkm != nil {
			a.pttHkm.Stop()
		}
		if a.undoHkm != nil {
			a.undoHkm.Stop()
		}
		a.hkmMu.Unlock()
	}, startupErr)
}

func (a *app) undoLastDictation() {
	a.cfgMu.RLock()
	typer := a.typer
	a.cfgMu.RUnlock()
	typer.Undo() //nolint:errcheck
}

func (a *app) rebindHotkey(newHotkey string) {
	a.hkmMu.Lock()
	defer a.hkmMu.Unlock()

	if a.hkm != nil {
		a.hkm.Stop()
		a.hkm = nil
	}

	hkm, err := hotkey.New(newHotkey)
	if err != nil {
		a.tray.SetError("Hotkey invalid: " + newHotkey)
		return
	}
	if err := hkm.Register(a.toggleDictation); err != nil {
		hkm.Stop()
		a.tray.SetError("Hotkey taken: " + newHotkey)
		return
	}
	a.hkm = hkm
}

func (a *app) rebindPTTHotkey(newHotkey string) {
	a.hkmMu.Lock()
	defer a.hkmMu.Unlock()

	if a.pttHkm != nil {
		a.pttHkm.Stop()
		a.pttHkm = nil
	}

	if newHotkey == "" {
		return
	}

	pttHkm, err := hotkey.New(newHotkey)
	if err != nil {
		a.tray.SetError("PTT hotkey invalid: " + newHotkey)
		return
	}
	if err := pttHkm.RegisterPushToTalk(a.startDictation, a.recorder.Stop); err != nil {
		pttHkm.Stop()
		a.tray.SetError("PTT hotkey taken: " + newHotkey)
		return
	}
	a.pttHkm = pttHkm
}

func (a *app) rebindUndoHotkey(newHotkey string) {
	a.hkmMu.Lock()
	defer a.hkmMu.Unlock()
	if a.undoHkm != nil {
		a.undoHkm.Stop()
		a.undoHkm = nil
	}
	if newHotkey == "" {
		return
	}
	uhkm, err := hotkey.New(newHotkey)
	if err != nil {
		return
	}
	if err := uhkm.Register(a.undoLastDictation); err != nil {
		uhkm.Stop()
		return
	}
	a.undoHkm = uhkm
}

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

	a.tray.SetListening()

	audioCh, err := a.recorder.Start(ctx)
	if err != nil {
		a.tray.SetError("Mic error: " + err.Error())
		return
	}

	a.cfgMu.RLock()
	rec := a.recognizer
	typer := a.typer
	a.cfgMu.RUnlock()

	rec.OnProcessing = func() { a.tray.SetProcessing() }

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
		}
		return
	}

	if text == "" {
		a.tray.SetError("Could not understand speech")
		return
	}

	if err := typer.Type(text); err != nil {
		a.tray.SetError("Type error: " + err.Error())
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
