// keyring.go
package s99config

import (
	"fmt"

	"github.com/zalando/go-keyring"
)

// keyringService returns the configured keyring service.
func (l *Loader) keyringService() (string, error) {
	if l.references == nil || l.references.KeyringService == "" {
		return "", errKeyringServiceRequired
	}
	return l.references.KeyringService, nil
}

// SetKeyringValue stores a value in the configured keyring service.
func (l *Loader) SetKeyringValue(key, value string) error {
	service, err := l.keyringService()
	if err != nil {
		return err
	}
	if err := keyring.Set(service, key, value); err != nil {
		return fmt.Errorf("keyring set %q: %w", key, err)
	}
	return nil
}

// GetKeyringValue retrieves a value from the configured keyring service.
func (l *Loader) GetKeyringValue(key string) (string, error) {
	service, err := l.keyringService()
	if err != nil {
		return "", err
	}
	value, err := keyring.Get(service, key)
	if err != nil {
		return "", fmt.Errorf("keyring get %q: %w", key, err)
	}
	return value, nil
}

// DeleteKeyringValue removes a value from the configured keyring service.
func (l *Loader) DeleteKeyringValue(key string) error {
	service, err := l.keyringService()
	if err != nil {
		return err
	}
	if err := keyring.Delete(service, key); err != nil {
		return fmt.Errorf("keyring delete %q: %w", key, err)
	}
	return nil
}
