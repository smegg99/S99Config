package s99config

import "errors"

// ErrNotLoaded is returned when an operation requires loaded configuration.
var ErrNotLoaded = errors.New("configuration has not been loaded")

// Internal sentinel errors reused across the package.
var (
	errCompileSchemaNilFilesystem      = errors.New("compile schema: schema filesystem is nil")
	errConfigurationMultipleJSONValues = errors.New("configuration contains multiple JSON values")
	errConfigurationNotObject          = errors.New("configuration must be a JSON object")
	errDecodeDestinationNil            = errors.New("decode destination cannot be nil")
	errDefinitionPathEmpty             = errors.New("CUE definition path cannot be empty")
	errKeyringServiceRequired          = errors.New("keyring references require ReferenceOptions.KeyringService")
	errNoParserAssociated              = errors.New("no parser associated with config file")
	errReferenceMissing                = errors.New("referenced value is not available")
	errSecretFactoryNil                = errors.New("secret factory cannot be nil")
	errSensitiveMapStringKeysRequired  = errors.New("sensitive values require string-keyed maps containing s99config.Secret or any values")
)
