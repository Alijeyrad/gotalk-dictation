package ui

import (
	_ "embed"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/driver/desktop"

	"github.com/Alijeyrad/gotalk-dictation/internal/config"
)

//go:embed assets/icon.png
var iconPNG []byte

type Tray struct {
	fyneApp     fyne.App
	popup       *X11Popup
	settingsWin fyne.Window // non-nil while the settings window is open
	aboutWin    fyne.Window // non-nil while the about window is open

	// popupGen is incremented on every state transition that should own the
	// popup. The auto-hide goroutines in SetDone/SetError capture their
	// generation at spawn time and only call SetIdle if it hasn't changed,
	// preventing a stale hide from clearing an active listening/processing state.
	popupGen atomic.Uint64

	cfgMu sync.RWMutex
	cfg   *config.Config

	OnSettingsSave func(*config.Config)
}

func (t *Tray) Run(cfg *config.Config, onDictate func(), onQuit func(), startupErr error) {
	t.cfgMu.Lock()
	t.cfg = cfg
	t.cfgMu.Unlock()

	popup, err := newX11Popup()
	if err == nil {
		t.popup = popup
	}

	a := app.NewWithID("com.alijeyrad.GoTalkDictation")
	t.fyneApp = a

	settingsItem := fyne.NewMenuItem("Settings…", func() {
		if t.OnSettingsSave == nil {
			return
		}
		// If the window is already open, just bring it to the front.
		if t.settingsWin != nil {
			t.settingsWin.Show()
			t.settingsWin.RequestFocus()
			return
		}
		t.cfgMu.RLock()
		current := t.cfg
		t.cfgMu.RUnlock()
		win := showSettingsWindow(t.fyneApp, current, t.OnSettingsSave)
		t.settingsWin = win
		win.SetOnClosed(func() { t.settingsWin = nil })
	})

	aboutItem := fyne.NewMenuItem("About…", func() {
		if t.aboutWin != nil {
			t.aboutWin.Show()
			t.aboutWin.RequestFocus()
			return
		}
		win := showAboutWindow(t.fyneApp)
		t.aboutWin = win
		win.SetOnClosed(func() { t.aboutWin = nil })
	})

	menu := fyne.NewMenu("GoTalk",
		fyne.NewMenuItem("Start Dictation", onDictate),
		fyne.NewMenuItemSeparator(),
		settingsItem,
		aboutItem,
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Quit", func() {
			onQuit()
			if t.popup != nil {
				t.popup.Close()
			}
			a.Quit()
		}),
	)

	iconRes := fyne.NewStaticResource("icon.png", iconPNG)
	if desk, ok := a.(desktop.App); ok {
		desk.SetSystemTrayMenu(menu)
		desk.SetSystemTrayIcon(iconRes)
	}

	if startupErr != nil {
		go t.SetError("Hotkey unavailable — check Settings")
	}

	a.Run()
}

func (t *Tray) UpdateConfig(cfg *config.Config) {
	t.cfgMu.Lock()
	t.cfg = cfg
	t.cfgMu.Unlock()
}

func (t *Tray) SetListening() {
	t.popupGen.Add(1) // invalidate any pending auto-hide goroutine
	if t.popup != nil {
		t.popup.Show(stListening)
	}
}

func (t *Tray) SetProcessing() {
	if t.popup != nil {
		t.popup.SetState(stProcessing)
	}
}

func (t *Tray) SetDone(_ string) {
	t.SetIdle()
}

func (t *Tray) SetIdle() {
	if t.popup != nil {
		t.popup.Hide()
	}
}

func (t *Tray) SetError(msg string) {
	log.Println("gotalk error:", msg)
	gen := t.popupGen.Add(1)
	if t.popup != nil {
		t.popup.Show(stError)
	}
	go func() {
		time.Sleep(3 * time.Second)
		if t.popupGen.Load() == gen {
			t.SetIdle()
		}
	}()
}
