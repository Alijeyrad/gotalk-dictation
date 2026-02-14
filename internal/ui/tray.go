package ui

import (
	_ "embed"
	"log"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/driver/desktop"

	"github.com/Alijeyrad/gotalk-dictation/internal/config"
)

//go:embed assets/icon.png
var iconPNG []byte

// Tray manages the system tray icon (via Fyne) and the dictation popup (via X11).
type Tray struct {
	fyneApp  fyne.App
	popup    *X11Popup
	dictItem *fyne.MenuItem

	// OnSettingsSave is called when the user saves settings from the settings window.
	OnSettingsSave func(*config.Config)
}

// Run initializes the system tray and X11 popup, then runs the Fyne event loop.
// Blocks until the app quits. Must be called on the main goroutine.
func (t *Tray) Run(cfg *config.Config, onDictate func(), onQuit func(), startupErr error) {
	popup, err := newX11Popup()
	if err != nil {
		log.Printf("warning: X11 popup unavailable (%v); animations disabled", err)
	}
	t.popup = popup

	a := app.NewWithID("com.alijeyrad.gotalk-dictation")
	t.fyneApp = a

	t.dictItem = fyne.NewMenuItem("Start Dictation (Alt+D)", onDictate)

	settingsItem := fyne.NewMenuItem("Settings…", func() {
		if t.OnSettingsSave != nil {
			OpenSettings(a, cfg, t.OnSettingsSave)
		}
	})

	menu := fyne.NewMenu("GoTalk",
		t.dictItem,
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
		go t.SetError("Alt+D taken — remove old DE shortcut")
	}

	a.Run()
}

// UpdateConfig refreshes the settings pointer used by the settings window.
// Call this after a settings save so re-opening settings shows the latest values.
func (t *Tray) UpdateConfig(cfg *config.Config) {
	// Settings window reads cfg at open time, so just replace the menu item closure.
	// The tray already holds the pointer if Run was called with it, but since cfg
	// may be a new allocation, we close over it in the new Run setup via main.go.
	_ = cfg // main.go owns the canonical cfg pointer; this is a no-op reminder.
}

// ---- State methods ---------------------------------------------------------

// SetListening shows the popup in the "Listening" state.
func (t *Tray) SetListening() {
	if t.popup != nil {
		t.popup.Show(stListening)
	}
}

// SetProcessing updates the popup to the "Processing" state.
func (t *Tray) SetProcessing() {
	if t.popup != nil {
		t.popup.SetState(stProcessing)
	}
}

// SetDone flashes green for 2 seconds then hides.
func (t *Tray) SetDone(text string) {
	if t.popup != nil {
		t.popup.SetState(stDone)
	}
	go func() {
		time.Sleep(2 * time.Second)
		t.SetIdle()
	}()
}

// SetIdle hides the popup.
func (t *Tray) SetIdle() {
	if t.popup != nil {
		t.popup.Hide()
	}
}

// SetError flashes red for 3 seconds then hides.
func (t *Tray) SetError(msg string) {
	if t.popup != nil {
		t.popup.Show(stError)
	}
	go func() {
		time.Sleep(3 * time.Second)
		t.SetIdle()
	}()
}
