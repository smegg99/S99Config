// dirs.go
package s99config

import (
	"os"
	"path/filepath"
	"runtime"
)

// Dirs are an app's per-user config, data and state directories.
type Dirs struct {
	Config string
	Data   string
	State  string
}

// UserDirs returns the directories for app: XDG on Linux and BSD, Local AppData on Windows, Application Support on macOS.
func UserDirs(app string) (Dirs, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return Dirs{}, err
	}
	config := filepath.Join(base, app)
	switch runtime.GOOS {
	case "windows":
		data := config
		if local := os.Getenv("LOCALAPPDATA"); filepath.IsAbs(local) {
			data = filepath.Join(local, app)
		}
		return Dirs{Config: config, Data: data, State: data}, nil
	case "darwin", "ios":
		return Dirs{Config: config, Data: config, State: config}, nil
	}
	data, err := xdgDir("XDG_DATA_HOME", ".local", "share")
	if err != nil {
		return Dirs{}, err
	}
	state, err := xdgDir("XDG_STATE_HOME", ".local", "state")
	if err != nil {
		return Dirs{}, err
	}
	return Dirs{Config: config, Data: filepath.Join(data, app), State: filepath.Join(state, app)}, nil
}

// xdgDir reads an XDG variable, falling back to a path under home when it is unset or relative.
func xdgDir(variable string, fallback ...string) (string, error) {
	if dir := os.Getenv(variable); filepath.IsAbs(dir) {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(append([]string{home}, fallback...)...), nil
}
