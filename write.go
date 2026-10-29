// write.go

package s99config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/knadh/koanf/v2"
)

// writeJSONFile writes indented JSON atomically.
func writeJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	return writeBytesAtomic(path, append(data, '\n'), 0o644)
}

// writeConfigFile writes config with the selected parser.
func writeConfigFile(path string, value map[string]any, parser koanf.Parser) error {
	if parser == nil {
		return errNoParserAssociated
	}
	encoded := value
	if _, isJSON := parser.(jsonParser); !isJSON {
		var err error
		encoded, err = nativeNumberMap(value)
		if err != nil {
			return fmt.Errorf("prepare config numbers: %w", err)
		}
	}
	data, err := parser.Marshal(encoded)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}
	return writeBytesAtomic(path, data, 0o600)
}

// nativeNumberMap converts JSON numbers for non-JSON formats.
func nativeNumberMap(input map[string]any) (map[string]any, error) {
	result := make(map[string]any, len(input))
	for key, item := range input {
		value, err := nativeNumberValue(item)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", key, err)
		}
		result[key] = value
	}
	return result, nil
}

// nativeNumberValue converts one JSON number value.
func nativeNumberValue(value any) (any, error) {
	switch value := value.(type) {
	case json.Number:
		if !strings.ContainsAny(string(value), ".eE") {
			if integer, err := strconv.ParseInt(string(value), 10, 64); err == nil {
				return integer, nil
			}
			return nil, fmt.Errorf("integer %s cannot be represented by this file format", value)
		}
		if decimal, err := strconv.ParseFloat(string(value), 64); err == nil {
			return decimal, nil
		}
		return nil, fmt.Errorf("number %s cannot be represented by this file format", value)
	case map[string]any:
		return nativeNumberMap(value)
	case []any:
		result := make([]any, len(value))
		for i, item := range value {
			var err error
			result[i], err = nativeNumberValue(item)
			if err != nil {
				return nil, fmt.Errorf("item %d: %w", i, err)
			}
		}
		return result, nil
	default:
		return value, nil
	}
}

// writeBytesAtomic replaces a file through a synced temp file, writing through a symlink to its target.
func writeBytesAtomic(path string, data []byte, defaultMode os.FileMode) error {
	path = linkTarget(path)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}

	mode := defaultMode
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}

	file, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	tempPath := file.Name()
	defer os.Remove(tempPath)

	if err := file.Chmod(mode); err != nil {
		file.Close()
		return fmt.Errorf("set temporary file permissions: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("sync temporary file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return syncDir(dir)
}

// linkTarget follows path's symlinks to the file they name, existing or not, so writing keeps the links.
func linkTarget(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	// EvalSymlinks fails on a dangling link; follow it by hand, bounded like the kernel's 40 hops.
	for range 40 {
		target, err := os.Readlink(path)
		if err != nil {
			return path
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(path), target)
		}
		path = target
	}
	return path
}

// syncDir makes a rename inside dir durable; Windows cannot sync a directory handle.
func syncDir(dir string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	handle, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open directory %s: %w", dir, err)
	}
	defer handle.Close()
	// Some network and FUSE filesystems cannot sync a directory; the rename already happened.
	if err := handle.Sync(); err != nil && !errors.Is(err, syscall.EINVAL) {
		return fmt.Errorf("sync directory %s: %w", dir, err)
	}
	return nil
}
