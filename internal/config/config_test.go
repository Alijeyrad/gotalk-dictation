package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.Hotkey != "Alt-d" {
		t.Errorf("Hotkey = %q, want %q", cfg.Hotkey, "Alt-d")
	}
	if cfg.UndoHotkey != "Alt-z" {
		t.Errorf("UndoHotkey = %q, want %q", cfg.UndoHotkey, "Alt-z")
	}
	if cfg.Language != "en-US" {
		t.Errorf("Language = %q, want %q", cfg.Language, "en-US")
	}
	if cfg.Timeout != 60 {
		t.Errorf("Timeout = %d, want %d", cfg.Timeout, 60)
	}
	if cfg.SilenceChunks != 12 {
		t.Errorf("SilenceChunks = %d, want %d", cfg.SilenceChunks, 12)
	}
	if cfg.Sensitivity != 2.5 {
		t.Errorf("Sensitivity = %f, want %f", cfg.Sensitivity, 2.5)
	}
	if !cfg.EnablePunctuation {
		t.Error("EnablePunctuation should be true by default")
	}
}

func TestLoadNonExistent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error for non-existent file: %v", err)
	}
	def := Default()
	if cfg.Hotkey != def.Hotkey || cfg.Language != def.Language || cfg.Timeout != def.Timeout {
		t.Error("Load() with missing file should return defaults")
	}
}

func TestLoadMalformedJSON(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cfgDir := filepath.Join(tmp, ".config", "gotalk-dictation")
	os.MkdirAll(cfgDir, 0755)
	os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte(`{"hotkey": `), 0644)

	_, err := Load()
	if err == nil {
		t.Error("Load() should return error for malformed JSON")
	}
}

func TestSaveRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg := Default()
	cfg.Language = "fr-FR"
	cfg.Timeout = 30
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded.Language != "fr-FR" {
		t.Errorf("Language = %q, want %q", loaded.Language, "fr-FR")
	}
	if loaded.Timeout != 30 {
		t.Errorf("Timeout = %d, want %d", loaded.Timeout, 30)
	}
}

func TestSaveCreatesDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg := Default()
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	if _, err := os.Stat(path()); err != nil {
		t.Errorf("config file not created: %v", err)
	}
}

func TestSaveFilePermissions(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg := Default()
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	info, err := os.Stat(path())
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	if info.Mode().Perm() != 0644 {
		t.Errorf("file permissions = %04o, want 0644", info.Mode().Perm())
	}
}

func TestLoadClamping(t *testing.T) {
	tests := []struct {
		name  string
		json  string
		check func(*testing.T, *Config)
	}{
		{
			name: "SilenceChunks 0 clamped to 12",
			json: `{"silence_chunks":0}`,
			check: func(t *testing.T, c *Config) {
				if c.SilenceChunks != 12 {
					t.Errorf("SilenceChunks = %d, want 12", c.SilenceChunks)
				}
			},
		},
		{
			name: "SilenceChunks 1 not clamped",
			json: `{"silence_chunks":1}`,
			check: func(t *testing.T, c *Config) {
				if c.SilenceChunks != 1 {
					t.Errorf("SilenceChunks = %d, want 1", c.SilenceChunks)
				}
			},
		},
		{
			name: "Sensitivity 0.1 clamped to 2.5",
			json: `{"sensitivity":0.1}`,
			check: func(t *testing.T, c *Config) {
				if c.Sensitivity != 2.5 {
					t.Errorf("Sensitivity = %f, want 2.5", c.Sensitivity)
				}
			},
		},
		{
			name: "Sensitivity 0.5 not clamped",
			json: `{"sensitivity":0.5}`,
			check: func(t *testing.T, c *Config) {
				if c.Sensitivity != 0.5 {
					t.Errorf("Sensitivity = %f, want 0.5", c.Sensitivity)
				}
			},
		},
		{
			name: "Timeout 2 clamped to 60",
			json: `{"timeout":2}`,
			check: func(t *testing.T, c *Config) {
				if c.Timeout != 60 {
					t.Errorf("Timeout = %d, want 60", c.Timeout)
				}
			},
		},
		{
			name: "Timeout 5 not clamped",
			json: `{"timeout":5}`,
			check: func(t *testing.T, c *Config) {
				if c.Timeout != 5 {
					t.Errorf("Timeout = %d, want 5", c.Timeout)
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			t.Setenv("HOME", tmp)
			cfgDir := filepath.Join(tmp, ".config", "gotalk-dictation")
			os.MkdirAll(cfgDir, 0755)
			os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte(tc.json), 0644)
			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error: %v", err)
			}
			tc.check(t, cfg)
		})
	}
}

func TestLoadPartialJSON(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cfgDir := filepath.Join(tmp, ".config", "gotalk-dictation")
	os.MkdirAll(cfgDir, 0755)
	// Only set language; other fields should remain at defaults.
	os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte(`{"language":"de-DE"}`), 0644)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Language != "de-DE" {
		t.Errorf("Language = %q, want %q", cfg.Language, "de-DE")
	}
	// Other fields should still be defaults.
	def := Default()
	if cfg.Hotkey != def.Hotkey {
		t.Errorf("Hotkey = %q, want default %q", cfg.Hotkey, def.Hotkey)
	}
	if cfg.Timeout != def.Timeout {
		t.Errorf("Timeout = %d, want default %d", cfg.Timeout, def.Timeout)
	}
}

func TestSaveProducesValidJSON(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	cfg := Default()
	cfg.Language = "ja-JP"
	cfg.SilenceChunks = 8
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	data, err := os.ReadFile(path())
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	var decoded Config
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}
	if decoded.Language != "ja-JP" {
		t.Errorf("Language = %q, want %q", decoded.Language, "ja-JP")
	}
	if decoded.SilenceChunks != 8 {
		t.Errorf("SilenceChunks = %d, want 8", decoded.SilenceChunks)
	}
}
