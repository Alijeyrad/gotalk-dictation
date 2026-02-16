//go:build x11test

package typing

import (
	"os"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	if os.Getenv("DISPLAY") == "" {
		os.Exit(0)
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
	if ty.lastRuneCount != 0 {
		t.Errorf("lastRuneCount after Undo = %d, want 0", ty.lastRuneCount)
	}
}
