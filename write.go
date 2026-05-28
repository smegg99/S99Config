// write.go
package s99config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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

// writeBytesAtomic replaces a file through a temp file.
func writeBytesAtomic(path string, data []byte, defaultMode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
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
	if err := file.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}
