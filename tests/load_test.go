package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/smegg99/s99config"
)

// TestLoadSupportedFormats checks JSON, YAML, and TOML loading.
func TestLoadSupportedFormats(t *testing.T) {
	cases := map[string]string{
		"testdata/config.json": "demo",
		"testdata/config.yaml": "yaml-demo",
		"testdata/config.toml": "toml-demo",
	}
	for path, name := range cases {
		t.Run(filepath.Ext(path), func(t *testing.T) {
			var got Config
			loader, err := s99config.New(schema)
			if err != nil {
				t.Fatal(err)
			}
			if err := loader.Load(path); err != nil {
				t.Fatal(err)
			}
			if err := loader.Decode(&got); err != nil {
				t.Fatal(err)
			}
			if got.App.Name != name || got.App.Port != 8080 {
				t.Fatalf("loaded settings = %+v", got)
			}
		})
	}
}

// TestEmbeddedMultiFileSchema checks embedded multi-file CUE schemas.
func TestEmbeddedMultiFileSchema(t *testing.T) {
	loader, err := s99config.NewFS(multiSchema, "multischema")
	if err != nil {
		t.Fatal(err)
	}
	if err := loader.LoadJSON("config.json", []byte(`{"app":{"name":"multi"}}`)); err != nil {
		t.Fatal(err)
	}

	var got struct {
		App struct {
			Name string `json:"name"`
			Port int    `json:"port"`
		} `json:"app"`
	}
	if err := loader.Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.App.Name != "multi" || got.App.Port != 9090 {
		t.Fatalf("loaded settings = %+v", got)
	}
}

// TestSchemaRejectsInvalidConfig checks CUE validation failures.
func TestSchemaRejectsInvalidConfig(t *testing.T) {
	loader, err := s99config.New(schema)
	if err != nil {
		t.Fatal(err)
	}

	err = loader.LoadJSON("invalid.json", []byte(`{"app":{"name":"demo","port":0}}`))
	if err == nil {
		t.Fatal("expected invalid port to be rejected")
	}
}

// TestDefinitionViewsAndPublicExport checks custom definitions and views.
func TestDefinitionViewsAndPublicExport(t *testing.T) {
	loader, err := s99config.New(
		[]byte(`#Settings: {name: string | *"demo"}`),
		s99config.WithDefinition("#Settings"),
	)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := loader.Load(path); err != nil {
		t.Fatal(err)
	}
	if loader.ConfigPath() == "" {
		t.Fatal("ConfigPath is empty")
	}
	raw, err := loader.RawMap()
	if err != nil || len(raw) != 0 {
		t.Fatalf("raw = %#v, %v", raw, err)
	}
	resolved, err := loader.Map()
	if err != nil || resolved["name"] != "demo" {
		t.Fatalf("resolved = %#v, %v", resolved, err)
	}
	publicPath := filepath.Join(t.TempDir(), "public.json")
	if err := loader.ExportPublicJSON(publicPath); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(publicPath)
	if err != nil || !strings.Contains(string(content), `"name": "demo"`) {
		t.Fatalf("public export = %s, %v", content, err)
	}
}
