// tests/schema.go
package tests

import "embed"

//go:generate go run cuelang.org/go/cmd/cue@v0.16.1 exp gengotypes .

//go:embed settings.cue
var schema []byte

//go:embed multischema/*.cue
var multiSchema embed.FS
