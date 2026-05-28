// value.go
package s99config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// ConfigPath returns the absolute path last passed to Load.
func (l *Loader) ConfigPath() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.configPath
}

// Decode unmarshals the resolved, default-filled configuration into dst.
// Loaded sensitive fields must accept the configured Secret implementation.
func (l *Loader) Decode(dst any) error {
	if dst == nil {
		return errDecodeDestinationNil
	}
	l.mu.RLock()
	if l.resolved == nil {
		l.mu.RUnlock()
		return ErrNotLoaded
	}
	resolved := cloneMap(l.resolved)
	private := l.private
	secrets := l.secrets
	secretFactory := l.secretFactory
	l.mu.RUnlock()

	data, err := json.Marshal(maskSecretMap(resolved, private))
	if err != nil {
		return fmt.Errorf("marshal loaded config: %w", err)
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("decode loaded config: %w", err)
	}
	if err := injectSecrets(dst, resolved, private, secrets, secretFactory); err != nil {
		return fmt.Errorf("decode loaded config: %w", err)
	}
	return nil
}

// Map returns a copy of the resolved, default-filled configuration, replacing
// loaded sensitive values with configured Secret implementations.
func (l *Loader) Map() (map[string]any, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.resolved == nil {
		return nil, ErrNotLoaded
	}
	return handleSecretMap(l.resolved, l.private, l.secrets, l.secretFactory), nil
}

// RawMap returns a copy of the original configuration object before defaults
// and reference resolution, replacing loaded sensitive values with configured
// Secret implementations.
func (l *Loader) RawMap() (map[string]any, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.raw == nil {
		return nil, ErrNotLoaded
	}
	return handleSecretMap(l.raw, l.private, l.secrets, l.secretFactory), nil
}

// decodeObject decodes one JSON object.
func decodeObject(data []byte) (map[string]any, error) {
	var result map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&result); err != nil {
		return nil, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, errConfigurationMultipleJSONValues
	}
	if result == nil {
		return nil, errConfigurationNotObject
	}
	return result, nil
}

// cloneMap copies a map recursively.
func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	result := make(map[string]any, len(input))
	for key, value := range input {
		result[key] = cloneValue(value)
	}
	return result
}

// cloneValue copies nested maps and slices.
func cloneValue(value any) any {
	switch value := value.(type) {
	case map[string]any:
		return cloneMap(value)
	case []any:
		result := make([]any, len(value))
		for i, item := range value {
			result[i] = cloneValue(item)
		}
		return result
	default:
		return value
	}
}
