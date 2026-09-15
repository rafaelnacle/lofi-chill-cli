package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

var Channels = []string{"rain", "brown", "waves", "birds", "fire"}

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

func Default() Config {
	return Config{Version: 1, Durations: [3]int{25, 5, 15}, Volumes: map[string]int{"rain": 35, "brown": 0, "waves": 0, "birds": 0, "fire": 0}, Chime: true, RadioVolume: 50}
}
func Path() (string, error) {
	d, e := os.UserConfigDir()
	if e != nil {
		return "", e
	}
	return filepath.Join(d, "lofi-chill", "config.json"), nil
}
func Load(path string) (Config, error) {
	c := Default()
	b, e := os.ReadFile(path)
	if os.IsNotExist(e) {
		return c, nil
	}
	if e != nil {
		return c, e
	}
	// Decode into defaults, but use a fresh volume map for migration: new channels stay silent.
	c.Volumes = nil
	if e = json.Unmarshal(b, &c); e != nil {
		return Default(), fmt.Errorf("configuração inválida: %w", e)
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
		for _, k := range Channels {
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
func Save(path string, c Config) error {
	b, e := json.MarshalIndent(c, "", "  ")
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".config-*")
	if e != nil {
		return e
	}
	name := f.Name()
	defer os.Remove(name)
	if _, e = f.Write(append(b, '\n')); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(name, path)
}
