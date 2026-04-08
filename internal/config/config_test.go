package config

import (
	"os"
	"path/filepath"
	"testing"
)

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

	want := Config{WorkDirs: []string{"/tmp/code", "/tmp/work"}}
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
