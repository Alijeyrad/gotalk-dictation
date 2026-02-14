package typing

import (
	"os/exec"
	"strings"
)

// punctuationMap maps spoken words to punctuation characters.
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

// Typer inserts text at the current cursor position using xdotool.
type Typer struct {
	EnablePunctuation bool
}

// Type inserts the given text at the cursor, optionally processing punctuation commands.
func (t *Typer) Type(text string) error {
	if t.EnablePunctuation {
		text = processPunctuation(text)
	}
	return exec.Command("xdotool", "type", "--clearmodifiers", "--", text).Run()
}

// processPunctuation replaces spoken punctuation words with their symbols.
// It scans word by word, trying two-word combinations first.
func processPunctuation(text string) string {
	words := strings.Fields(text)
	result := make([]string, 0, len(words))
	i := 0
	for i < len(words) {
		// Try two-word combination first
		if i+1 < len(words) {
			twoWord := strings.ToLower(words[i] + " " + words[i+1])
			if punct, ok := punctuationMap[twoWord]; ok {
				result = append(result, punct)
				i += 2
				continue
			}
		}
		// Single word
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
