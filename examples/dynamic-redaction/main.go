package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/smegg99/s99config"
)

//go:generate go run cuelang.org/go/cmd/cue@v0.16.1 exp gengotypes .

//go:embed settings.cue
var schema []byte

func main() {
	_ = os.Setenv("DYNAMIC_API_KEY", "dynamic-secret-value")

	loader, err := s99config.New(schema, s99config.WithReferences(s99config.ReferenceOptions{}))
	if err != nil {
		log.Fatal(err)
	}
	if err := loader.Load("config.yaml"); err != nil {
		log.Fatal(err)
	}

	resolved, err := loader.Map()
	if err != nil {
		log.Fatal(err)
	}
	var decoded map[string]any
	if err := loader.Decode(&decoded); err != nil {
		log.Fatal(err)
	}
	encoded, err := json.Marshal(decoded)
	if err != nil {
		log.Fatal(err)
	}
	public, err := loader.PublicJSON()
	if err != nil {
		log.Fatal(err)
	}

	var typed Config
	typedErr := loader.Decode(&typed)

	fmt.Printf("resolved map api_key: %s\n", resolved["api_key"])
	fmt.Printf("decoded map api_key:  %s\n", decoded["api_key"])
	fmt.Printf("decoded map JSON:     %s\n", encoded)
	fmt.Printf("public JSON:          %s\n", public)
	fmt.Printf("typed struct without Secret field fails safely: %t\n", typedErr != nil)
}
