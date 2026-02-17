package main

import (
	"context"
	"time"

	"github.com/Alijeyrad/gotalk-dictation/internal/speech"
)

func (a *app) newRecognizer() *speech.Recognizer {
	return &speech.Recognizer{
		Language:       a.cfg.Language,
		APIKey:         a.cfg.APIKey,
		UseAdvancedAPI: a.cfg.UseAdvancedAPI,
		SilenceChunks:  a.cfg.SilenceChunks,
		Sensitivity:    a.cfg.Sensitivity,
	}
}

func (a *app) newPTTRecognizer() *speech.Recognizer {
	r := a.newRecognizer()
	r.SkipVAD = true
	return r
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

func (a *app) pttStartDictation() {
	a.cfgMu.RLock()
	rec := a.newPTTRecognizer()
	a.cfgMu.RUnlock()
	go a.runDictation(rec)
}

func (a *app) startDictation() {
	a.cfgMu.RLock()
	rec := a.recognizer
	a.cfgMu.RUnlock()
	a.runDictation(rec)
}

func (a *app) runDictation(rec *speech.Recognizer) {
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

func (a *app) undoLastDictation() {
	a.cfgMu.RLock()
	typer := a.typer
	a.cfgMu.RUnlock()
	typer.Undo() //nolint:errcheck
}
