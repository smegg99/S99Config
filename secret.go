// secret.go
package s99config

import (
	"encoding"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"reflect"
)

const redactedSecret = "[redacted]"
const redactedLoader = "s99config.Loader{configuration state redacted}"

// secretStore holds plaintext values by path.
type secretStore struct {
	values map[string]string
}

// Secret is a sensitive value that redacts itself in output; Reveal is the explicit plaintext access.
type Secret interface {
	Reveal() string
	IsSet() bool
	fmt.Stringer
	fmt.GoStringer
	fmt.Formatter
	json.Marshaler
	encoding.TextMarshaler
	slog.LogValuer
}

// SecretValue is the default Secret: a path-bound handle that prints as [redacted].
type SecretValue struct {
	store *secretStore
	path  string
}

// Reveal returns the plaintext sensitive value referenced by this handle.
func (s SecretValue) Reveal() string {
	if s.store == nil {
		return ""
	}
	return s.store.values[s.path]
}

// IsSet reports whether this handle references a loaded sensitive value.
func (s SecretValue) IsSet() bool {
	if s.store == nil {
		return false
	}
	_, ok := s.store.values[s.path]
	return ok
}

// Path returns the value's encoded config path, for a SecretFactory to choose per field.
func (s SecretValue) Path() string {
	return s.path
}

// String prevents plaintext exposure through ordinary string formatting.
func (s SecretValue) String() string {
	return redactedSecret
}

// GoString prevents plaintext exposure through Go-syntax formatting.
func (s SecretValue) GoString() string {
	return redactedSecret
}

// Format redacts SecretValue values for every fmt formatting verb.
func (s SecretValue) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, redactedSecret)
}

// MarshalJSON prevents plaintext exposure through JSON logging or exporting.
func (s SecretValue) MarshalJSON() ([]byte, error) {
	return json.Marshal(redactedSecret)
}

// MarshalText prevents plaintext exposure through text-marshaling loggers.
func (s SecretValue) MarshalText() ([]byte, error) {
	return []byte(redactedSecret), nil
}

// LogValue prevents plaintext exposure through log/slog.
func (s SecretValue) LogValue() slog.Value {
	return slog.StringValue(redactedSecret)
}

// SecretPresenter renders a sensitive value for output; calling Reveal in it exposes the value.
type SecretPresenter func(SecretValue) string

// PresentedSecret is a Secret with custom output for fmt, JSON, text and slog; embed it in app types.
type PresentedSecret struct {
	SecretValue
	presenter SecretPresenter
}

// NewPresentedSecret wraps value with presenter; a nil presenter keeps [redacted].
func NewPresentedSecret(value SecretValue, presenter SecretPresenter) PresentedSecret {
	return PresentedSecret{SecretValue: value, presenter: presenter}
}

// presentation returns the configured display text.
func (s PresentedSecret) presentation() string {
	if s.presenter == nil {
		return redactedSecret
	}
	return s.presenter(s.SecretValue)
}

// String presents the configured representation.
func (s PresentedSecret) String() string {
	return s.presentation()
}

// GoString presents the configured representation.
func (s PresentedSecret) GoString() string {
	return s.presentation()
}

// Format presents the configured representation for every fmt verb.
func (s PresentedSecret) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, s.presentation())
}

// MarshalJSON marshals the configured representation.
func (s PresentedSecret) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.presentation())
}

// MarshalText returns the configured representation.
func (s PresentedSecret) MarshalText() ([]byte, error) {
	return []byte(s.presentation()), nil
}

// LogValue presents the configured representation through log/slog.
func (s PresentedSecret) LogValue() slog.Value {
	return slog.StringValue(s.presentation())
}

// SecretFactory turns a sensitive value into the Secret that Decode and Map hand out.
type SecretFactory func(SecretValue) Secret

// defaultSecretFactory returns the default redacting handle.
func defaultSecretFactory(value SecretValue) Secret {
	return value
}

// buildSecret creates a secret handle for a path.
func buildSecret(secrets *secretStore, path string, factory SecretFactory) Secret {
	value := SecretValue{store: secrets, path: path}
	if factory == nil {
		return value
	}
	secret := factory(value)
	if secret == nil {
		return value
	}
	reflected := reflect.ValueOf(secret)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if reflected.IsNil() {
			return value
		}
	}
	return secret
}

// String prevents formatting a loader from exposing its loaded secret store.
func (l *Loader) String() string {
	return redactedLoader
}

// GoString prevents Go-syntax formatting from exposing loaded secret state.
func (l *Loader) GoString() string {
	return redactedLoader
}

// Format redacts Loader values for every fmt formatting verb.
func (l *Loader) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, redactedLoader)
}

// LogValue prevents plaintext exposure when a Loader is passed to log/slog.
func (l *Loader) LogValue() slog.Value {
	return slog.StringValue(redactedLoader)
}
