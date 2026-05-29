package s99config

import "errors"

// ErrNotLoaded is returned when an operation requires loaded configuration.
var ErrNotLoaded = errors.New("configuration has not been loaded")

// Internal sentinel errors reused across the package.
var (
	errCompileSchemaNilFilesystem      = errors.New("s99config: compile schema: schema filesystem is nil")
	errConfigurationMultipleJSONValues = errors.New("s99config: configuration contains multiple JSON values")
	errConfigurationNotObject          = errors.New("s99config: configuration must be a JSON object")
	errDecodeDestinationNil            = errors.New("s99config: decode destination cannot be nil")
	errDefinitionPathEmpty             = errors.New("s99config: CUE definition path cannot be empty")
	errKeyringServiceRequired          = errors.New("s99config: keyring references require ReferenceOptions.KeyringService")
	errNoParserAssociated              = errors.New("s99config: no parser associated with config file")
	errReferenceMissing                = errors.New("s99config: referenced value is not available")
	errSecretFactoryNil                = errors.New("s99config: secret factory cannot be nil")
	errSensitiveMapStringKeysRequired  = errors.New("s99config: sensitive values require string-keyed maps containing s99config.Secret or any values")
)
