// options.go
package s99config

import "strings"

// Option configures a Loader.
type Option func(*options) error

// options holds loader setup values.
type options struct {
	definition    string
	references    *ReferenceOptions
	secretFactory SecretFactory
}

// WithDefinition selects the CUE definition used for validation.
// The default is "#Config".
func WithDefinition(path string) Option {
	return func(o *options) error {
		if strings.TrimSpace(path) == "" {
			return errDefinitionPathEmpty
		}
		o.definition = path
		return nil
	}
}

// WithReferences enables resolution of @{source:key} strings.
func WithReferences(refs ReferenceOptions) Option {
	return func(o *options) error {
		copy := refs
		o.references = &copy
		return nil
	}
}

// WithSecretFactory selects the Secret implementation injected for sensitive
// loaded values. The factory receives an opaque SecretValue handle. A nil or
// typed-nil Secret returned by the factory falls back to the default redacted
// handle.
func WithSecretFactory(factory SecretFactory) Option {
	return func(o *options) error {
		if factory == nil {
			return errSecretFactoryNil
		}
		o.secretFactory = factory
		return nil
	}
}
