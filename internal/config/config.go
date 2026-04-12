// Package config loads and saves ~/.conch/config.json.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	WorkDirs []string `json:"work_dirs"`
	SlugMode string   `json:"slug_mode,omitempty"`
}

// EffectiveSlugMode returns the configured slug mode, defaulting to "slugineer".
func (c Config) EffectiveSlugMode() string {
	if c.SlugMode == "" {
		return "slugineer"
	}
	return c.SlugMode
}

func path() string {
	return filepath.Join(os.Getenv("HOME"), ".conch", "config.json")
}

// Load reads the config file. Returns an empty Config (not an error) when the file does not exist.
func Load() (Config, error) {
	data, err := os.ReadFile(path())
	if os.IsNotExist(err) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, err
	}
	var c Config
	return c, json.Unmarshal(data, &c)
}

func Save(c Config) error {
	p := path()
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}

// FindRepos walks each work dir exactly one level deep. A directory is considered
// a repo if it contains a .git entry.
func FindRepos(workDirs []string) ([]string, error) {
	var repos []string
	for _, dir := range workDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			candidate := filepath.Join(dir, e.Name())
			if _, err := os.Stat(filepath.Join(candidate, ".git")); err == nil {
				repos = append(repos, candidate)
			}
		}
	}
	return repos, nil
}
