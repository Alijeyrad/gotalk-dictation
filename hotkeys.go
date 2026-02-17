package main

import "github.com/Alijeyrad/gotalk-dictation/internal/hotkey"

// bindHotkey stops any existing manager, then creates and registers a new one.
// Must be called with hkmMu held. Returns nil if key is empty (disables the hotkey).
func (a *app) bindHotkey(hkm **hotkey.Manager, key string, register func(*hotkey.Manager) error) error {
	if *hkm != nil {
		(*hkm).Stop()
		*hkm = nil
	}
	if key == "" {
		return nil
	}
	h, err := hotkey.New(key)
	if err != nil {
		return err
	}
	if err := register(h); err != nil {
		h.Stop()
		return err
	}
	*hkm = h
	return nil
}

// registerHotkeys sets up all three hotkey managers from the current config.
// Called once at startup.
func (a *app) registerHotkeys() error {
	a.cfgMu.RLock()
	cfg := a.cfg
	a.cfgMu.RUnlock()

	a.hkmMu.Lock()
	defer a.hkmMu.Unlock()

	if err := a.bindHotkey(&a.hkm, cfg.Hotkey, func(h *hotkey.Manager) error {
		return h.Register(a.toggleDictation)
	}); err != nil {
		return err
	}

	// PTT and undo errors are non-fatal — app still works with just the toggle hotkey.
	a.bindHotkey(&a.pttHkm, cfg.PTTHotkey, func(h *hotkey.Manager) error { //nolint:errcheck
		return h.RegisterPushToTalk(a.pttStartDictation, a.recorder.Stop)
	})
	a.bindHotkey(&a.undoHkm, cfg.UndoHotkey, func(h *hotkey.Manager) error { //nolint:errcheck
		return h.Register(a.undoLastDictation)
	})

	return nil
}

func (a *app) rebindHotkey(key string) {
	a.hkmMu.Lock()
	defer a.hkmMu.Unlock()

	if err := a.bindHotkey(&a.hkm, key, func(h *hotkey.Manager) error {
		return h.Register(a.toggleDictation)
	}); err != nil {
		a.tray.SetError("Hotkey error: " + key)
	}
}

func (a *app) rebindPTTHotkey(key string) {
	a.hkmMu.Lock()
	defer a.hkmMu.Unlock()

	if err := a.bindHotkey(&a.pttHkm, key, func(h *hotkey.Manager) error {
		return h.RegisterPushToTalk(a.pttStartDictation, a.recorder.Stop)
	}); err != nil {
		a.tray.SetError("PTT hotkey error: " + key)
	}
}

func (a *app) rebindUndoHotkey(key string) {
	a.hkmMu.Lock()
	defer a.hkmMu.Unlock()

	if err := a.bindHotkey(&a.undoHkm, key, func(h *hotkey.Manager) error {
		return h.Register(a.undoLastDictation)
	}); err != nil {
		a.tray.SetError("Undo hotkey error: " + key)
	}
}
