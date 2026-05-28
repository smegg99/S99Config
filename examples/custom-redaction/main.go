package main

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
	loader, err := s99config.New(schema, s99config.WithSecretFactory(newStyledSecret))
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

	fmt.Printf("red marker: %s\n", config.RedMarker)
	fmt.Printf("partial:    %s\n", config.Partial)
	fmt.Printf("fingerprint:%s\n", config.Fingerprint)
	fmt.Printf("public JSON: %s\n", public)
	fmt.Printf("explicit use site received secret: %t\n", send(config.Partial.Reveal()))
}

func send(secret string) bool {
	return secret != ""
}
