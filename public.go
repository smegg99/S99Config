// public.go
package s99config

import "encoding/json"

// PublicJSON returns configuration without sensitive fields.
func (l *Loader) PublicJSON() ([]byte, error) {
	l.mu.RLock()
	if l.public == nil {
		l.mu.RUnlock()
		return nil, ErrNotLoaded
	}
	public := cloneMap(l.public)
	l.mu.RUnlock()
	return json.Marshal(public)
}

// ExportPublicJSON writes indented configuration without sensitive values.
func (l *Loader) ExportPublicJSON(path string) error {
	l.mu.RLock()
	if l.public == nil {
		l.mu.RUnlock()
		return ErrNotLoaded
	}
	public := cloneMap(l.public)
	l.mu.RUnlock()
	return writeJSONFile(path, public)
}

// buildPublicMap copies only public configuration values.
func buildPublicMap(input map[string]any, prefix string, private *privateTracker) map[string]any {
	output := make(map[string]any, len(input))
	for key, value := range input {
		path := joinPath(prefix, key)
		if private.hiddenPath(path) {
			continue
		}
		if publicValue, ok := buildPublicValue(value, path, private); ok {
			output[key] = publicValue
		}
	}
	return output
}

// buildPublicValue returns one public value when safe.
func buildPublicValue(value any, path string, private *privateTracker) (any, bool) {
	switch value := value.(type) {
	case map[string]any:
		return buildPublicMap(value, path, private), true
	case []any:
		output := make([]any, 0, len(value))
		for i, item := range value {
			publicItem, ok := buildPublicValue(item, indexPath(path, i), private)
			if ok {
				output = append(output, publicItem)
			}
		}
		return output, true
	case string:
		return value, true
	default:
		return value, true
	}
}
