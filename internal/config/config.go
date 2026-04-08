package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	WorkDirs []string `json:"work_dirs"`
}

func path() string {
	return filepath.Join(os.Getenv("HOME"), ".conch", "config.json")
}

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

// FindRepos walks each work dir one level deep and returns paths that contain a .git directory.
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
