// validate.go
package s99config

import (
	"encoding/json"
	"fmt"
	"strings"

	"cuelang.org/go/cue"
	cueerrors "cuelang.org/go/cue/errors"
	cuejson "cuelang.org/go/encoding/json"
)

// validateLocked validates config against CUE.
func (l *Loader) validateLocked(name string, config map[string]any, private *privateTracker) (map[string]any, error) {
	data, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("marshal config for validation: %w", err)
	}
	expr, err := cuejson.Extract(name, data)
	if err != nil {
		return nil, fmt.Errorf("parse config as CUE value: %w", err)
	}

	unified := l.def.Unify(l.ctx.BuildExpr(expr))
	if err := unified.Validate(cue.Concrete(true)); err != nil {
		details := strings.TrimSpace(cueerrors.Details(err, nil))
		details = private.redact(details)
		return nil, fmt.Errorf("invalid config %s:\n%s", name, details)
	}

	data, err = unified.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("export validated config: %w", err)
	}
	return decodeObject(data)
}
