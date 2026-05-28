// schema.go
package s99config

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
)

// New constructs a Loader from a CUE schema embedded by the application.
//
//	//go:embed config.cue
//	var schema []byte
//
//	loader, err := s99config.New(schema)
func New(schema []byte, opts ...Option) (*Loader, error) {
	settings, err := configureOptions(opts)
	if err != nil {
		return nil, err
	}

	ctx := cuecontext.New()
	return newLoader(ctx, ctx.CompileBytes(schema, cue.Filename("schema.cue")), settings)
}

// NewFS constructs a Loader from all .cue files in dir of an embedded
// filesystem. The files are compiled together as one CUE package.
//
//	//go:embed schema/*.cue
//	var schemas embed.FS
//
//	loader, err := s99config.NewFS(schemas, "schema")
func NewFS(schemaFS fs.FS, dir string, opts ...Option) (*Loader, error) {
	if schemaFS == nil {
		return nil, errCompileSchemaNilFilesystem
	}

	settings, err := configureOptions(opts)
	if err != nil {
		return nil, err
	}

	if dir == "" {
		dir = "."
	}
	dir = path.Clean(dir)
	if !fs.ValidPath(dir) {
		return nil, fmt.Errorf("compile schema: invalid filesystem directory %q", dir)
	}

	files, err := fs.Glob(schemaFS, path.Join(dir, "*.cue"))
	if err != nil {
		return nil, fmt.Errorf("compile schema: find CUE files in %q: %w", dir, err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("compile schema: no .cue files in %q", dir)
	}

	// CUE's loader needs absolute filenames for in-memory overlay sources.
	root := filepath.Join(filepath.VolumeName(os.TempDir())+string(filepath.Separator), ".s99config", "embedded-schema")
	overlay := make(map[string]load.Source, len(files))
	for _, name := range files {
		content, err := fs.ReadFile(schemaFS, name)
		if err != nil {
			return nil, fmt.Errorf("compile schema: read %q: %w", name, err)
		}
		overlay[filepath.Join(root, filepath.Base(name))] = load.FromBytes(content)
	}

	instances := load.Instances([]string{"."}, &load.Config{
		Dir:        root,
		ModuleRoot: root,
		Overlay:    overlay,
	})
	if len(instances) != 1 {
		return nil, fmt.Errorf("compile schema: loaded %d CUE instances, want 1", len(instances))
	}

	ctx := cuecontext.New()
	return newLoader(ctx, ctx.BuildInstance(instances[0]), settings)
}

// configureOptions applies user options to defaults.
func configureOptions(opts []Option) (options, error) {
	settings := options{definition: defaultDefinition, secretFactory: defaultSecretFactory}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(&settings); err != nil {
			return options{}, fmt.Errorf("configure loader: %w", err)
		}
	}
	return settings, nil
}

// newLoader validates a compiled schema and builds a Loader.
func newLoader(ctx *cue.Context, compiled cue.Value, settings options) (*Loader, error) {
	if err := compiled.Err(); err != nil {
		return nil, fmt.Errorf("compile schema: %w", err)
	}

	def := compiled.LookupPath(cue.ParsePath(settings.definition))
	if err := def.Err(); err != nil {
		return nil, fmt.Errorf("lookup CUE definition %s: %w", settings.definition, err)
	}
	secretPaths, err := collectSecretPaths(def)
	if err != nil {
		return nil, fmt.Errorf("inspect CUE definition %s: %w", settings.definition, err)
	}
	referenceDefaultPaths, err := collectReferenceDefaultPaths(def)
	if err != nil {
		return nil, fmt.Errorf("inspect CUE definition %s defaults: %w", settings.definition, err)
	}

	return &Loader{
		ctx:                   ctx,
		def:                   def,
		definition:            settings.definition,
		secretPaths:           secretPaths,
		referenceDefaultPaths: referenceDefaultPaths,
		references:            settings.references,
		secretFactory:         settings.secretFactory,
	}, nil
}
