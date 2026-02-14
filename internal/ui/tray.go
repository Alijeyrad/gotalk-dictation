package ui

import (
	_ "embed"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/driver/desktop"

	"github.com/Alijeyrad/gotalk-dictation/internal/config"
)

//go:embed assets/icon.png
var iconPNG []byte

type Tray struct {
	fyneApp fyne.App
	popup   *X11Popup

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

	a := app.NewWithID("com.alijeyrad.gotalk-dictation")
	t.fyneApp = a

	settingsItem := fyne.NewMenuItem("Settings…", func() {
		if t.OnSettingsSave == nil {
			return
		}
		t.cfgMu.RLock()
		current := t.cfg
		t.cfgMu.RUnlock()
		showSettingsWindow(t.fyneApp, current, t.OnSettingsSave)
	})

	menu := fyne.NewMenu("GoTalk",
		fyne.NewMenuItem("Start Dictation", onDictate),
		fyne.NewMenuItemSeparator(),
		settingsItem,
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
	if t.popup != nil {
		t.popup.Show(stListening)
	}
}

func (t *Tray) SetProcessing() {
	if t.popup != nil {
		t.popup.SetState(stProcessing)
	}
}

func (t *Tray) SetDone(text string) {
	if t.popup != nil {
		t.popup.ShowDone(text)
	}
	go func() {
		time.Sleep(2 * time.Second)
		t.SetIdle()
	}()
}

func (t *Tray) SetIdle() {
	if t.popup != nil {
		t.popup.Hide()
	}
}

func (t *Tray) SetError(_ string) {
	if t.popup != nil {
		t.popup.Show(stError)
	}
	go func() {
		time.Sleep(3 * time.Second)
		t.SetIdle()
	}()
}
