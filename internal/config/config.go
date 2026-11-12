// Package config loads and saves the Botler settings file.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

const systemSoundsDir = "/System/Library/Sounds"

// Notification kinds, one per Claude Code hook outcome.
const (
	KindDone   = "done"
	KindFailed = "failed"
	KindInput  = "input"
)

// KindSettings configures how one notification kind is announced.
type KindSettings struct {
	// Enabled switches the whole event off; Banner and Sound only apply while it is on.
	Enabled   bool   `json:"enabled"`
	Banner    bool   `json:"banner"`
	Sound     bool   `json:"sound"`
	SoundName string `json:"soundName"`
	// AutoHide is how many seconds a banner stays up; 0 keeps it until dismissed.
	AutoHide int `json:"autoHide"`
}

// Config is shared by the menu-bar app and the hook CLI.
type Config struct {
	Done          KindSettings `json:"done"`
	Failed        KindSettings `json:"failed"`
	Input         KindSettings `json:"input"`
	BannerPos     string       `json:"bannerPosition"`
	OnlyWhenAway  bool         `json:"onlyWhenAway"`
	TrayFlash     bool         `json:"trayFlash"`
	TrayMode      string       `json:"trayMode"`
	HooksPrompted bool         `json:"hooksPrompted"`
}

// Banner positions.
const (
	BannerTopRight    = "topRight"
	BannerTopCenter   = "topCenter"
	BannerTopLeft     = "topLeft"
	BannerBottomRight = "bottomRight"
	BannerBottomLeft  = "bottomLeft"
)

// Tray icon visibility modes.
const (
	TrayAlways   = "always"
	TrayOnNotify = "onNotify"
	TrayNever    = "never"
)

// Default returns the settings used when no config file exists.
func Default() Config {
	return Config{
		Done:         KindSettings{Enabled: true, Banner: true, Sound: true, SoundName: "Glass", AutoHide: 10},
		Failed:       KindSettings{Enabled: true, Banner: true, Sound: true, SoundName: "Basso"},
		Input:        KindSettings{Enabled: true, Banner: true, Sound: true, SoundName: "Ping"},
		BannerPos:    BannerTopRight,
		OnlyWhenAway: true,
		TrayFlash:    true,
		TrayMode:     TrayAlways,
	}
}

// Kind returns the settings for kind, falling back to Done for unknown kinds.
func (c Config) Kind(kind string) KindSettings {
	switch kind {
	case KindFailed:
		return c.Failed
	case KindInput:
		return c.Input
	default:
		return c.Done
	}
}

// TrayHidden reports whether the tray icon should stay hidden while idle.
func (c Config) TrayHidden() bool {
	return c.TrayMode == TrayOnNotify || c.TrayMode == TrayNever
}

// Dir returns ~/.config/botler.
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".config", "botler"), nil
}

func path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Load reads the config file, falling back to Default when it does not exist.
func Load() (Config, error) {
	p, err := path()
	if err != nil {
		return Default(), err
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Default(), fmt.Errorf("read config: %w", err)
	}
	cfg := Default()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Default(), fmt.Errorf("parse %s: %w", p, err)
	}
	return cfg, nil
}

// Save writes via temp file and rename because the CLI may read it concurrently.
func Save(cfg Config) error {
	p, err := path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if err := os.Rename(tmp, p); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}

// SoundPath maps a sound name to its file: custom sounds keep their extension, system sounds such as "Glass" have none.
func SoundPath(name string) string {
	if filepath.Ext(name) == "" {
		return filepath.Join(systemSoundsDir, name+".aiff")
	}
	dir, err := SoundsDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, filepath.Base(name))
}

// SoundsDir returns the directory holding user-added sounds.
func SoundsDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "sounds"), nil
}

// AudioExtensions lists the file types afplay can play.
var AudioExtensions = []string{".aiff", ".aif", ".caf", ".m4a", ".mp3", ".wav"}

// CustomSounds lists the user-added sound file names.
func CustomSounds() ([]string, error) {
	dir, err := SoundsDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list custom sounds: %w", err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && slices.Contains(AudioExtensions, strings.ToLower(filepath.Ext(e.Name()))) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// ImportSound copies src into the sounds directory and returns its sound name.
func ImportSound(src string) (string, error) {
	if !slices.Contains(AudioExtensions, strings.ToLower(filepath.Ext(src))) {
		return "", fmt.Errorf("unsupported audio type %q", filepath.Ext(src))
	}
	dir, err := SoundsDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create sounds dir: %w", err)
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return "", fmt.Errorf("read sound: %w", err)
	}
	name := filepath.Base(src)
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		return "", fmt.Errorf("write sound: %w", err)
	}
	return name, nil
}

// RemoveSound deletes a user-added sound.
func RemoveSound(name string) error {
	if filepath.Ext(name) == "" {
		return fmt.Errorf("%q is a system sound", name)
	}
	dir, err := SoundsDir()
	if err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(dir, filepath.Base(name))); err != nil {
		return fmt.Errorf("remove sound: %w", err)
	}
	return nil
}

// SystemSounds lists the sound names available in /System/Library/Sounds.
func SystemSounds() ([]string, error) {
	entries, err := os.ReadDir(systemSoundsDir)
	if err != nil {
		return nil, fmt.Errorf("list system sounds: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if name, ok := strings.CutSuffix(e.Name(), ".aiff"); ok {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, nil
}
