// patch.go
package s99config

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// Patch deep-merges into the file last loaded by Load and validates before
// writing it. Existing raw reference expressions are preserved.
func (l *Loader) Patch(patch any) error {
	patchMap, err := objectFromValue(patch)
	if err != nil {
		return fmt.Errorf("encode config patch: %w", err)
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.configPath == "" || l.raw == nil {
		return fmt.Errorf("patch config: %w; load from a file first", ErrNotLoaded)
	}

	raw := revealSecretMap(l.raw)
	deepMerge(raw, patchMap)
	state, err := l.prepareLocked(l.configPath, raw)
	if err != nil {
		return err
	}
	if err := writeConfigFile(l.configPath, raw, l.parser); err != nil {
		return fmt.Errorf("write patched config: %w", err)
	}
	l.applyLocked(l.configPath, l.parser, state)
	return nil
}

// objectFromValue converts a patch input into an object map.
func objectFromValue(value any) (map[string]any, error) {
	if path, ok := firstSecretValuePath(reflect.ValueOf(value), "$", make(map[visit]struct{})); ok {
		return nil, fmt.Errorf("patch contains s99config.Secret at %s; call Reveal explicitly if plaintext should be written", path)
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return decodeObject(data)
}

// secretInterfaceType is the reflected Secret interface.
var secretInterfaceType = reflect.TypeOf((*Secret)(nil)).Elem()

// visit identifies a reflected value already scanned.
type visit struct {
	typ reflect.Type
	ptr uintptr
}

// firstSecretValuePath finds the first secret handle in a value.
func firstSecretValuePath(value reflect.Value, path string, seen map[visit]struct{}) (string, bool) {
	if !value.IsValid() {
		return "", false
	}
	if value.Kind() == reflect.Interface {
		if value.IsNil() {
			return "", false
		}
		return firstSecretValuePath(value.Elem(), path, seen)
	}
	if isNil(value) {
		return "", false
	}
	if value.Type().Implements(secretInterfaceType) {
		return path, true
	}

	switch value.Kind() {
	case reflect.Pointer:
		if visited(value, seen) {
			return "", false
		}
		return firstSecretValuePath(value.Elem(), path, seen)
	case reflect.Map:
		if visited(value, seen) {
			return "", false
		}
		for _, key := range value.MapKeys() {
			childPath := fmt.Sprintf("%s[%v]", path, key.Interface())
			if found, ok := firstSecretValuePath(value.MapIndex(key), childPath, seen); ok {
				return found, true
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < value.Len(); i++ {
			childPath := fmt.Sprintf("%s[%d]", path, i)
			if found, ok := firstSecretValuePath(value.Index(i), childPath, seen); ok {
				return found, true
			}
		}
	case reflect.Struct:
		typ := value.Type()
		for i := 0; i < value.NumField(); i++ {
			field := typ.Field(i)
			if field.PkgPath != "" {
				continue
			}
			childPath := path + "." + field.Name
			if found, ok := firstSecretValuePath(value.Field(i), childPath, seen); ok {
				return found, true
			}
		}
	}
	return "", false
}

// visited reports whether a pointer-like value was already scanned.
func visited(value reflect.Value, seen map[visit]struct{}) bool {
	entry := visit{typ: value.Type(), ptr: value.Pointer()}
	if _, ok := seen[entry]; ok {
		return true
	}
	seen[entry] = struct{}{}
	return false
}

// isNil reports whether a reflected value is nil-capable and nil.
func isNil(value reflect.Value) bool {
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

// deepMerge recursively merges source into destination.
func deepMerge(destination, source map[string]any) {
	for key, value := range source {
		dst, dstOK := destination[key].(map[string]any)
		src, srcOK := value.(map[string]any)
		if dstOK && srcOK {
			deepMerge(dst, src)
		} else {
			destination[key] = value
		}
	}
}
