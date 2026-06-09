# S99Config

`s99config` is a small Go configuration loader built around CUE schemas.

It loads JSON, YAML, and TOML files, validates them against an embedded CUE
definition, applies CUE defaults, resolves configured references, and decodes
the final value into your generated Go config type.

## Why

I usually handled configuration by copying a project-specific config package
from one of my previous projects into the next one. After a while I noticed I was really using the
same package everywhere, with only small changes to the schema and application
types. So I separated that common part into one reusable package.

The application keeps its own CUE schema and generated Go type; `s99config`
does the repetitive work of loading files, applying defaults, validating
values, and resolving references. It is quite handy for the way I build
projects. Is it the best or most efficient way? Probably not, but it works for me.

## Features

- CUE validation and defaults for application config.
- JSON, YAML, and TOML loading through Koanf.
- Generated Go struct support via `cue exp gengotypes`.
- Optional references for environment variables, OS keyring values, config-dir
  paths, and data-dir paths.
- Secret handles that redact by default in `fmt`, JSON, text marshaling, and
  `log/slog`.
- Public JSON export with private values omitted.
- Default-file writing and validated patching.

## Install

Requires Go 1.25 or newer.

```sh
go get github.com/smegg99/s99config
```

## Quick Start

Create a CUE schema:

```cue
package main

#Config: {
	server: {
		host: string | *"127.0.0.1"
		port: int & >0 & <=65535 | *8080
	}
	api_token: string @go(APIToken,type=Secret)
	log_dir:   string
	region:    string | *"local"
}
```

Create a config file:

```yaml
server:
  host: localhost
api_token: "@{env:API_TOKEN}"
log_dir: "@{datadir:logs}"
region: "@{pubenv?:APP_REGION}"
```

Add local aliases for generated field types:

```go
package main

import "github.com/smegg99/s99config"

type Secret = s99config.Secret
```

Load and decode:

```go
package main

import (
	_ "embed"
	"log"

	"github.com/smegg99/s99config"
)

//go:generate go run cuelang.org/go/cmd/cue@v0.16.1 exp gengotypes .

//go:embed config.cue
var schema []byte

func main() {
	loader, err := s99config.New(
		schema,
		s99config.WithReferences(s99config.ReferenceOptions{DataDir: "./data"}),
	)
	if err != nil {
		log.Fatal(err)
	}
	if err := loader.Load("config.yaml"); err != nil {
		log.Fatal(err)
	}

	var cfg Config
	if err := loader.Decode(&cfg); err != nil {
		log.Fatal(err)
	}

	token := cfg.APIToken.Reveal()
	_ = token
}
```

Generate and run:

```sh
go generate .
API_TOKEN=secret APP_REGION=eu-west go run .
```

For multiple `.cue` files, embed an `fs.FS` and use `NewFS`:

```go
//go:embed *.cue
var schemas embed.FS

loader, err := s99config.NewFS(schemas, ".")
```

## References

References are resolved only when `WithReferences` is enabled.

| Reference | Reads from | Public JSON |
| --- | --- | --- |
| `@{env:NAME}` | Environment variable | omitted |
| `@{keyring:NAME}` | OS keyring | omitted |
| `@{pubenv:NAME}` | Environment variable | included |
| `@{cfgdir:path}` | Config file directory | included |
| `@{datadir:path}` | `ReferenceOptions.DataDir` | included |

Add `?` after the source to allow missing values:

```yaml
region: "@{pubenv?:APP_REGION}"
```

Resolved values are treated as literal strings. If `APP_REGION` contains
`@{env:OTHER}`, it stays that exact text.

Keyring references require `ReferenceOptions.KeyringService`. They use
`go-keyring`, which maps to Keychain on macOS, Credential Manager on Windows,
and Secret Service backends such as GNOME Keyring or KWallet on Linux. Headless
Linux, containers, and CI often have no unlocked keyring backend.

```go
loader, err := s99config.New(
	schema,
	s99config.WithReferences(s99config.ReferenceOptions{
		KeyringService: "my-application",
	}),
)

_ = loader.SetKeyringValue("api-token", "secret")
value, err := loader.GetKeyringValue("api-token")
_ = value
_ = loader.DeleteKeyringValue("api-token")
```

## Secrets

Private references, currently `env` and `keyring`, are automatically sensitive.
Sensitive values decode into `s99config.Secret` handles and print as
`[redacted]`.

```go
fmt.Println(cfg.APIToken)      // [redacted]
fmt.Println(cfg.APIToken.IsSet())
token := cfg.APIToken.Reveal() // explicit plaintext access
```

Use `@secret()` for literal config values that must also be protected:

```cue
#Config: {
	api_token: string @secret() @go(APIToken,type=Secret)
}
```

`Decode` refuses to put a protected value into a plain `string` field. For
dynamic use, `Map`, `RawMap`, and `Decode(&map[string]any{})` return secret
handles for protected values.

`PublicJSON` and `ExportPublicJSON` omit protected values. Values derived from
a secret are not detected automatically; mark the derived field `@secret()` if
it must stay private.

Custom redaction is available with `WithSecretFactory` and
`NewPresentedSecret`. See [`examples/custom-redaction`](./examples/custom-redaction).

## Writing Config

```go
defaults, err := loader.DefaultsJSON()
err = loader.WriteDefaults("config.yaml")
err = loader.Patch(map[string]any{"server": map[string]any{"port": 9090}})
public, err := loader.PublicJSON()
err = loader.ExportPublicJSON("public-config.json")
```

`WriteDefaults` requires defaults for all required fields and creates new files
with `0600` permissions. `Patch` validates before writing, preserves raw
reference expressions, and rewrites the file in its original format. Comments
and hand formatting are not preserved.

## Examples

Run examples from their own directories, for example:

```sh
cd examples/basic
go run .
```

- [`examples/basic`](./examples/basic): multi-file CUE schema and YAML loading.
- [`examples/references`](./examples/references): private and public references.
- [`examples/configured-secret`](./examples/configured-secret): literal secret values.
- [`examples/dynamic-redaction`](./examples/dynamic-redaction): map-based usage.
- [`examples/custom-redaction`](./examples/custom-redaction): custom secret presentation.
- [`examples/formats`](./examples/formats): JSON, YAML, and TOML loading.

## License

MIT
