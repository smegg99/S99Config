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

// WithDefinition selects the CUE definition to validate against, #Config by default.
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

// WithSecretFactory sets the Secret built for each sensitive value; a nil result falls back to the redacted handle.
func WithSecretFactory(factory SecretFactory) Option {
	return func(o *options) error {
		if factory == nil {
			return errSecretFactoryNil
		}
		o.secretFactory = factory
		return nil
	}
}
