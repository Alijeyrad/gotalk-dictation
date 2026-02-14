package ui

import (
	_ "embed"
	"log"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/driver/desktop"
)

//go:embed assets/icon.png
var iconPNG []byte

// Tray manages the system tray icon (via Fyne) and the dictation popup (via X11).
// The X11 popup is override-redirect: no title bar, no focus stealing, always on top.
type Tray struct {
	fyneApp  fyne.App
	popup    *X11Popup
	dictItem *fyne.MenuItem
}

// Run initializes the system tray and X11 popup, then runs the Fyne event loop.
// Blocks until the app quits. Must be called on the main goroutine.
func (t *Tray) Run(onDictate func(), onQuit func(), startupErr error) {
	// X11 popup — created before Fyne so it's ready immediately.
	popup, err := newX11Popup()
	if err != nil {
		log.Printf("warning: X11 popup unavailable (%v); status will only appear in logs", err)
	}
	t.popup = popup

	a := app.NewWithID("com.alijeyrad.gotalk-dictation")
	t.fyneApp = a

	t.dictItem = fyne.NewMenuItem("Start Dictation (Alt+D)", onDictate)
	menu := fyne.NewMenu("GoTalk",
		t.dictItem,
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

