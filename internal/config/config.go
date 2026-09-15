// Package config loads and atomically saves local preferences and daily counts.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Channels returns a fresh array of persistent channel identifiers in mixer order.
func Channels() [5]string {
	return [5]string{"rain", "brown", "waves", "birds", "fire"}
}

// Config stores preferences and the daily focus count, never active sessions.
// Version identifies the file schema. Durations contains focus/short/long minutes.
// Volumes maps channel identifiers to percentages; Chime enables completion sound.
// Station is the radio catalog index and RadioVolume is its percentage volume.
// Day is the local date (YYYY-MM-DD) associated with Completed.
type Config struct {
	Version     int            `json:"version"`
	Durations   [3]int         `json:"durations"`
	Volumes     map[string]int `json:"volumes"`
	Chime       bool           `json:"chime"`
	Station     int            `json:"station"`
	RadioVolume int            `json:"radio_volume"`
	Day         string         `json:"day"`
	Completed   int            `json:"completed"`
}

// Default returns initial preferences with an independent volume map.
func Default() Config {
	return Config{
		Version:     1,
		Durations:   [3]int{25, 5, 15},
		Volumes:     map[string]int{"rain": 35, "brown": 0, "waves": 0, "birds": 0, "fire": 0},
		Chime:       true,
		RadioVolume: 50,
	}
}

// Path returns the platform-specific configuration file path without creating it.
// It returns an error if the user configuration directory cannot be determined.
func Path() (string, error) {
	d, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "lofi-chill", "config.json"), nil
}

// Load reads and validates preferences from path. A missing file uses defaults.
// Missing channels in an existing volume map start muted; invalid values are repaired.
// Unreadable, malformed, or newer-schema files return defaults and an error.
// Load never modifies the file.
func Load(path string) (Config, error) {
	c := Default()
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	// Decode into defaults, but use a fresh volume map for migration: new channels stay silent.
	c.Volumes = nil
	if err = json.Unmarshal(b, &c); err != nil {
		return Default(), fmt.Errorf("configuração inválida: %w", err)
	}
	if c.Version > 1 {
		return Default(), fmt.Errorf("versão de configuração não suportada: %d", c.Version)
	}
	defaults := Default()
	for i, n := range c.Durations {
		if n < 1 || n > 120 {
			c.Durations[i] = defaults.Durations[i]
		}
	}
	if c.Volumes == nil {
		c.Volumes = defaults.Volumes
	} else {
		for _, k := range Channels() {
			n := c.Volumes[k]
			if n < 0 || n > 100 {
				n = 0
			}
			c.Volumes[k] = n
		}
	}
	if c.Station < 0 || c.Station > 3 {
		c.Station = 0
	}
	if c.RadioVolume < 0 || c.RadioVolume > 100 {
		c.RadioVolume = 50
	}
	if c.Completed < 0 {
		c.Completed = 0
	}
	c.Version = 1
	return c, nil
}

// Save writes c to path using a private temporary file and an atomic rename.
// It creates parent directories as needed and returns any write or sync error.
// Callers must serialize saves when ordering matters.
func Save(path string, c Config) error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".config-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(append(b, '\n')); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
