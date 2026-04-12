package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadConfig_defaults: missing file → empty Config, no error, EffectiveSlugMode defaults to "slugineer".
func TestLoadConfig_defaults(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.WorkDirs) != 0 {
		t.Fatalf("expected empty WorkDirs, got %v", c.WorkDirs)
	}
	if c.EffectiveSlugMode() != "slugineer" {
		t.Fatalf("expected default slugineer, got %q", c.EffectiveSlugMode())
	}
}

// TestLoadConfig_overrides: full JSON config → all fields read correctly.
func TestLoadConfig_overrides(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	want := Config{WorkDirs: []string{"/code/a", "/code/b"}, SlugMode: "lite"}
	if err := Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.WorkDirs) != 2 || got.WorkDirs[0] != "/code/a" || got.WorkDirs[1] != "/code/b" {
		t.Fatalf("unexpected WorkDirs: %v", got.WorkDirs)
	}
	if got.SlugMode != "lite" {
		t.Fatalf("expected slug_mode lite, got %q", got.SlugMode)
	}
}

// TestLoadConfig_missingFile: no config file → Load returns empty Config with no error.
func TestLoadConfig_missingFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	c, err := Load()
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if len(c.WorkDirs) != 0 || c.SlugMode != "" {
		t.Fatalf("expected zero-value Config, got %+v", c)
	}
}

func TestLoadMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.WorkDirs) != 0 {
		t.Fatal("expected empty config")
	}
}

func TestSaveLoad(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	os.MkdirAll(filepath.Join(dir, ".conch"), 0755)

	want := Config{WorkDirs: []string{"/tmp/code", "/tmp/work"}, SlugMode: "lite"}
	if err := Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.WorkDirs) != 2 || got.WorkDirs[0] != want.WorkDirs[0] {
		t.Fatalf("got %v", got.WorkDirs)
	}
	if got.SlugMode != "lite" {
		t.Fatalf("expected slug_mode lite, got %q", got.SlugMode)
	}
}

func TestEffectiveSlugMode(t *testing.T) {
	if (Config{}).EffectiveSlugMode() != "slugineer" {
		t.Fatal("expected default slugineer")
	}
	if (Config{SlugMode: "off"}).EffectiveSlugMode() != "off" {
		t.Fatal("expected off")
	}
}

func TestFindRepos(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"alpha", "beta"} {
		os.MkdirAll(filepath.Join(dir, name, ".git"), 0755)
	}
	os.MkdirAll(filepath.Join(dir, "notarepo"), 0755)

	repos, err := FindRepos([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 2 {
		t.Fatalf("expected 2 repos, got %d: %v", len(repos), repos)
	}
}
