// defaults.go

package s99config

import (
	"encoding/json"
	"fmt"
	"strings"

	"cuelang.org/go/cue"
	cueerrors "cuelang.org/go/cue/errors"
	"github.com/knadh/koanf/v2"
)

// DefaultsJSON returns indented JSON built from schema defaults, secrets included; treat it as private.
func (l *Loader) DefaultsJSON() ([]byte, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.def.Validate(cue.Concrete(true)); err != nil {
		details := strings.TrimSpace(cueerrors.Details(err, nil))
		return nil, fmt.Errorf("CUE definition %s does not describe a complete default config:\n%s", l.definition, details)
	}
	data, err := l.def.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("export default config: %w", err)
	}

	var formatted []byte
	formatted, err = json.MarshalIndent(json.RawMessage(data), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("format default config: %w", err)
	}
	return append(formatted, '\n'), nil
}

// WriteDefaults writes schema defaults in the format of path's extension, as an owner-only file.
func (l *Loader) WriteDefaults(path string) error {
	parser, err := parserForPath(path)
	if err != nil {
		return err
	}
	return l.writeDefaultsWithParser(path, parser)
}

// writeDefaultsWithParser writes defaults using parser.
func (l *Loader) writeDefaultsWithParser(path string, parser koanf.Parser) error {
	data, err := l.DefaultsJSON()
	if err != nil {
		return err
	}
	defaults, err := decodeObject(data)
	if err != nil {
		return fmt.Errorf("decode defaults: %w", err)
	}
	return writeConfigFile(path, defaults, parser)
}
