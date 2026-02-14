//go:build x11test

package typing

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	if os.Getenv("DISPLAY") == "" {
		// No X11 display: skip all X11 tests.
		os.Exit(0)
	}
	for _, tool := range []string{"xdotool", "xclip"} {
		if _, err := exec.LookPath(tool); err != nil {
			// Required tool not found: skip all X11 tests.
			os.Exit(0)
		}
	}
	os.Exit(m.Run())
}

func TestTypeShortText(t *testing.T) {
	ty := &Typer{}
	if err := ty.Type("hi"); err != nil {
		t.Fatalf("Type() error: %v", err)
	}
	if ty.lastRuneCount != 2 {
		t.Errorf("lastRuneCount = %d, want 2", ty.lastRuneCount)
	}
}

func TestTypeLongTextUsesClipboard(t *testing.T) {
	ty := &Typer{}
	// 60-char string exceeds clipboardThreshold (50).
	text := strings.Repeat("a", 60)
	if err := ty.Type(text); err != nil {
		t.Fatalf("Type() error for long text: %v", err)
	}
	if ty.lastRuneCount != 60 {
		t.Errorf("lastRuneCount = %d, want 60", ty.lastRuneCount)
	}
}

func TestTypeWithPunctuation(t *testing.T) {
	ty := &Typer{EnablePunctuation: true}
	if err := ty.Type("hello period"); err != nil {
		t.Fatalf("Type() error: %v", err)
	}
}

func TestUndoAfterType(t *testing.T) {
	ty := &Typer{}
	if err := ty.Type("abc"); err != nil {
		t.Fatalf("Type() error: %v", err)
	}
	if err := ty.Undo(); err != nil {
		t.Fatalf("Undo() error: %v", err)
	}
	// After undo, lastRuneCount should be reset to 0.
	if ty.lastRuneCount != 0 {
		t.Errorf("lastRuneCount after Undo = %d, want 0", ty.lastRuneCount)
	}
}
