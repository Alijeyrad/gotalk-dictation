package typing

import (
	"strings"
	"sync"
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

	once sync.Once
	x    *x11Typer
}

func (t *Typer) init() {
	t.once.Do(func() {
		x, err := newX11Typer()
		if err != nil {
			return
		}
		t.x = x
	})
}

func (t *Typer) Type(text string) error {
	if t.EnablePunctuation {
		text = processPunctuation(text)
	}
	t.lastRuneCount = len([]rune(text))
	t.init()
	if t.x == nil {
		return errNoX11
	}
	if t.lastRuneCount >= clipboardThreshold {
		return t.typeViaClipboard(text)
	}
	t.x.typeString(text)
	return nil
}

func (t *Typer) typeViaClipboard(text string) error {
	t.x.setClipboardAndPaste(text)
	return nil
}

func (t *Typer) Undo() error {
	if t.lastRuneCount == 0 {
		return nil
	}
	n := t.lastRuneCount
	t.lastRuneCount = 0
	t.init()
	if t.x == nil {
		return errNoX11
	}
	t.x.sendBackspaces(n)
	return nil
}

var errNoX11 = &x11Error{}

type x11Error struct{}

func (e *x11Error) Error() string { return "X11 connection unavailable" }

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
	var out strings.Builder
	for i, token := range result {
		if i > 0 {
			prev := result[i-1]
			if strings.TrimSpace(prev) != "" && strings.TrimSpace(token) != "" {
				out.WriteByte(' ')
			}
		}
		out.WriteString(token)
	}
	return out.String()
}
