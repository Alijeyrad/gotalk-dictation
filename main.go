package main

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/Alijeyrad/gotalk-dictation/internal/audio"
	"github.com/Alijeyrad/gotalk-dictation/internal/config"
	"github.com/Alijeyrad/gotalk-dictation/internal/hotkey"
	"github.com/Alijeyrad/gotalk-dictation/internal/speech"
	"github.com/Alijeyrad/gotalk-dictation/internal/typing"
	"github.com/Alijeyrad/gotalk-dictation/internal/ui"
	"github.com/Alijeyrad/gotalk-dictation/internal/version"
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
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("gotalk-dictation %s (%s)\n", version.Version, version.Commit)
		os.Exit(0)
	}

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
	a.recognizer = a.newRecognizer()

	var startupErr error
	if err := a.registerHotkeys(); err != nil {
		startupErr = err
	}

	a.setupCallbacks()
	a.tray.Run(cfg, a.toggleDictation, a.stop, startupErr)
}

func (a *app) setupCallbacks() {
	a.tray.OnSettingsSave = func(newCfg *config.Config) {
		newCfg.Save() //nolint:errcheck

		a.cfgMu.RLock()
		oldHotkey := a.cfg.Hotkey
		oldPTTHotkey := a.cfg.PTTHotkey
		oldUndoHotkey := a.cfg.UndoHotkey
		a.cfgMu.RUnlock()

		a.cfgMu.Lock()
		a.cfg = newCfg
		a.recognizer = a.newRecognizer()
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
}

func (a *app) stop() {
	a.mu.Lock()
	cancel := a.cancelDicta
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}

	a.hkmMu.Lock()
	defer a.hkmMu.Unlock()
	for _, hkm := range []*hotkey.Manager{a.hkm, a.pttHkm, a.undoHkm} {
		if hkm != nil {
			hkm.Stop()
		}
	}
}
