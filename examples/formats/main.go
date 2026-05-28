package main

// main.go

import (
	_ "embed"
	"fmt"
	"log"
	"os"

	"github.com/smegg99/s99config"
)

//go:generate go run cuelang.org/go/cmd/cue@v0.16.1 exp gengotypes .

//go:embed settings.cue
var schema []byte

func main() {
	_ = os.Setenv("FORMAT_SECRET", "private-env-value")
	_ = os.Setenv("FORMAT_REGION", "public-env-value")

	for _, path := range []string{
		"config/config.json",
		"config/config.yaml",
		"config/config.toml",
	} {
		if err := show(path); err != nil {
			log.Fatal(err)
		}
	}
}

func show(path string) error {
	loader, err := s99config.New(
		schema,
		s99config.WithReferences(s99config.ReferenceOptions{DataDir: "./data"}),
	)
	if err != nil {
		return err
	}
	if err := loader.Load(path); err != nil {
		return err
	}

	var config Config
	if err := loader.Decode(&config); err != nil {
		return err
	}
	public, err := loader.PublicJSON()
	if err != nil {
		return err
	}

	fmt.Printf("%s: inferred-secret=%s declared-secret=%s public=%s\n",
		config.Name, config.Private.EnvValue, config.Private.ConfiguredValue, public)
	return nil
}
