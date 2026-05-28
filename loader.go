// loader.go
package s99config

import (
	"sync"

	"cuelang.org/go/cue"
	"github.com/knadh/koanf/v2"
)

const defaultDefinition = "#Config"

// Loader validates configuration with one embedded CUE schema.
type Loader struct {
	mu                    sync.RWMutex
	ctx                   *cue.Context
	def                   cue.Value
	definition            string
	secretPaths           map[string]struct{}
	referenceDefaultPaths map[string]struct{}
	references            *ReferenceOptions
	secretFactory         SecretFactory
	configPath            string
	parser                koanf.Parser
	raw                   map[string]any
	resolved              map[string]any
	public                map[string]any
	private               *privateTracker
	secrets               *secretStore
}

// loadedState carries prepared config state before applying it.
type loadedState struct {
	raw      map[string]any
	resolved map[string]any
	public   map[string]any
	private  *privateTracker
	secrets  *secretStore
}
