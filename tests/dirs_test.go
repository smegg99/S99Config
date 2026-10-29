// tests/dirs_test.go

package tests

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/smegg99/s99config"
)

// UserDirs follows the XDG variables and ignores a relative one.
func TestUserDirsFollowXDG(t *testing.T) {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		t.Skip("XDG layout only")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "/xdg/config")
	t.Setenv("XDG_DATA_HOME", "relative/data")
	t.Setenv("XDG_STATE_HOME", "/xdg/state")

	dirs, err := s99config.UserDirs("demo")
	if err != nil {
		t.Fatal(err)
	}
	want := s99config.Dirs{
		Config: "/xdg/config/demo",
		Data:   filepath.Join(home, ".local", "share", "demo"),
		State:  "/xdg/state/demo",
	}
	if dirs != want {
		t.Fatalf("UserDirs = %+v, want %+v", dirs, want)
	}
}

// LoadOrCreate writes the defaults on first run and keeps the file after that.
func TestLoadOrCreate(t *testing.T) {
	loader, err := s99config.New([]byte(`#Config: {name: string | *"first"}`))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "app", "config.json")
	if err := loader.LoadOrCreate(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"name":"kept"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := loader.LoadOrCreate(path); err != nil {
		t.Fatal(err)
	}
	resolved, err := loader.Map()
	if err != nil {
		t.Fatal(err)
	}
	if resolved["name"] != "kept" {
		t.Fatalf("name = %#v, want %q", resolved["name"], "kept")
	}
	content, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(content), "kept") {
		t.Fatalf("file was rewritten: %s %v", content, err)
	}
}
