// sensitive.go
package s99config

import (
	"fmt"

	"cuelang.org/go/cue"
)

// collectSecretPaths finds fields marked @secret.
func collectSecretPaths(def cue.Value) (map[string]struct{}, error) {
	paths := make(map[string]struct{})
	if err := walkSecretFields(def, "", paths); err != nil {
		return nil, err
	}
	return paths, nil
}

// collectReferenceDefaultPaths finds default reference fields.
func collectReferenceDefaultPaths(def cue.Value) (map[string]struct{}, error) {
	paths := make(map[string]struct{})
	if err := walkReferenceDefaultFields(def, "", paths); err != nil {
		return nil, err
	}
	return paths, nil
}

// walkReferenceDefaultFields records reference defaults recursively.
func walkReferenceDefaultFields(value cue.Value, prefix string, paths map[string]struct{}) error {
	fields, err := value.Fields(cue.Optional(true))
	if err != nil {
		return nil
	}
	for fields.Next() {
		path := joinPath(prefix, fields.Selector().Unquoted())
		field := fields.Value()
		if defaultValue, ok := field.Default(); ok {
			if text, err := defaultValue.String(); err == nil {
				if _, isReference := parseReference(text); isReference {
					paths[path] = struct{}{}
				}
			}
		}
		if err := walkReferenceDefaultFields(field, path, paths); err != nil {
			return err
		}
	}
	return nil
}

// walkSecretFields records @secret fields recursively.
func walkSecretFields(value cue.Value, prefix string, paths map[string]struct{}) error {
	fields, err := value.Fields(cue.Optional(true))
	if err != nil {
		return nil
	}
	for fields.Next() {
		path := joinPath(prefix, fields.Selector().Unquoted())
		field := fields.Value()
		attribute := field.Attribute("secret")
		if err := attribute.Err(); err == nil {
			paths[path] = struct{}{}
		}
		if err := walkSecretFields(field, path, paths); err != nil {
			return err
		}
	}
	return nil
}

// markDeclaredSecrets marks @secret values as private.
func (l *Loader) markDeclaredSecrets(config map[string]any, private *privateTracker) error {
	for path := range l.secretPaths {
		value, ok := lookupValue(config, path)
		if !ok {
			continue
		}
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("sensitive config field %q must resolve to a string", path)
		}
		private.mark(path, text)
	}
	return nil
}

// lookupValue reads a value by encoded object path.
func lookupValue(input map[string]any, path string) (any, bool) {
	keys, ok := objectPathKeys(path)
	if !ok {
		return nil, false
	}
	var current any = input
	for _, key := range keys {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[key]
		if !ok {
			return nil, false
		}
	}
	return current, true
}
