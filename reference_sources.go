// reference_sources.go
package s99config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
)

const (
	srcEnv     = "env"
	srcKeyring = "keyring"
	srcPubEnv  = "pubenv"
	srcCfgDir  = "cfgdir"
	srcDataDir = "datadir"
)

// resolve resolves a reference and handles optional misses.
func (r resolver) resolve(ref reference) (string, error) {
	value, err := r.resolveStrict(ref)
	if err != nil && ref.optional && errors.Is(err, errReferenceMissing) {
		return "", nil
	}
	return value, err
}

// resolveStrict resolves a reference without optional handling.
func (r resolver) resolveStrict(ref reference) (string, error) {
	switch ref.source {
	case srcEnv, srcPubEnv:
		value, ok := os.LookupEnv(ref.key)
		if !ok {
			return "", fmt.Errorf("%w: environment variable %q is not set", errReferenceMissing, ref.key)
		}
		return value, nil
	case srcKeyring:
		if r.options.KeyringService == "" {
			return "", errKeyringServiceRequired
		}
		value, err := keyring.Get(r.options.KeyringService, ref.key)
		if err != nil {
			if errors.Is(err, keyring.ErrNotFound) {
				return "", fmt.Errorf("%w: keyring lookup %q: %v", errReferenceMissing, ref.key, err)
			}
			return "", fmt.Errorf("keyring lookup %q: %w", ref.key, err)
		}
		return value, nil
	case srcCfgDir:
		return joinReferencePath(r.options.ConfigDir, ref.key, srcCfgDir)
	case srcDataDir:
		return joinReferencePath(r.options.DataDir, ref.key, srcDataDir)
	default:
		return "", fmt.Errorf("unknown reference source %q", ref.source)
	}
}

// joinReferencePath resolves a relative reference path.
func joinReferencePath(base, relative, source string) (string, error) {
	if base == "" {
		return "", fmt.Errorf("%s reference has no base directory", source)
	}
	return filepath.Join(base, relative), nil
}
