package typing

import "testing"

func TestProcessPunctuationNoOp(t *testing.T) {
	got := processPunctuation("hello world")
	if got != "hello world" {
		t.Errorf("got %q, want %q", got, "hello world")
	}
}

func TestProcessPunctuationSingleWords(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"period", "."},
		{"comma", ","},
		{"colon", ":"},
		{"semicolon", ";"},
		{"dash", "-"},
		{"hyphen", "-"},
		{"ellipsis", "..."},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := processPunctuation(tc.input)
			if got != tc.want {
				t.Errorf("processPunctuation(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestProcessPunctuationTwoWordKeys(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"question mark", "?"},
		{"exclamation mark", "!"},
		{"exclamation point", "!"},
		{"new line", "\n"},
		{"new paragraph", "\n\n"},
		{"open parenthesis", "("},
		{"close parenthesis", ")"},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := processPunctuation(tc.input)
			if got != tc.want {
				t.Errorf("processPunctuation(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestProcessPunctuationMidSentence(t *testing.T) {
	got := processPunctuation("hello comma world period")
	want := "hello , world ."
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestProcessPunctuationCaseInsensitive(t *testing.T) {
	got := processPunctuation("PERIOD")
	if got != "." {
		t.Errorf("got %q, want %q", got, ".")
	}
}

func TestProcessPunctuationTwoWordPriority(t *testing.T) {
	// "question mark" should be consumed as a two-word unit.
	got := processPunctuation("question mark")
	if got != "?" {
		t.Errorf("got %q, want %q", got, "?")
	}
}

func TestProcessPunctuationSpaceAroundNewline(t *testing.T) {
	// "new line" should insert a newline with no extra spaces around it.
	got := processPunctuation("first new line second")
	want := "first\nsecond"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestProcessPunctuationNewParagraph(t *testing.T) {
	got := processPunctuation("first new paragraph second")
	want := "first\n\nsecond"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestProcessPunctuationEmpty(t *testing.T) {
	got := processPunctuation("")
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

func TestTyperUndoZeroCount(t *testing.T) {
	ty := &Typer{}
	// lastRuneCount is 0, so Undo() should return nil without invoking xdotool.
	if err := ty.Undo(); err != nil {
		t.Errorf("Undo() with zero count returned error: %v", err)
	}
}
