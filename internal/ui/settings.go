package ui

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/Alijeyrad/gotalk-dictation/internal/config"
	"github.com/Alijeyrad/gotalk-dictation/internal/version"
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

func showSettingsWindow(fyneApp fyne.App, cfg *config.Config, onSave func(*config.Config)) fyne.Window {
	w := fyneApp.NewWindow("GoTalk Dictation — Settings")
	w.SetIcon(fyne.NewStaticResource("icon.png", iconPNG))
	w.Resize(fyne.NewSize(480, 520))

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
	currentPTTHotkey := cfg.PTTHotkey

	pttBtnLabel := func(h string) string {
		if h == "" {
			return "(not set — click to assign)"
		}
		return h
	}

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
	startCapture := func(btn *widget.Button, target *string, displayFn func(string) string, onChange func()) {
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
				if displayFn != nil {
					btn.SetText(displayFn(*target))
				} else {
					btn.SetText(*target)
				}
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
			if displayFn != nil {
				btn.SetText(displayFn(*target))
			} else {
				btn.SetText(*target)
			}
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
		startCapture(hotkeyBtn, &currentHotkey, nil, func() { updateSaveBtn() })
	}

	undoHotkeyBtn := widget.NewButton(cfg.UndoHotkey, nil)
	undoHotkeyBtn.OnTapped = func() {
		if activeCapture == undoHotkeyBtn {
			stopCapture()
			undoHotkeyBtn.SetText(currentUndoHotkey)
			return
		}
		startCapture(undoHotkeyBtn, &currentUndoHotkey, nil, func() { updateSaveBtn() })
	}

	pttHotkeyBtn := widget.NewButton(pttBtnLabel(cfg.PTTHotkey), nil)
	pttClearBtn := widget.NewButton("Clear", func() {
		stopCapture()
		currentPTTHotkey = ""
		pttHotkeyBtn.SetText(pttBtnLabel(""))
		updateSaveBtn()
	})
	pttClearBtn.Importance = widget.LowImportance
	if currentPTTHotkey == "" {
		pttClearBtn.Disable()
	}
	pttHotkeyBtn.OnTapped = func() {
		if activeCapture == pttHotkeyBtn {
			stopCapture()
			pttHotkeyBtn.SetText(pttBtnLabel(currentPTTHotkey))
			return
		}
		startCapture(pttHotkeyBtn, &currentPTTHotkey, pttBtnLabel, func() {
			pttClearBtn.Enable()
			updateSaveBtn()
		})
	}

	timeoutEntry := widget.NewEntry()
	timeoutEntry.SetText(strconv.Itoa(cfg.Timeout))
	timeoutEntry.SetPlaceHolder("seconds (default 60)")

	punctCheck := widget.NewCheck("Add punctuation", nil)
	punctCheck.SetChecked(cfg.EnablePunctuation)

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
			currentPTTHotkey != cfg.PTTHotkey ||
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

	doSave := func() {
		timeout, err := strconv.Atoi(timeoutEntry.Text)
		if err != nil || timeout < 5 {
			timeout = cfg.Timeout
		}
		newCfg := &config.Config{
			Hotkey:            currentHotkey,
			UndoHotkey:        currentUndoHotkey,
			PTTHotkey:         currentPTTHotkey,
			Language:          currentLang(),
			Timeout:           timeout,
			SilenceChunks:     int(silenceSlider.Value),
			Sensitivity:       sensitivitySlider.Value,
			APIKey:            apiKeyEntry.Text,
			UseAdvancedAPI:    advancedCheck.Checked,
			EnablePunctuation: punctCheck.Checked,
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

	pttHint := widget.NewLabelWithStyle(
		"Hold this key to record; release to transcribe. Works alongside the toggle hotkey.",
		fyne.TextAlignLeading, fyne.TextStyle{Italic: true},
	)

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

		widget.NewLabelWithStyle("Toggle hotkey", fyne.TextAlignTrailing, fyne.TextStyle{Bold: true}),
		hotkeyBtn,

		widget.NewLabelWithStyle("Push-to-talk", fyne.TextAlignTrailing, fyne.TextStyle{Bold: true}),
		container.NewBorder(nil, nil, nil, pttClearBtn, pttHotkeyBtn),

		widget.NewLabel(""),
		pttHint,

		widget.NewLabelWithStyle("Undo hotkey", fyne.TextAlignTrailing, fyne.TextStyle{Bold: true}),
		undoHotkeyBtn,

		widget.NewLabelWithStyle("Max duration", fyne.TextAlignTrailing, fyne.TextStyle{Bold: true}),
		container.NewBorder(nil, nil, nil, widget.NewLabel("sec"), timeoutEntry),

		widget.NewLabel(""),
		punctCheck,
	)

	versionLabel := widget.NewLabelWithStyle(
		fmt.Sprintf("v%s (%s)", version.Version, version.Commit),
		fyne.TextAlignCenter,
		fyne.TextStyle{Italic: true},
	)

	w.SetContent(container.NewBorder(
		nil,
		container.NewVBox(
			versionLabel,
			container.NewHBox(layout.NewSpacer(), closeBtn, saveBtn),
		),
		nil, nil,
		container.NewVScroll(form),
	))
	w.Show()
	return w
}

func showAboutWindow(fyneApp fyne.App) fyne.Window {
	w := fyneApp.NewWindow("About GoTalk Dictation")
	w.SetIcon(fyne.NewStaticResource("icon.png", iconPNG))
	w.Resize(fyne.NewSize(360, 300))
	w.SetFixedSize(true)

	icon := fyne.NewStaticResource("icon.png", iconPNG)
	iconWidget := widget.NewIcon(icon)

	title := widget.NewLabelWithStyle("GoTalk Dictation",
		fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	ver := widget.NewLabelWithStyle(
		fmt.Sprintf("Version %s  ·  commit %s", version.Version, version.Commit),
		fyne.TextAlignCenter, fyne.TextStyle{Italic: true})
	desc := widget.NewLabelWithStyle(
		"System-wide speech-to-text dictation for Linux",
		fyne.TextAlignCenter, fyne.TextStyle{})
	desc.Wrapping = fyne.TextWrapWord

	ghURL, _ := url.Parse("https://github.com/Alijeyrad/gotalk-dictation")
	issuesURL, _ := url.Parse("https://github.com/Alijeyrad/gotalk-dictation/issues")

	links := container.NewHBox(
		layout.NewSpacer(),
		widget.NewHyperlink("GitHub", ghURL),
		widget.NewLabel("·"),
		widget.NewHyperlink("Report an issue", issuesURL),
		layout.NewSpacer(),
	)

	license := widget.NewLabelWithStyle("MIT License · © Ali Julaee Rad",
		fyne.TextAlignCenter, fyne.TextStyle{})

	closeBtn := widget.NewButton("Close", func() { w.Close() })
	closeBtn.Importance = widget.LowImportance

	content := container.NewVBox(
		container.NewCenter(iconWidget),
		title,
		ver,
		widget.NewSeparator(),
		desc,
		links,
		widget.NewSeparator(),
		license,
		container.NewCenter(closeBtn),
	)

	w.SetContent(container.NewPadded(content))
	w.Show()
	return w
}
