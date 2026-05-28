// references.go
package s99config

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ReferenceOptions controls opt-in @{source:key} resolution. Values returned
// by a source are treated as literal strings, not additional references.
type ReferenceOptions struct {
	ConfigDir      string
	DataDir        string
	KeyringService string
}

// reference is one parsed @{source:key} value.
type reference struct {
	source   string
	key      string
	optional bool
}

// resolver resolves references for a config load.
type resolver struct {
	options ReferenceOptions
	private *privateTracker
}

// privateTracker records private paths and values.
type privateTracker struct {
	values map[string]string
	hidden map[string]struct{}
}

// newPrivateTracker creates empty private tracking state.
func newPrivateTracker() *privateTracker {
	return &privateTracker{
		values: make(map[string]string),
		hidden: make(map[string]struct{}),
	}
}

// newResolver creates a resolver for one config source.
func (l *Loader) newResolver(configName string, private *privateTracker) resolver {
	options := *l.references
	if options.ConfigDir == "" && configName != "" {
		if absolute, err := filepath.Abs(configName); err == nil {
			options.ConfigDir = filepath.Dir(absolute)
		}
	}
	if options.DataDir == "" {
		options.DataDir = options.ConfigDir
	}
	return resolver{options: options, private: private}
}

// parseReference parses a full reference string.
func parseReference(value string) (reference, bool) {
	if !strings.HasPrefix(value, "@{") || !strings.HasSuffix(value, "}") {
		return reference{}, false
	}
	source, key, ok := strings.Cut(value[2:len(value)-1], ":")
	if !ok || key == "" {
		return reference{}, false
	}
	optional := strings.HasSuffix(source, "?")
	source = strings.TrimSuffix(source, "?")
	if source == "" || !isReferenceSourceName(source) {
		return reference{}, false
	}
	return reference{source: source, optional: optional, key: key}, true
}

// isPrivate reports whether a reference source is sensitive.
func (r reference) isPrivate() bool {
	return r.source == srcEnv || r.source == srcKeyring
}

// isReferenceSourceName validates a source identifier.
func isReferenceSourceName(source string) bool {
	for _, ch := range source {
		if (ch < 'a' || ch > 'z') && (ch < 'A' || ch > 'Z') &&
			(ch < '0' || ch > '9') && ch != '_' {
			return false
		}
	}
	return true
}

// resolveMap resolves references in a map.
func (r resolver) resolveMap(input map[string]any, prefix string) error {
	for key, value := range input {
		path := joinPath(prefix, key)
		resolved, err := r.resolveValue(value, path)
		if err != nil {
			return fmt.Errorf("field %q: %w", path, err)
		}
		input[key] = resolved
	}
	return nil
}

// resolveValue resolves references in one value.
func (r resolver) resolveValue(value any, path string) (any, error) {
	switch value := value.(type) {
	case string:
		ref, ok := parseReference(value)
		if !ok {
			return value, nil
		}
		resolved, err := r.resolve(ref)
		if err != nil {
			return nil, err
		}
		if ref.isPrivate() {
			r.private.mark(path, resolved)
		}
		return resolved, nil
	case map[string]any:
		return value, r.resolveMap(value, path)
	case []any:
		for i, item := range value {
			itemPath := indexPath(path, i)
			var err error
			value[i], err = r.resolveValue(item, itemPath)
			if err != nil {
				return nil, err
			}
			if r.private.hiddenPath(itemPath) {
				r.private.hide(path)
			}
		}
	}
	return value, nil
}

// mark records a private path and value.
func (p *privateTracker) mark(path, secret string) {
	p.values[path] = secret
	p.hidden[path] = struct{}{}
}

// hide records a private path without its value.
func (p *privateTracker) hide(path string) {
	p.hidden[path] = struct{}{}
}

// hiddenPath reports whether a path is hidden.
func (p *privateTracker) hiddenPath(path string) bool {
	_, ok := p.hidden[path]
	return ok
}

// value returns a private value by path.
func (p *privateTracker) value(path string) (string, bool) {
	value, ok := p.values[path]
	return value, ok
}

// pathsOnly drops private values but keeps paths.
func (p *privateTracker) pathsOnly() *privateTracker {
	output := newPrivateTracker()
	for path := range p.values {
		output.values[path] = ""
	}
	for path := range p.hidden {
		output.hidden[path] = struct{}{}
	}
	return output
}

// hasValueUnder reports whether a private value is below prefix.
func (p *privateTracker) hasValueUnder(prefix string) bool {
	for path := range p.values {
		if prefix == "" || path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}

// contains reports whether value contains a private string.
func (p *privateTracker) contains(value string) bool {
	for _, secret := range p.values {
		if secret != "" && strings.Contains(value, secret) {
			return true
		}
	}
	return false
}

// redact replaces known private values in text.
func (p *privateTracker) redact(value string) string {
	secrets := make([]string, 0, len(p.values))
	for _, secret := range p.values {
		secrets = append(secrets, redactionForms(secret)...)
	}
	secrets = uniqueStrings(secrets)
	sort.Slice(secrets, func(i, j int) bool {
		return len(secrets[i]) > len(secrets[j])
	})
	for _, secret := range secrets {
		value = strings.ReplaceAll(value, secret, redactedSecret)
	}
	return value
}

// redactionForms returns raw and escaped secret forms.
func redactionForms(secret string) []string {
	if secret == "" {
		return nil
	}
	forms := []string{secret}
	for _, quoted := range []string{strconv.Quote(secret), strconv.QuoteToASCII(secret)} {
		forms = append(forms, quoted)
		if unquoted := strings.TrimPrefix(strings.TrimSuffix(quoted, `"`), `"`); unquoted != quoted {
			forms = append(forms, unquoted)
		}
	}
	return forms
}

// uniqueStrings removes duplicates.
func uniqueStrings(input []string) []string {
	seen := make(map[string]struct{}, len(input))
	output := input[:0]
	for _, value := range input {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		output = append(output, value)
	}
	return output
}

// joinPath appends an escaped object key.
func joinPath(prefix, key string) string {
	return prefix + "/" + escapePathToken(key)
}

// indexPath appends a list index.
func indexPath(prefix string, index int) string {
	return prefix + "/" + strconv.Itoa(index)
}

// escapePathToken escapes one path token.
func escapePathToken(value string) string {
	value = strings.ReplaceAll(value, "~", "~0")
	return strings.ReplaceAll(value, "/", "~1")
}

// unescapePathToken unescapes one path token.
func unescapePathToken(value string) (string, bool) {
	var result strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] != '~' {
			result.WriteByte(value[i])
			continue
		}
		if i+1 >= len(value) {
			return "", false
		}
		i++
		switch value[i] {
		case '0':
			result.WriteByte('~')
		case '1':
			result.WriteByte('/')
		default:
			return "", false
		}
	}
	return result.String(), true
}

// objectPathKeys converts an encoded path to object keys.
func objectPathKeys(path string) ([]string, bool) {
	if path == "" {
		return nil, true
	}
	if path[0] != '/' {
		return nil, false
	}
	tokens := strings.Split(path[1:], "/")
	keys := make([]string, len(tokens))
	for i, token := range tokens {
		var ok bool
		keys[i], ok = unescapePathToken(token)
		if !ok {
			return nil, false
		}
	}
	return keys, true
}
