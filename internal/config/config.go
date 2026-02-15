package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	Hotkey     string `json:"hotkey"`
	UndoHotkey string `json:"undo_hotkey"`
	// PTTHotkey is a separate push-to-talk hotkey (hold to record, release to
	// transcribe). When non-empty it operates independently of Hotkey so both
	// toggle and PTT can be used at the same time.
	PTTHotkey string `json:"ptt_hotkey"`
	Language  string `json:"language"`
	Timeout   int    `json:"timeout"`

	// SilenceChunks is consecutive silent chunks (~62 ms each) that end a phrase.
	SilenceChunks int `json:"silence_chunks"`

	// Sensitivity is the RMS threshold multiplier (lower = more sensitive, range 1–6).
	Sensitivity float64 `json:"sensitivity"`

	// APIKey overrides the built-in Chromium key for the free speech API.
	APIKey            string `json:"api_key,omitempty"`
	UseAdvancedAPI    bool   `json:"use_advanced_api"`
	EnablePunctuation bool   `json:"enable_punctuation"`
}

func Default() *Config {
	return &Config{
		Hotkey:            "Alt-d",
		UndoHotkey:        "Alt-z",
		Language:          "en-US",
		Timeout:           60,
		SilenceChunks:     12,
		Sensitivity:       2.5,
		EnablePunctuation: true,
	}
}

func dir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "gotalk-dictation")
}

func path() string {
	return filepath.Join(dir(), "config.json")
}

func Load() (*Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path())
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	if cfg.SilenceChunks < 1 {
		cfg.SilenceChunks = 12
	}
	if cfg.Sensitivity < 0.5 {
		cfg.Sensitivity = 2.5
	}
	if cfg.Timeout < 5 {
		cfg.Timeout = 60
	}
	return cfg, nil
}

func (c *Config) Save() error {
	if err := os.MkdirAll(dir(), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path(), data, 0644)
}
