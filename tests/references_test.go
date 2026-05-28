package tests

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/smegg99/s99config"
	"github.com/zalando/go-keyring"
)

// TestReferencesCanHideSecretsFromPublicJSON checks private and public refs.
func TestReferencesCanHideSecretsFromPublicJSON(t *testing.T) {
	t.Setenv("S99_TEST_TOKEN", "secret")

	loader, err := s99config.New(
		schema,
		s99config.WithReferences(s99config.ReferenceOptions{DataDir: t.TempDir()}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := loader.LoadJSON("config.json", []byte(`{
		"app":{"name":"demo"},
		"token":"@{env:S99_TEST_TOKEN}",
		"output":"@{datadir:output.json}"
	}`)); err != nil {
		t.Fatal(err)
	}

	var got Config
	if err := loader.Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Token.Reveal() != "secret" {
		t.Fatalf("token = %q", got.Token.Reveal())
	}

	publicJSON, err := loader.PublicJSON()
	if err != nil {
		t.Fatal(err)
	}
	var public map[string]any
	if err := json.Unmarshal(publicJSON, &public); err != nil {
		t.Fatal(err)
	}
	if _, ok := public["token"]; ok {
		t.Fatalf("public config contains token: %s", publicJSON)
	}
	if output, ok := public["output"].(string); !ok || filepath.Base(output) != "output.json" {
		t.Fatalf("public output = %v", public["output"])
	}
}

// TestResolvedSourceValuesAreNotRecursiveReferences checks literal ref output.
func TestResolvedSourceValuesAreNotRecursiveReferences(t *testing.T) {
	t.Setenv("S99_OUTER", "@{env:S99_INNER}")
	t.Setenv("S99_INNER", "unexpected-inner-secret")
	loader, err := s99config.New(
		[]byte(`#Config: {value: string, copied: value}`),
		s99config.WithReferences(s99config.ReferenceOptions{}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := loader.LoadJSON("config.json", []byte(`{"value":"@{pubenv:S99_OUTER}"}`)); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Value  string `json:"value"`
		Copied string `json:"copied"`
	}
	if err := loader.Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Value != "@{env:S99_INNER}" || got.Copied != "@{env:S99_INNER}" {
		t.Fatalf("source values were recursively resolved: value=%q copied=%q", got.Value, got.Copied)
	}
}

// TestDefaultPublicSourceValuesRemainLiteralWhenCopied checks default refs.
func TestDefaultPublicSourceValuesRemainLiteralWhenCopied(t *testing.T) {
	t.Setenv("S99_DEFAULT_OUTER", "@{env:S99_DEFAULT_INNER}")
	t.Setenv("S99_DEFAULT_INNER", "unexpected-inner-secret")
	loader, err := s99config.New(
		[]byte(`#Config: {value: string | *"@{pubenv:S99_DEFAULT_OUTER}", copied: value}`),
		s99config.WithReferences(s99config.ReferenceOptions{}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := loader.LoadJSON("config.json", []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Value  string `json:"value"`
		Copied string `json:"copied"`
	}
	if err := loader.Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Value != "@{env:S99_DEFAULT_INNER}" || got.Copied != "@{env:S99_DEFAULT_INNER}" {
		t.Fatalf("default source values were recursively resolved: value=%q copied=%q", got.Value, got.Copied)
	}
}

// TestOptionalReferenceOnlyIgnoresMissingValues checks optional ref behavior.
func TestOptionalReferenceOnlyIgnoresMissingValues(t *testing.T) {
	loader, err := s99config.New(
		[]byte(`#Config: {value: string}`),
		s99config.WithReferences(s99config.ReferenceOptions{}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := loader.LoadJSON("config.json", []byte(`{"value":"@{pubenv?:S99_MISSING_VALUE}"}`)); err != nil {
		t.Fatal(err)
	}

	err = loader.LoadJSON("config.json", []byte(`{"value":"@{typo?:value}"}`))
	if err == nil || !strings.Contains(err.Error(), "unknown reference source") {
		t.Fatalf("unknown optional source error = %v", err)
	}
}

// TestFileReferenceSourcesAreNotSupported checks unsupported file refs.
func TestFileReferenceSourcesAreNotSupported(t *testing.T) {
	loader, err := s99config.New(
		[]byte(`#Config: {value: string}`),
		s99config.WithReferences(s99config.ReferenceOptions{}),
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, ref := range []string{"@{file:secret.txt}", "@{pubfile:message.txt}"} {
		err := loader.LoadJSON("config.json", []byte(`{"value":"`+ref+`"}`))
		if err == nil || !strings.Contains(err.Error(), "unknown reference source") {
			t.Fatalf("reference %s error = %v", ref, err)
		}
	}
}

// TestMalformedReferenceLikeValuesRemainLiteral checks malformed ref strings.
func TestMalformedReferenceLikeValuesRemainLiteral(t *testing.T) {
	loader, err := s99config.New(
		[]byte(`#Config: {value: string}`),
		s99config.WithReferences(s99config.ReferenceOptions{}),
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, ref := range []string{"@{env}", "@{env:}", "@{:VALUE}"} {
		if err := loader.LoadJSON("config.json", []byte(`{"value":"`+ref+`"}`)); err != nil {
			t.Fatalf("reference %s error = %v", ref, err)
		}
		var got struct {
			Value string `json:"value"`
		}
		if err := loader.Decode(&got); err != nil {
			t.Fatal(err)
		}
		if got.Value != ref {
			t.Fatalf("reference value = %q, want %q", got.Value, ref)
		}
	}
}

// TestKeyringReferencesAndHelpers checks keyring refs and helper methods.
func TestKeyringReferencesAndHelpers(t *testing.T) {
	keyring.MockInit()
	loader, err := s99config.New(
		schema,
		s99config.WithReferences(s99config.ReferenceOptions{KeyringService: "s99config-tests"}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := loader.SetKeyringValue("token", "keyring-secret"); err != nil {
		t.Fatal(err)
	}
	value, err := loader.GetKeyringValue("token")
	if err != nil || value != "keyring-secret" {
		t.Fatalf("keyring token = %q, %v", value, err)
	}
	if err := loader.LoadJSON("config.json", []byte(`{"app":{"name":"demo"},"token":"@{keyring:token}"}`)); err != nil {
		t.Fatal(err)
	}
	public, err := loader.PublicJSON()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(public), "keyring-secret") || strings.Contains(string(public), "token") {
		t.Fatalf("public config exposes keyring token: %s", public)
	}
	if err := loader.DeleteKeyringValue("token"); err != nil {
		t.Fatal(err)
	}
}
