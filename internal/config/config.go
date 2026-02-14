package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	Hotkey              string `json:"hotkey"`
	Language            string `json:"language"`
	Timeout             int    `json:"timeout"`
	PhraseTimeLimit     int    `json:"phrase_time_limit"`
	EnablePunctuation   bool   `json:"enable_punctuation"`
	EnableNotifications bool   `json:"enable_notifications"`
}

func Default() *Config {
	return &Config{
		Hotkey:              "Alt-d",
		Language:            "en-US",
		Timeout:             30,
		PhraseTimeLimit:     60,
		EnablePunctuation:   true,
		EnableNotifications: true,
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
