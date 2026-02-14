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

var languages = []struct{ code, label string }{
	{"fa-IR", "Persian (Farsi)"},
	{"en-US", "English (US)"},
	{"en-GB", "English (UK)"},
	{"es-ES", "Spanish (Spain)"},
	{"es-MX", "Spanish (Mexico)"},
	{"es-419", "Spanish (Latin America)"},
	{"fr-FR", "French (France)"},
	{"fr-CA", "French (Canada)"},
	{"de-DE", "German"},
	{"it-IT", "Italian"},
	{"pt-PT", "Portuguese (Portugal)"},
	{"pt-BR", "Portuguese (Brazil)"},
	{"ar-SA", "Arabic (Saudi Arabia)"},
	{"ar-EG", "Arabic (Egypt)"},
	{"zh-CN", "Chinese (Simplified)"},
	{"zh-TW", "Chinese (Traditional)"},
	{"nl-NL", "Dutch"},
	{"hi-IN", "Hindi"},
	{"ja-JP", "Japanese"},
	{"ko-KR", "Korean"},
	{"pl-PL", "Polish"},
	{"ru-RU", "Russian"},
	{"sv-SE", "Swedish"},
	{"tr-TR", "Turkish"},
	{"uk-UA", "Ukrainian"},
}

func showSettingsWindow(fyneApp fyne.App, cfg *config.Config, onSave func(*config.Config)) {
	w := fyneApp.NewWindow("GoTalk Dictation — Settings")
	w.SetIcon(fyne.NewStaticResource("icon.png", iconPNG))
	w.Resize(fyne.NewSize(460, 500))
	w.SetFixedSize(true)

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
		initialLabel = cfg.Language
	}
	langSelect := widget.NewSelect(langLabels, nil)
	langSelect.SetSelected(initialLabel)

	apiKeyEntry := widget.NewPasswordEntry()
	apiKeyEntry.SetText(cfg.APIKey)
	apiKeyEntry.SetPlaceHolder("Leave blank to use built-in key")

	advancedCheck := widget.NewCheck("Use Google Cloud Speech API", nil)
	advancedCheck.SetChecked(cfg.UseAdvancedAPI)
	advancedNote := widget.NewLabelWithStyle(
		"Requires GOOGLE_APPLICATION_CREDENTIALS or gcloud ADC",
		fyne.TextAlignLeading, fyne.TextStyle{Italic: true},
	)

	silenceLabel := widget.NewLabel(fmt.Sprintf("%.0f chunks (~%.0f ms)",
		float64(cfg.SilenceChunks), float64(cfg.SilenceChunks)*62))
	silenceSlider := widget.NewSlider(4, 32)
	silenceSlider.SetValue(float64(cfg.SilenceChunks))

	sensitivityLabel := widget.NewLabel(fmt.Sprintf("%.1f", cfg.Sensitivity))
	sensitivitySlider := widget.NewSlider(1.0, 6.0)
	sensitivitySlider.Step = 0.1
	sensitivitySlider.SetValue(cfg.Sensitivity)

	currentHotkey := cfg.Hotkey
	currentUndoHotkey := cfg.UndoHotkey

	// updateSaveBtn is declared before the hotkey buttons so closures can
	// reference it; the body is assigned further below.
	var updateSaveBtn func()

	// heldMods tracks pressed modifiers manually because
	// desktop.Driver.CurrentKeyModifiers() is unreliable on KDE Wayland.
	var heldMods fyne.KeyModifier
	// activeCapture points to the button currently waiting for a key combination.
	var activeCapture *widget.Button

	stopCapture := func() {
		activeCapture = nil
		heldMods = 0
		if dc, ok := w.Canvas().(desktop.Canvas); ok {
			dc.SetOnKeyDown(nil)
			dc.SetOnKeyUp(nil)
		}
	}

	// startCapture wires the canvas key handlers to capture one key combination
	// into *target, then updates btn's label and calls onChange.
	startCapture := func(btn *widget.Button, target *string, onChange func()) {
		dc, ok := w.Canvas().(desktop.Canvas)
		if !ok {
			return
		}
		activeCapture = btn
		heldMods = 0
		btn.SetText("Press key combination…")

		dc.SetOnKeyUp(func(ev *fyne.KeyEvent) {
			n := strings.ToLower(string(ev.Name))
			switch {
			case strings.Contains(n, "control"):
				heldMods &^= fyne.KeyModifierControl
			case strings.Contains(n, "alt"):
				heldMods &^= fyne.KeyModifierAlt
			case strings.Contains(n, "shift"):
				heldMods &^= fyne.KeyModifierShift
			case strings.Contains(n, "super") || strings.Contains(n, "meta"):
				heldMods &^= fyne.KeyModifierSuper
			}
		})

		dc.SetOnKeyDown(func(ev *fyne.KeyEvent) {
			if activeCapture != btn {
				return
			}
			n := strings.ToLower(string(ev.Name))
			switch {
			case strings.Contains(n, "control"):
				heldMods |= fyne.KeyModifierControl
				return
			case strings.Contains(n, "alt"):
				heldMods |= fyne.KeyModifierAlt
				return
			case strings.Contains(n, "shift"):
				heldMods |= fyne.KeyModifierShift
				return
			case strings.Contains(n, "super") || strings.Contains(n, "meta"):
				heldMods |= fyne.KeyModifierSuper
				return
			case strings.Contains(n, "caps"):
				return
			}

			if ev.Name == fyne.KeyEscape {
				stopCapture()
				btn.SetText(*target)
				return
			}

			// Require at least one of Alt/Ctrl/Super; Shift alone is not enough.
			if heldMods&(fyne.KeyModifierAlt|fyne.KeyModifierControl|fyne.KeyModifierSuper) == 0 {
				return
			}

			var parts []string
			if heldMods&fyne.KeyModifierControl != 0 {
				parts = append(parts, "Ctrl")
			}
			if heldMods&fyne.KeyModifierAlt != 0 {
				parts = append(parts, "Alt")
			}
			if heldMods&fyne.KeyModifierShift != 0 {
				parts = append(parts, "Shift")
			}
			if heldMods&fyne.KeyModifierSuper != 0 {
				parts = append(parts, "Super")
			}
			parts = append(parts, n)

			stopCapture()
			*target = strings.Join(parts, "-")
			btn.SetText(*target)
			onChange()
		})
	}

	hotkeyBtn := widget.NewButton(cfg.Hotkey, nil)
	hotkeyBtn.OnTapped = func() {
		if activeCapture == hotkeyBtn {
			stopCapture()
			hotkeyBtn.SetText(currentHotkey)
			return
		}
		startCapture(hotkeyBtn, &currentHotkey, func() { updateSaveBtn() })
	}

	undoHotkeyBtn := widget.NewButton(cfg.UndoHotkey, nil)
	undoHotkeyBtn.OnTapped = func() {
		if activeCapture == undoHotkeyBtn {
			stopCapture()
			undoHotkeyBtn.SetText(currentUndoHotkey)
			return
		}
		startCapture(undoHotkeyBtn, &currentUndoHotkey, func() { updateSaveBtn() })
	}

	timeoutEntry := widget.NewEntry()
	timeoutEntry.SetText(strconv.Itoa(cfg.Timeout))
	timeoutEntry.SetPlaceHolder("seconds (default 60)")

	punctCheck := widget.NewCheck("Add punctuation", nil)
	punctCheck.SetChecked(cfg.EnablePunctuation)

	pushToTalkCheck := widget.NewCheck("Push-to-talk (hold key to record, release to submit)", nil)
	pushToTalkCheck.SetChecked(cfg.PushToTalk)

	saveBtn := widget.NewButton("Save", nil)
	saveBtn.Importance = widget.HighImportance
	saveBtn.Disable()

	closeBtn := widget.NewButton("Close", nil)

	currentLang := func() string {
		if code, ok := labelToCode[langSelect.Selected]; ok {
			return code
		}
		return langSelect.Selected
	}

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
			currentUndoHotkey != cfg.UndoHotkey ||
			timeout != cfg.Timeout ||
			punctCheck.Checked != cfg.EnablePunctuation ||
			pushToTalkCheck.Checked != cfg.PushToTalk
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
	pushToTalkCheck.OnChanged = func(_ bool) { updateSaveBtn() }

	doSave := func() {
		timeout, err := strconv.Atoi(timeoutEntry.Text)
		if err != nil || timeout < 5 {
			timeout = cfg.Timeout
		}
		newCfg := &config.Config{
			Hotkey:            currentHotkey,
			UndoHotkey:        currentUndoHotkey,
			Language:          currentLang(),
			Timeout:           timeout,
			SilenceChunks:     int(silenceSlider.Value),
			Sensitivity:       sensitivitySlider.Value,
			APIKey:            apiKeyEntry.Text,
			UseAdvancedAPI:    advancedCheck.Checked,
			EnablePunctuation: punctCheck.Checked,
			PushToTalk:        pushToTalkCheck.Checked,
		}
		onSave(newCfg)
		*cfg = *newCfg
		saveBtn.Disable()
	}

	saveBtn.OnTapped = func() { doSave() }

	tryClose := func() {
		if !hasChanges() {
			stopCapture()
			w.Close()
			return
		}
		dialog.NewCustomConfirm(
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
		).Show()
	}

	closeBtn.OnTapped = tryClose
	w.SetCloseIntercept(tryClose)

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

		widget.NewLabelWithStyle("Undo hotkey", fyne.TextAlignTrailing, fyne.TextStyle{Bold: true}),
		undoHotkeyBtn,

		widget.NewLabelWithStyle("Max duration", fyne.TextAlignTrailing, fyne.TextStyle{Bold: true}),
		container.NewBorder(nil, nil, nil, widget.NewLabel("sec"), timeoutEntry),

		widget.NewLabel(""),
		punctCheck,

		widget.NewLabel(""),
		pushToTalkCheck,
	)

	w.SetContent(container.NewBorder(nil, container.NewHBox(layout.NewSpacer(), closeBtn, saveBtn), nil, nil, container.NewVScroll(form)))
	w.Show()
}
