// sensitive.go
package s99config

import (
	"fmt"
	"strings"

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

// anyToken stands for every key of a pattern field or every list index; no escaped key can equal it.
const anyToken = "~*"

// walkSecretFields records @secret fields recursively, through pattern fields and list elements.
func walkSecretFields(value cue.Value, prefix string, paths map[string]struct{}) error {
	if element := value.LookupPath(cue.MakePath(cue.AnyIndex)); element.Exists() {
		if err := walkSecretFields(element, prefix+"/"+anyToken, paths); err != nil {
			return err
		}
	}
	if pattern := value.LookupPath(cue.MakePath(cue.AnyString)); pattern.Exists() {
		if err := walkSecretField(pattern, prefix+"/"+anyToken, paths); err != nil {
			return err
		}
	}
	fields, err := value.Fields(cue.Optional(true))
	if err != nil {
		return nil
	}
	for fields.Next() {
		if err := walkSecretField(fields.Value(), joinPath(prefix, fields.Selector().Unquoted()), paths); err != nil {
			return err
		}
	}
	return nil
}

// walkSecretField records one field when it is marked @secret, then walks into it.
func walkSecretField(field cue.Value, path string, paths map[string]struct{}) error {
	attribute := field.Attribute("secret")
	if err := attribute.Err(); err == nil {
		paths[path] = struct{}{}
	}
	return walkSecretFields(field, path, paths)
}

// markDeclaredSecrets marks @secret values as private.
func (l *Loader) markDeclaredSecrets(config map[string]any, private *privateTracker) error {
	for path := range l.secretPaths {
		tokens := strings.Split(path, "/")[1:]
		if err := markSecretMatches(config, "", tokens, private); err != nil {
			return err
		}
	}
	return nil
}

// markSecretMatches marks every value that tokens reach from value; a list of strings marks each item.
func markSecretMatches(value any, path string, tokens []string, private *privateTracker) error {
	if len(tokens) == 0 {
		return markSecretValue(value, path, private)
	}
	token, rest := tokens[0], tokens[1:]
	switch value := value.(type) {
	case map[string]any:
		if token != anyToken {
			key, ok := unescapePathToken(token)
			if !ok {
				return nil
			}
			item, ok := value[key]
			if !ok {
				return nil
			}
			return markSecretMatches(item, joinPath(path, key), rest, private)
		}
		for key, item := range value {
			if err := markSecretMatches(item, joinPath(path, key), rest, private); err != nil {
				return err
			}
		}
	case []any:
		if token != anyToken {
			return nil
		}
		for i, item := range value {
			if err := markSecretMatches(item, indexPath(path, i), rest, private); err != nil {
				return err
			}
		}
	}
	return nil
}

// markSecretValue marks a string, or each string of a list, as private.
func markSecretValue(value any, path string, private *privateTracker) error {
	switch value := value.(type) {
	case string:
		private.mark(path, value)
		return nil
	case []any:
		for i, item := range value {
			text, ok := item.(string)
			if !ok {
				return fmt.Errorf("sensitive config field %q must hold strings", path)
			}
			private.mark(indexPath(path, i), text)
		}
		private.hide(path)
		return nil
	default:
		return fmt.Errorf("sensitive config field %q must resolve to a string", path)
	}
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
