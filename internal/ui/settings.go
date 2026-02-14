package ui

import (
	"fmt"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/Alijeyrad/gotalk-dictation/internal/config"
)

// OpenSettings shows the settings window.
// Must be called from the Fyne main goroutine (e.g. inside a menu callback).
func OpenSettings(fyneApp fyne.App, cfg *config.Config, onSave func(*config.Config)) {
	showSettingsWindow(fyneApp, cfg, onSave)
}

func showSettingsWindow(fyneApp fyne.App, cfg *config.Config, onSave func(*config.Config)) {
	w := fyneApp.NewWindow("GoTalk Dictation — Settings")
	w.SetIcon(fyne.NewStaticResource("icon.png", iconPNG))
	w.Resize(fyne.NewSize(460, 520))
	w.SetFixedSize(true)

	// ---- Speech recognition ----
	langEntry := widget.NewEntry()
	langEntry.SetText(cfg.Language)
	langEntry.SetPlaceHolder("e.g. en-US, fa-IR, de-DE")

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

	// ---- General ----
	hotkeyEntry := widget.NewEntry()
	hotkeyEntry.SetText(cfg.Hotkey)
	hotkeyEntry.SetPlaceHolder("e.g. Alt-d, Ctrl-space")

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

	// hasChanges returns true if any widget differs from the original cfg.
	hasChanges := func() bool {
		timeout, err := strconv.Atoi(timeoutEntry.Text)
		if err != nil || timeout < 5 {
			timeout = cfg.Timeout
		}
		return langEntry.Text != cfg.Language ||
			apiKeyEntry.Text != cfg.APIKey ||
			advancedCheck.Checked != cfg.UseAdvancedAPI ||
			int(silenceSlider.Value) != cfg.SilenceChunks ||
			fmt.Sprintf("%.1f", sensitivitySlider.Value) != fmt.Sprintf("%.1f", cfg.Sensitivity) ||
			hotkeyEntry.Text != cfg.Hotkey ||
			timeout != cfg.Timeout ||
			punctCheck.Checked != cfg.EnablePunctuation
	}

	// updateSaveBtn enables or disables Save based on whether anything changed.
	updateSaveBtn := func() {
		if hasChanges() {
			saveBtn.Enable()
		} else {
			saveBtn.Disable()
		}
	}

	// Wire all widgets to updateSaveBtn.
	langEntry.OnChanged = func(_ string) { updateSaveBtn() }
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
	hotkeyEntry.OnChanged = func(_ string) { updateSaveBtn() }
	timeoutEntry.OnChanged = func(_ string) { updateSaveBtn() }
	punctCheck.OnChanged = func(_ bool) { updateSaveBtn() }

	// doSave builds the new config from current widget values and calls onSave.
	doSave := func() {
		timeout, err := strconv.Atoi(timeoutEntry.Text)
		if err != nil || timeout < 5 {
			timeout = cfg.Timeout
		}
		newCfg := &config.Config{
			Hotkey:            hotkeyEntry.Text,
			Language:          langEntry.Text,
			Timeout:           timeout,
			SilenceChunks:     int(silenceSlider.Value),
			Sensitivity:       sensitivitySlider.Value,
			APIKey:            apiKeyEntry.Text,
			UseAdvancedAPI:    advancedCheck.Checked,
			EnablePunctuation: punctCheck.Checked,
		}
		onSave(newCfg)
		// Update cfg baseline so Save disables again and hasChanges is accurate.
		*cfg = *newCfg
		saveBtn.Disable()
	}

	saveBtn.OnTapped = func() { doSave() }

	// tryClose closes the window, but prompts first if there are unsaved changes.
	tryClose := func() {
		if !hasChanges() {
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
		langEntry,

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
		hotkeyEntry,

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
