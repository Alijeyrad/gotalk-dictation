package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	// Hotkey is the global keyboard shortcut, e.g. "Alt-d".
	Hotkey string `json:"hotkey"`

	// Language is the BCP-47 language code sent to the speech API, e.g. "en-US".
	Language string `json:"language"`

	// Timeout is the maximum total recording+recognition time in seconds.
	Timeout int `json:"timeout"`

	// SilenceChunks is how many consecutive silent audio chunks (each ~62 ms)
	// must follow speech before the phrase is considered finished.
	// Higher = more patient pauses; lower = cuts off faster. Default 12 (~0.75 s).
	SilenceChunks int `json:"silence_chunks"`

	// Sensitivity is the RMS threshold multiplier for speech detection.
	// Lower = more sensitive (picks up quiet voices); higher = ignores more noise.
	// Range 1.0–6.0, default 2.5.
	Sensitivity float64 `json:"sensitivity"`

	// APIKey overrides the default public Chromium key for the free speech API.
	// Leave blank to use the built-in key.
	APIKey string `json:"api_key,omitempty"`

	// UseAdvancedAPI forces use of the Google Cloud Speech API.
	// Requires GOOGLE_APPLICATION_CREDENTIALS or gcloud ADC to be configured.
	UseAdvancedAPI bool `json:"use_advanced_api"`

	// EnablePunctuation adds punctuation to transcripts (typer level).
	EnablePunctuation bool `json:"enable_punctuation"`
}

func Default() *Config {
	return &Config{
		Hotkey:            "Alt-d",
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
	// Clamp loaded values to valid ranges.
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
