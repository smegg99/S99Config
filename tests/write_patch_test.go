package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/smegg99/s99config"
)

// TestLargeJSONIntegersKeepPrecision checks JSON number decoding.
func TestLargeJSONIntegersKeepPrecision(t *testing.T) {
	const input = `{"id":9007199254740993}`
	for _, load := range []func(*s99config.Loader) error{
		func(loader *s99config.Loader) error {
			return loader.LoadJSON("config.json", []byte(input))
		},
		func(loader *s99config.Loader) error {
			path := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
				return err
			}
			return loader.Load(path)
		},
	} {
		loader, err := s99config.New([]byte(`#Config: {id: int}`))
		if err != nil {
			t.Fatal(err)
		}
		if err := load(loader); err != nil {
			t.Fatal(err)
		}
		var got struct {
			ID int64 `json:"id"`
		}
		if err := loader.Decode(&got); err != nil {
			t.Fatal(err)
		}
		if got.ID != 9007199254740993 {
			t.Fatalf("id = %d", got.ID)
		}
	}
}

// TestPatchKeepsLargeJSONIntegerPrecision checks JSON patch precision.
func TestPatchKeepsLargeJSONIntegerPrecision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"id":9007199254740993123456789}`), 0o644); err != nil {
		t.Fatal(err)
	}
	loader, err := s99config.New([]byte(`#Config: {id: int, name?: string}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := loader.Load(path); err != nil {
		t.Fatal(err)
	}
	if err := loader.Patch(map[string]any{"name": "updated"}); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "9007199254740993123456789") {
		t.Fatalf("patched JSON integer lost precision: %s", content)
	}
}

// TestPatchPreservesProtectedRawValues checks raw secret writes.
func TestPatchPreservesProtectedRawValues(t *testing.T) {
	t.Setenv("S99_PATCH_TOKEN", "resolved-secret")
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{name: "literal", content: `{"token":"literal-secret"}`, want: "literal-secret"},
		{name: "reference", content: `{"token":"@{env:S99_PATCH_TOKEN}"}`, want: "@{env:S99_PATCH_TOKEN}"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(path, []byte(test.content), 0o644); err != nil {
				t.Fatal(err)
			}
			loader, err := s99config.New(
				[]byte(`#Config: {token: string @secret(), name?: string}`),
				s99config.WithReferences(s99config.ReferenceOptions{}),
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := loader.Load(path); err != nil {
				t.Fatal(err)
			}
			if err := loader.Patch(map[string]any{"name": "updated"}); err != nil {
				t.Fatal(err)
			}
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(content), test.want) || strings.Contains(string(content), "[redacted]") {
				t.Fatalf("patched protected config = %s", content)
			}
			if test.name == "reference" && strings.Contains(string(content), "resolved-secret") {
				t.Fatalf("patched reference exposed resolved secret: %s", content)
			}
		})
	}
}

// TestPatchRejectsSecretHandles checks explicit secret reveal is required.
func TestPatchRejectsSecretHandles(t *testing.T) {
	loader, err := s99config.New([]byte(`#Config: {token: string @secret(), name?: string}`))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"token":"literal-secret"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := loader.Load(path); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Token s99config.Secret `json:"token"`
	}
	if err := loader.Decode(&got); err != nil {
		t.Fatal(err)
	}

	err = loader.Patch(map[string]any{"token": got.Token})
	if err == nil || !strings.Contains(err.Error(), "call Reveal explicitly") {
		t.Fatalf("patch secret handle error = %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "[redacted]") || !strings.Contains(string(content), "literal-secret") {
		t.Fatalf("patch wrote secret handle presentation: %s", content)
	}
}

// TestPatchCycleReturnsMarshalError checks cyclic patch rejection.
func TestPatchCycleReturnsMarshalError(t *testing.T) {
	loader, err := s99config.New([]byte(`#Config: {name?: string}`))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := loader.Load(path); err != nil {
		t.Fatal(err)
	}

	patch := map[string]any{}
	patch["self"] = patch
	err = loader.Patch(patch)
	if err == nil || !strings.Contains(err.Error(), "unsupported value") {
		t.Fatalf("cyclic patch error = %v", err)
	}
}

// TestNonJSONWriteRejectsOutOfRangeIntegers checks TOML/YAML limits.
func TestNonJSONWriteRejectsOutOfRangeIntegers(t *testing.T) {
	loader, err := s99config.New([]byte(`#Config: {id: int, name?: string}`))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("id = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := loader.Load(path); err != nil {
		t.Fatal(err)
	}
	err = loader.Patch(map[string]any{"id": json.Number("9007199254740993123456789")})
	if err == nil || !strings.Contains(err.Error(), "cannot be represented") {
		t.Fatalf("large TOML integer error = %v", err)
	}
}

// TestPatchPreservesYAMLFormat checks format-specific patch output.
func TestPatchPreservesYAMLFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("app:\n  name: original\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	loader, err := s99config.New(schema)
	if err != nil {
		t.Fatal(err)
	}
	if err := loader.Load(path); err != nil {
		t.Fatal(err)
	}
	if err := loader.Patch(map[string]any{"app": map[string]any{"name": "changed"}}); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "name: changed") || strings.HasPrefix(strings.TrimSpace(string(content)), "{") {
		t.Fatalf("patched YAML = %s", content)
	}
}

// TestWriteDefaultsUsesFileFormat checks default output format.
func TestWriteDefaultsUsesFileFormat(t *testing.T) {
	loader, err := s99config.New([]byte(`#Config: {app: {name: string | *"generated", port: int | *8080}}`))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := loader.WriteDefaults(path); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `[app]`) {
		t.Fatalf("generated TOML = %s", content)
	}
	if err := loader.Load(path); err != nil {
		t.Fatal(err)
	}
}

// TestWriteDefaultsCreatesPrivateFilePermissions checks new file mode.
func TestWriteDefaultsCreatesPrivateFilePermissions(t *testing.T) {
	loader, err := s99config.New([]byte(`#Config: {token: string | *"literal-secret" @secret()}`))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := loader.WriteDefaults(path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("default config permissions = %#o, want %#o", got, os.FileMode(0o600))
	}
}
