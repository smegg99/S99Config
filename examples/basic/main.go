package main

// main.go

import (
	"embed"
	"fmt"
	"log"

	"github.com/smegg99/s99config"
)

//go:generate go run cuelang.org/go/cmd/cue@v0.16.1 exp gengotypes .

//go:embed *.cue
var schemas embed.FS

func main() {
	loader, err := s99config.NewFS(schemas, ".")
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

	fmt.Printf("listening on %s:%d\n", config.Server.Host, config.Server.Port)
}
