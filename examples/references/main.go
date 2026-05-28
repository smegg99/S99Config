package main

// main.go

import (
	_ "embed"
	"fmt"
	"log"

	"github.com/smegg99/s99config"
)

//go:generate go run cuelang.org/go/cmd/cue@v0.16.1 exp gengotypes .

//go:embed settings.cue
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

	var config Config
	if err := loader.Decode(&config); err != nil {
		log.Fatal(err)
	}

	public, err := loader.PublicJSON()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("private env redacted in config: %s\n", config.PrivateValues.FromEnv)
	fmt.Printf("private env is available for an explicit use site: %t\n", config.PrivateValues.FromEnv.IsSet())
	fmt.Printf("public env resolved: %s\n", config.PublicValues.FromEnv)
	fmt.Printf("optional missing env: %q\n", config.PublicValues.OptionalValue)
	fmt.Printf("public JSON: %s\n", public)
}
