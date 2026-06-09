// load.go
package s99config

import (
	"fmt"
	"path/filepath"

	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// Load reads configuration based on its .json, .yaml, .yml, or .toml
// extension. CUE defaults are included in the loaded value.
func (l *Loader) Load(path string) error {
	parser, err := parserForPath(path)
	if err != nil {
		return err
	}
	return l.LoadWithParser(path, parser)
}

// LoadWithParser reads configuration using a custom Koanf parser.
func (l *Loader) LoadWithParser(path string, parser koanf.Parser) error {
	if parser == nil {
		return fmt.Errorf("load config %s: parser cannot be nil", path)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve config path %s: %w", path, err)
	}
	config := koanf.New(".")
	var raw map[string]any
	if err := config.Load(file.Provider(path), parser, koanf.WithMergeFunc(func(src, _ map[string]any) error {
		raw = src
		return nil
	})); err != nil {
		return fmt.Errorf("load config %s: %w", path, err)
	}

	state, err := l.prepareLocked(absolutePath, raw)
	if err != nil {
		return err
	}
	l.applyLocked(absolutePath, parser, state)
	return nil
}

// LoadJSON validates JSON from memory. name is reported in errors and
// provides a location for relative references.
func (l *Loader) LoadJSON(name string, data []byte) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	raw, err := decodeObject(data)
	if err != nil {
		return fmt.Errorf("parse config %s: %w", name, err)
	}
	state, err := l.prepareLocked(name, raw)
	if err != nil {
		return err
	}
	l.applyLocked("", nil, state)
	return nil
}

// prepareLocked resolves and validates raw config.
func (l *Loader) prepareLocked(name string, raw map[string]any) (loadedState, error) {
	input := cloneMap(raw)
	private := newPrivateTracker()
	if l.references != nil {
		if err := l.newResolver(name, private).resolveMap(input, ""); err != nil {
			return loadedState{}, fmt.Errorf("resolve config references: %w", err)
		}
	}
	if err := l.markDeclaredSecrets(input, private); err != nil {
		return loadedState{}, err
	}
	full, err := l.validateLocked(name, input, private)
	if err != nil {
		return loadedState{}, err
	}

	if l.references != nil {
		resolvedDefaults := make(map[string]struct{})
		for {
			changed, err := l.resolveDefaultsLocked(name, raw, input, full, private, resolvedDefaults)
			if err != nil {
				return loadedState{}, fmt.Errorf("resolve default references: %w", err)
			}
			if !changed {
				break
			}
			full, err = l.validateLocked(name, input, private)
			if err != nil {
				return loadedState{}, fmt.Errorf("validate resolved references: %w", err)
			}
		}
	}

	if err := l.markDeclaredSecrets(full, private); err != nil {
		return loadedState{}, err
	}
	public := buildPublicMap(full, "", private)
	secrets := &secretStore{values: cloneStrings(private.values)}
	return loadedState{
		raw:      concealRawSecretMap(raw, private, secrets, l.secretFactory),
		resolved: handleSecretMap(full, private, secrets, l.secretFactory),
		public:   public,
		private:  private.pathsOnly(),
		secrets:  secrets,
	}, nil
}

// applyLocked stores prepared config state.
func (l *Loader) applyLocked(path string, parser koanf.Parser, state loadedState) {
	l.configPath = path
	l.parser = parser
	l.raw = state.raw
	l.resolved = state.resolved
	l.public = state.public
	l.private = state.private
	l.secrets = state.secrets
}

// valuePathStep stores one path segment.
type valuePathStep struct {
	key   string
	index int
	list  bool
}

// resolveDefaultsLocked resolves reference defaults.
func (l *Loader) resolveDefaultsLocked(name string, raw, input, full map[string]any, private *privateTracker, done map[string]struct{}) (bool, error) {
	resolver := l.newResolver(name, private)
	return resolveDefaultValues(full, raw, true, "", nil, input, resolver, l.referenceDefaultPaths, done)
}

// resolveDefaultValues walks defaults and fills resolved references.
func resolveDefaultValues(value, raw any, rawExists bool, path string, steps []valuePathStep, input map[string]any, resolver resolver, referenceDefaults, done map[string]struct{}) (bool, error) {
	switch value := value.(type) {
	case string:
		if rawExists {
			return false, nil
		}
		if _, ok := referenceDefaults[path]; !ok {
			return false, nil
		}
		if _, ok := done[path]; ok {
			return false, nil
		}
		ref, ok := parseReference(value)
		if !ok {
			return false, nil
		}
		resolved, err := resolver.resolve(ref)
		if err != nil {
			return false, fmt.Errorf("field %q: %w", path, err)
		}
		if ref.isPrivate() {
			resolver.private.mark(path, resolved)
		}
		setValueAtPath(input, steps, resolved)
		done[path] = struct{}{}
		return true, nil
	case map[string]any:
		rawMap, rawMapOK := raw.(map[string]any)
		changed := false
		for key, item := range value {
			itemRaw, itemExists := rawMap[key]
			if !rawExists || !rawMapOK {
				itemExists = false
			}
			itemChanged, err := resolveDefaultValues(
				item,
				itemRaw,
				itemExists,
				joinPath(path, key),
				append(steps, valuePathStep{key: key}),
				input,
				resolver,
				referenceDefaults,
				done,
			)
			if err != nil {
				return false, err
			}
			changed = changed || itemChanged
		}
		return changed, nil
	case []any:
		rawList, rawListOK := raw.([]any)
		changed := false
		for i, item := range value {
			itemExists := rawExists && rawListOK && i < len(rawList)
			var itemRaw any
			if itemExists {
				itemRaw = rawList[i]
			}
			itemChanged, err := resolveDefaultValues(
				item,
				itemRaw,
				itemExists,
				indexPath(path, i),
				append(steps, valuePathStep{index: i, list: true}),
				input,
				resolver,
				referenceDefaults,
				done,
			)
			if err != nil {
				return false, err
			}
			changed = changed || itemChanged
		}
		return changed, nil
	default:
		return false, nil
	}
}

// setValueAtPath writes a value into a nested object.
func setValueAtPath(root map[string]any, steps []valuePathStep, value any) {
	if len(steps) == 0 {
		return
	}
	setChildValue(root, steps, value)
}

// setChildValue writes one recursive path segment.
func setChildValue(current any, steps []valuePathStep, value any) any {
	if len(steps) == 0 {
		return value
	}
	step := steps[0]
	if step.list {
		list, _ := current.([]any)
		for len(list) <= step.index {
			list = append(list, nil)
		}
		list[step.index] = setChildValue(list[step.index], steps[1:], value)
		return list
	}
	object, _ := current.(map[string]any)
	if object == nil {
		object = make(map[string]any)
	}
	object[step.key] = setChildValue(object[step.key], steps[1:], value)
	return object
}

// cloneStrings copies a string map.
func cloneStrings(input map[string]string) map[string]string {
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}
