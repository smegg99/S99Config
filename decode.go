package s99config

import (
	"fmt"
	"reflect"
	"strings"
)

// handleSecretMap replaces private values with secret handles.
func handleSecretMap(input map[string]any, private *privateTracker, secrets *secretStore, factory SecretFactory) map[string]any {
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = handleSecretValue(value, joinPath("", key), private, secrets, factory)
	}
	return output
}

// concealRawSecretMap hides raw secrets that were literal values.
func concealRawSecretMap(input map[string]any, private *privateTracker, secrets *secretStore, factory SecretFactory) map[string]any {
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = concealRawSecretValue(value, joinPath("", key), private, secrets, factory)
	}
	return output
}

// concealRawSecretValue hides one raw secret value.
func concealRawSecretValue(value any, path string, private *privateTracker, secrets *secretStore, factory SecretFactory) any {
	if secret, ok := private.value(path); ok {
		if text, ok := value.(string); ok && text == secret {
			return buildSecret(secrets, path, factory)
		}
	}
	switch value := value.(type) {
	case map[string]any:
		output := make(map[string]any, len(value))
		for key, item := range value {
			output[key] = concealRawSecretValue(item, joinPath(path, key), private, secrets, factory)
		}
		return output
	case []any:
		output := make([]any, len(value))
		for i, item := range value {
			output[i] = concealRawSecretValue(item, indexPath(path, i), private, secrets, factory)
		}
		return output
	default:
		return value
	}
}

// revealSecretMap converts secret handles to plaintext.
func revealSecretMap(input map[string]any) map[string]any {
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = revealSecretValue(value)
	}
	return output
}

// revealSecretValue converts one secret handle to plaintext.
func revealSecretValue(value any) any {
	switch value := value.(type) {
	case Secret:
		return value.Reveal()
	case map[string]any:
		return revealSecretMap(value)
	case []any:
		output := make([]any, len(value))
		for i, item := range value {
			output[i] = revealSecretValue(item)
		}
		return output
	default:
		return value
	}
}

// handleSecretValue replaces one private value with a secret handle.
func handleSecretValue(value any, path string, private *privateTracker, secrets *secretStore, factory SecretFactory) any {
	if _, ok := private.value(path); ok {
		return buildSecret(secrets, path, factory)
	}
	switch value := value.(type) {
	case map[string]any:
		output := make(map[string]any, len(value))
		for key, item := range value {
			itemPath := joinPath(path, key)
			output[key] = handleSecretValue(item, itemPath, private, secrets, factory)
		}
		return output
	case []any:
		output := make([]any, len(value))
		for i, item := range value {
			itemPath := indexPath(path, i)
			output[i] = handleSecretValue(item, itemPath, private, secrets, factory)
		}
		return output
	default:
		return value
	}
}

// maskSecretMap removes secrets before JSON decoding.
func maskSecretMap(input map[string]any, private *privateTracker) map[string]any {
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = maskSecretValue(value, joinPath("", key), private)
	}
	return output
}

// maskSecretValue masks one secret for JSON decoding.
func maskSecretValue(value any, path string, private *privateTracker) any {
	if _, ok := private.value(path); ok {
		return nil
	}
	switch value := value.(type) {
	case map[string]any:
		output := make(map[string]any, len(value))
		for key, item := range value {
			output[key] = maskSecretValue(item, joinPath(path, key), private)
		}
		return output
	case []any:
		output := make([]any, len(value))
		for i, item := range value {
			output[i] = maskSecretValue(item, indexPath(path, i), private)
		}
		return output
	default:
		return value
	}
}

// injectSecrets inserts secret handles after JSON decoding.
func injectSecrets(dst any, source map[string]any, private *privateTracker, secrets *secretStore, factory SecretFactory) error {
	value := reflect.ValueOf(dst)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return nil
	}
	return injectSecretValue(value.Elem(), source, "", private, secrets, factory)
}

// injectSecretValue inserts secret handles recursively.
func injectSecretValue(target reflect.Value, source any, path string, private *privateTracker, secrets *secretStore, factory SecretFactory) error {
	if _, ok := private.value(path); ok {
		return setSecretHandle(target, path, secrets, factory)
	}
	target = indirectTarget(target)
	if !target.IsValid() {
		return nil
	}
	switch source := source.(type) {
	case map[string]any:
		switch target.Kind() {
		case reflect.Interface:
			if target.IsNil() {
				return nil
			}
			return injectSecretValue(target.Elem(), source, path, private, secrets, factory)
		case reflect.Map:
			if target.Type().Key().Kind() != reflect.String {
				if private.hasValueUnder(path) {
					return errSensitiveMapStringKeysRequired
				}
				return nil
			}
			for key, item := range source {
				mapKey := reflect.ValueOf(key).Convert(target.Type().Key())
				current := target.MapIndex(mapKey)
				if !current.IsValid() {
					continue
				}
				replacement := reflect.New(target.Type().Elem()).Elem()
				replacement.Set(current)
				if err := injectSecretValue(replacement, item, joinPath(path, key), private, secrets, factory); err != nil {
					return err
				}
				target.SetMapIndex(mapKey, replacement)
			}
			return nil
		}
		if target.Kind() != reflect.Struct {
			return nil
		}
		for key, item := range source {
			field, ok := jsonField(target, key)
			if !ok {
				continue
			}
			if err := injectSecretValue(field, item, joinPath(path, key), private, secrets, factory); err != nil {
				return err
			}
		}
	case []any:
		if target.Kind() != reflect.Slice && target.Kind() != reflect.Array {
			return nil
		}
		for i, item := range source {
			if i >= target.Len() {
				break
			}
			if err := injectSecretValue(target.Index(i), item, indexPath(path, i), private, secrets, factory); err != nil {
				return err
			}
		}
	}
	return nil
}

// setSecretHandle assigns one secret handle.
func setSecretHandle(target reflect.Value, path string, secrets *secretStore, factory SecretFactory) error {
	secret := buildSecret(secrets, path, factory)
	value := reflect.ValueOf(secret)
	if target.CanSet() && value.Type().AssignableTo(target.Type()) {
		target.Set(value)
		return nil
	}
	if target.Kind() == reflect.Pointer && target.CanSet() && value.Type().AssignableTo(target.Type().Elem()) {
		pointer := reflect.New(target.Type().Elem())
		pointer.Elem().Set(value)
		target.Set(pointer)
		return nil
	}
	return fmt.Errorf("sensitive field %q must accept s99config.Secret implementation %T, not %s", path, secret, target.Type())
}

// indirectTarget follows or allocates pointers.
func indirectTarget(value reflect.Value) reflect.Value {
	for value.IsValid() && value.Kind() == reflect.Pointer {
		if value.IsNil() {
			if !value.CanSet() {
				return reflect.Value{}
			}
			value.Set(reflect.New(value.Type().Elem()))
		}
		value = value.Elem()
	}
	return value
}

// jsonField finds an exported struct field by JSON name.
func jsonField(value reflect.Value, name string) (reflect.Value, bool) {
	typ := value.Type()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue
		}
		jsonName := strings.Split(field.Tag.Get("json"), ",")[0]
		if jsonName == "-" {
			continue
		}
		if jsonName == "" {
			jsonName = field.Name
		}
		if jsonName == name {
			return value.Field(i), true
		}
	}
	return reflect.Value{}, false
}
