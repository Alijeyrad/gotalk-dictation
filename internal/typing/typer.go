package typing

import (
	"bytes"
	"os/exec"
	"strings"
)

var punctuationMap = map[string]string{
	"period":            ".",
	"comma":             ",",
	"question mark":     "?",
	"exclamation mark":  "!",
	"exclamation point": "!",
	"colon":             ":",
	"semicolon":         ";",
	"new line":          "\n",
	"new paragraph":     "\n\n",
	"open parenthesis":  "(",
	"close parenthesis": ")",
	"dash":              "-",
	"hyphen":            "-",
	"ellipsis":          "...",
}

const clipboardThreshold = 50 // chars; above this use clipboard paste

type Typer struct {
	EnablePunctuation bool
	lastRuneCount     int
}

func (t *Typer) Type(text string) error {
	if t.EnablePunctuation {
		text = processPunctuation(text)
	}
	t.lastRuneCount = len([]rune(text))
	if t.lastRuneCount >= clipboardThreshold {
		return typeViaClipboard(text)
	}
	return exec.Command("xdotool", "type", "--clearmodifiers", "--delay", "0", "--", text).Run()
}

// typeViaClipboard saves the current clipboard, writes text to it, pastes,
// then restores the original clipboard contents.
func typeViaClipboard(text string) error {
	// Save current clipboard.
	saved, _ := exec.Command("xclip", "-selection", "clipboard", "-o").Output()

	// Write new text to clipboard.
	cmd := exec.Command("xclip", "-selection", "clipboard")
	cmd.Stdin = bytes.NewReader([]byte(text))
	if err := cmd.Run(); err != nil {
		// xclip not available — fall back to xdotool.
		return exec.Command("xdotool", "type", "--clearmodifiers", "--delay", "0", "--", text).Run()
	}

	// Paste.
	if err := exec.Command("xdotool", "key", "--clearmodifiers", "ctrl+v").Run(); err != nil {
		return err
	}

	// Restore original clipboard (best-effort).
	if len(saved) > 0 {
		restore := exec.Command("xclip", "-selection", "clipboard")
		restore.Stdin = bytes.NewReader(saved)
		restore.Run() //nolint:errcheck
	}
	return nil
}

func (t *Typer) Undo() error {
	if t.lastRuneCount == 0 {
		return nil
	}
	// Build a BackSpace key sequence.
	keys := make([]string, t.lastRuneCount)
	for i := range keys {
		keys[i] = "BackSpace"
	}
	t.lastRuneCount = 0
	return exec.Command("xdotool", append([]string{"key", "--clearmodifiers", "--delay", "0"}, keys...)...).Run()
}

func processPunctuation(text string) string {
	words := strings.Fields(text)
	result := make([]string, 0, len(words))
	i := 0
	for i < len(words) {
		if i+1 < len(words) {
			twoWord := strings.ToLower(words[i] + " " + words[i+1])
			if punct, ok := punctuationMap[twoWord]; ok {
				result = append(result, punct)
				i += 2
				continue
			}
		}
		oneWord := strings.ToLower(words[i])
		if punct, ok := punctuationMap[oneWord]; ok {
			result = append(result, punct)
		} else {
			result = append(result, words[i])
		}
		i++
	}
	return strings.Join(result, " ")
}
