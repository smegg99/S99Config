// format.go
package s99config

import (
	stdjson "encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/v2"
)

// parserForPath selects a parser from a file extension.
func parserForPath(path string) (koanf.Parser, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		return jsonParser{}, nil
	case ".yaml", ".yml":
		return yaml.Parser(), nil
	case ".toml":
		return toml.Parser(), nil
	default:
		return nil, fmt.Errorf("unsupported config format %q; use LoadWithParser for a custom Koanf parser", filepath.Ext(path))
	}
}

// jsonParser adapts encoding/json to Koanf.
type jsonParser struct{}

// Unmarshal decodes a JSON object.
func (jsonParser) Unmarshal(data []byte) (map[string]any, error) {
	return decodeObject(data)
}

// Marshal encodes a JSON object.
func (jsonParser) Marshal(config map[string]any) ([]byte, error) {
	return stdjson.Marshal(config)
}
