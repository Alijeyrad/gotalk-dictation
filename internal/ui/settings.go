package ui

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/Alijeyrad/gotalk-dictation/internal/config"
)

// OpenSettings shows the settings window.
// Must be called from the Fyne main goroutine (e.g. inside a menu callback).
func OpenSettings(fyneApp fyne.App, cfg *config.Config, onSave func(*config.Config)) {
	showSettingsWindow(fyneApp, cfg, onSave)
}

// languages is the ordered list shown in the language dropdown.
var languages = []struct{ code, label string }{
	{"en-US", "English (US)"},
	{"es-ES", "Spanish"},
	{"fa-IR", "Persian (Farsi)"},
	{"fr-FR", "French"},
	{"de-DE", "German"},
}

func showSettingsWindow(fyneApp fyne.App, cfg *config.Config, onSave func(*config.Config)) {
	w := fyneApp.NewWindow("GoTalk Dictation — Settings")
	w.SetIcon(fyne.NewStaticResource("icon.png", iconPNG))
	w.Resize(fyne.NewSize(460, 500))
	w.SetFixedSize(true)

	// ---- Language dropdown ----
	langLabels := make([]string, len(languages))
	labelToCode := make(map[string]string, len(languages))
	codeToLabel := make(map[string]string, len(languages))
	for i, l := range languages {
		langLabels[i] = l.label
		labelToCode[l.label] = l.code
		codeToLabel[l.code] = l.label
	}

	initialLabel := codeToLabel[cfg.Language]
	if initialLabel == "" {
		initialLabel = cfg.Language // unknown code: show raw
	}
	langSelect := widget.NewSelect(langLabels, nil)
	langSelect.SetSelected(initialLabel)

	// ---- Speech recognition ----
	apiKeyEntry := widget.NewPasswordEntry()
	apiKeyEntry.SetText(cfg.APIKey)
	apiKeyEntry.SetPlaceHolder("Leave blank to use built-in key")

	advancedCheck := widget.NewCheck("Use Google Cloud Speech API", nil)
	advancedCheck.SetChecked(cfg.UseAdvancedAPI)
	advancedNote := widget.NewLabelWithStyle(
		"Requires GOOGLE_APPLICATION_CREDENTIALS or gcloud ADC",
		fyne.TextAlignLeading, fyne.TextStyle{Italic: true},
	)

	// ---- VAD tuning ----
	silenceLabel := widget.NewLabel(fmt.Sprintf("%.0f chunks (~%.0f ms)",
		float64(cfg.SilenceChunks), float64(cfg.SilenceChunks)*62))
	silenceSlider := widget.NewSlider(4, 32)
	silenceSlider.SetValue(float64(cfg.SilenceChunks))

	sensitivityLabel := widget.NewLabel(fmt.Sprintf("%.1f", cfg.Sensitivity))
	sensitivitySlider := widget.NewSlider(1.0, 6.0)
	sensitivitySlider.Step = 0.1
	sensitivitySlider.SetValue(cfg.Sensitivity)

	// ---- Hotkey capture ----
	// currentHotkey holds the value that will be written on Save.
	currentHotkey := cfg.Hotkey
	capturing := false

	// updateSaveBtn is declared here so hotkeyBtn's closure can reference it
	// before the body is assigned below (Go closures capture the variable).
	var updateSaveBtn func()

	hotkeyBtn := widget.NewButton(cfg.Hotkey, nil)

	stopCapture := func() {
		capturing = false
		if dc, ok := w.Canvas().(desktop.Canvas); ok {
			dc.SetOnKeyDown(nil)
		}
	}

	hotkeyBtn.OnTapped = func() {
		if capturing {
			// Second tap cancels capture.
			stopCapture()
			hotkeyBtn.SetText(currentHotkey)
			return
		}

		capturing = true
		hotkeyBtn.SetText("Press key combination…")

		dc, ok := w.Canvas().(desktop.Canvas)
		if !ok {
			// Fallback: can't capture on this platform.
			capturing = false
			hotkeyBtn.SetText(currentHotkey)
			return
		}

		dc.SetOnKeyDown(func(ev *fyne.KeyEvent) {
			if !capturing {
				return
			}

			// Read modifiers held at key-down time.
			var mods fyne.KeyModifier
			if drv, ok := fyne.CurrentApp().Driver().(desktop.Driver); ok {
				mods = drv.CurrentKeyModifiers()
			}

			// Require at least one modifier (Alt, Ctrl, Super).
			// Allow Shift only in combination with others.
			meaningful := mods & (fyne.KeyModifierAlt | fyne.KeyModifierControl | fyne.KeyModifierSuper)
			if meaningful == 0 {
				return
			}

			// Build "Mod-Mod-key" string.
			var parts []string
			if mods&fyne.KeyModifierControl != 0 {
				parts = append(parts, "Ctrl")
			}
			if mods&fyne.KeyModifierAlt != 0 {
				parts = append(parts, "Alt")
			}
			if mods&fyne.KeyModifierShift != 0 {
				parts = append(parts, "Shift")
			}
			if mods&fyne.KeyModifierSuper != 0 {
				parts = append(parts, "Super")
			}
			parts = append(parts, strings.ToLower(string(ev.Name)))
			hotkey := strings.Join(parts, "-")

			stopCapture()
			currentHotkey = hotkey
			hotkeyBtn.SetText(hotkey)
			// trigger change detection
			updateSaveBtn()
		})
	}

	// ---- General ----
	timeoutEntry := widget.NewEntry()
	timeoutEntry.SetText(strconv.Itoa(cfg.Timeout))
	timeoutEntry.SetPlaceHolder("seconds (default 60)")

	punctCheck := widget.NewCheck("Add punctuation", nil)
	punctCheck.SetChecked(cfg.EnablePunctuation)

	// ---- Buttons ----
	saveBtn := widget.NewButton("Save", nil)
	saveBtn.Importance = widget.HighImportance
	saveBtn.Disable()

	closeBtn := widget.NewButton("Close", nil)

	// currentLang returns the BCP-47 code for the currently selected label.
	currentLang := func() string {
		if code, ok := labelToCode[langSelect.Selected]; ok {
			return code
		}
		return langSelect.Selected
	}

	// hasChanges returns true if any widget differs from the original cfg.
	hasChanges := func() bool {
		timeout, err := strconv.Atoi(timeoutEntry.Text)
		if err != nil || timeout < 5 {
			timeout = cfg.Timeout
		}
		return currentLang() != cfg.Language ||
			apiKeyEntry.Text != cfg.APIKey ||
			advancedCheck.Checked != cfg.UseAdvancedAPI ||
			int(silenceSlider.Value) != cfg.SilenceChunks ||
			fmt.Sprintf("%.1f", sensitivitySlider.Value) != fmt.Sprintf("%.1f", cfg.Sensitivity) ||
			currentHotkey != cfg.Hotkey ||
			timeout != cfg.Timeout ||
			punctCheck.Checked != cfg.EnablePunctuation
	}

	updateSaveBtn = func() {
		if hasChanges() {
			saveBtn.Enable()
		} else {
			saveBtn.Disable()
		}
	}

	langSelect.OnChanged = func(_ string) { updateSaveBtn() }
	apiKeyEntry.OnChanged = func(_ string) { updateSaveBtn() }
	advancedCheck.OnChanged = func(_ bool) { updateSaveBtn() }
	silenceSlider.OnChanged = func(v float64) {
		silenceLabel.SetText(fmt.Sprintf("%.0f chunks (~%.0f ms)", v, v*62))
		updateSaveBtn()
	}
	sensitivitySlider.OnChanged = func(v float64) {
		sensitivityLabel.SetText(fmt.Sprintf("%.1f", v))
		updateSaveBtn()
	}
	timeoutEntry.OnChanged = func(_ string) { updateSaveBtn() }
	punctCheck.OnChanged = func(_ bool) { updateSaveBtn() }

	// doSave builds the new config from current widget values and calls onSave.
	doSave := func() {
		timeout, err := strconv.Atoi(timeoutEntry.Text)
		if err != nil || timeout < 5 {
			timeout = cfg.Timeout
		}
		newCfg := &config.Config{
			Hotkey:            currentHotkey,
			Language:          currentLang(),
			Timeout:           timeout,
			SilenceChunks:     int(silenceSlider.Value),
			Sensitivity:       sensitivitySlider.Value,
			APIKey:            apiKeyEntry.Text,
			UseAdvancedAPI:    advancedCheck.Checked,
			EnablePunctuation: punctCheck.Checked,
		}
		onSave(newCfg)
		// Update cfg baseline so Save disables again.
		*cfg = *newCfg
		saveBtn.Disable()
	}

	saveBtn.OnTapped = func() { doSave() }

	// tryClose prompts for unsaved changes, then closes.
	tryClose := func() {
		if !hasChanges() {
			stopCapture()
			w.Close()
			return
		}
		d := dialog.NewCustomConfirm(
			"Unsaved changes",
			"Save", "Discard",
			widget.NewLabel("You have unsaved changes."),
			func(save bool) {
				if save {
					doSave()
				}
				stopCapture()
				w.Close()
			},
			w,
		)
		d.Show()
	}

	closeBtn.OnTapped = tryClose
	w.SetCloseIntercept(tryClose)

	// ---- Layout ----
	form := container.New(layout.NewFormLayout(),
		widget.NewLabelWithStyle("Language", fyne.TextAlignTrailing, fyne.TextStyle{Bold: true}),
		langSelect,

		widget.NewLabelWithStyle("Custom API key", fyne.TextAlignTrailing, fyne.TextStyle{Bold: true}),
		apiKeyEntry,

		widget.NewLabel(""),
		advancedCheck,
		widget.NewLabel(""),
		advancedNote,

		widget.NewSeparator(), widget.NewSeparator(),

		widget.NewLabelWithStyle("Silence end", fyne.TextAlignTrailing, fyne.TextStyle{Bold: true}),
		container.NewBorder(nil, nil, nil, silenceLabel, silenceSlider),

		widget.NewLabelWithStyle("Sensitivity", fyne.TextAlignTrailing, fyne.TextStyle{Bold: true}),
		container.NewBorder(nil, nil, nil, sensitivityLabel, sensitivitySlider),

		widget.NewSeparator(), widget.NewSeparator(),

		widget.NewLabelWithStyle("Hotkey", fyne.TextAlignTrailing, fyne.TextStyle{Bold: true}),
		hotkeyBtn,

		widget.NewLabelWithStyle("Max duration", fyne.TextAlignTrailing, fyne.TextStyle{Bold: true}),
		container.NewBorder(nil, nil, nil, widget.NewLabel("sec"), timeoutEntry),

		widget.NewLabel(""),
		punctCheck,
	)

	buttons := container.NewHBox(layout.NewSpacer(), closeBtn, saveBtn)
	content := container.NewBorder(nil, buttons, nil, nil, container.NewVScroll(form))

	w.SetContent(content)
	w.Show()
}

