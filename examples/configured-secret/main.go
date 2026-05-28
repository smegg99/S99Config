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
	loader, err := s99config.New(schema)
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

	fmt.Printf("service=%s api_key=%s configured=%t\n",
		config.Service, config.APIKey, config.APIKey.IsSet())
	fmt.Printf("public JSON: %s\n", public)

	connect(config.APIKey.Reveal())
}

func connect(apiKey string) {
	fmt.Printf("explicit use site received an API key: %t\n", apiKey != "")
}
