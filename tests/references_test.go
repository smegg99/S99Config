// tests/references_test.go

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
		[]byte(`#Config: {value?: string}`),
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

// TestKeyringReferencesStayPrivate checks a keyring reference resolves and stays out of public JSON.
func TestKeyringReferencesStayPrivate(t *testing.T) {
	keyring.MockInit()
	loader, err := s99config.New(
		schema,
		s99config.WithReferences(s99config.ReferenceOptions{KeyringService: "s99config-tests"}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := keyring.Set("s99config-tests", "token", "keyring-secret"); err != nil {
		t.Fatal(err)
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
}

// A missing optional reference leaves the field to its schema default.
func TestMissingOptionalReferenceUsesTheDefault(t *testing.T) {
	loader, err := s99config.New(
		[]byte(`#Config: {region: string | *"eu"}`),
		s99config.WithReferences(s99config.ReferenceOptions{}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := loader.LoadJSON("config.json", []byte(`{"region":"@{pubenv?:S99_MISSING_REGION}"}`)); err != nil {
		t.Fatal(err)
	}
	resolved, err := loader.Map()
	if err != nil {
		t.Fatal(err)
	}
	if resolved["region"] != "eu" {
		t.Fatalf("region = %#v, want the default %q", resolved["region"], "eu")
	}
}

// A missing optional secret reference gives a handle that is not set.
func TestMissingOptionalSecretIsNotSet(t *testing.T) {
	loader, err := s99config.New(schema, s99config.WithReferences(s99config.ReferenceOptions{}))
	if err != nil {
		t.Fatal(err)
	}
	if err := loader.LoadJSON("config.json", []byte(`{"app":{"name":"demo"},"token":"@{env?:S99_MISSING_TOKEN}"}`)); err != nil {
		t.Fatal(err)
	}
	var got Config
	if err := loader.Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Token == nil || got.Token.IsSet() {
		t.Fatalf("token = %#v after a missing optional reference, want an unset handle", got.Token)
	}
}

// Without WithReferences a reference-shaped value is an error, not a silent literal.
func TestReferenceShapedValueNeedsReferencesEnabled(t *testing.T) {
	loader, err := s99config.New([]byte(`#Config: {value: string}`))
	if err != nil {
		t.Fatal(err)
	}
	err = loader.LoadJSON("config.json", []byte(`{"value":"@{env:S99_TOKEN}"}`))
	if err == nil || !strings.Contains(err.Error(), "/value") {
		t.Fatalf("LoadJSON error = %v, want one naming /value", err)
	}
	if err := loader.LoadJSON("config.json", []byte(`{"value":"@{env}"}`)); err != nil {
		t.Fatalf("a malformed reference stays literal: %v", err)
	}
}

// @{statedir:...} joins StateDir, which defaults to DataDir.
func TestStateDirReferences(t *testing.T) {
	for _, tc := range []struct {
		options s99config.ReferenceOptions
		want    string
	}{
		{s99config.ReferenceOptions{DataDir: "/data", StateDir: "/state"}, filepath.Join("/state", "app.db")},
		{s99config.ReferenceOptions{DataDir: "/data"}, filepath.Join("/data", "app.db")},
	} {
		loader, err := s99config.New([]byte(`#Config: {db: string}`), s99config.WithReferences(tc.options))
		if err != nil {
			t.Fatal(err)
		}
		if err := loader.LoadJSON("config.json", []byte(`{"db":"@{statedir:app.db}"}`)); err != nil {
			t.Fatal(err)
		}
		resolved, err := loader.Map()
		if err != nil {
			t.Fatal(err)
		}
		if resolved["db"] != tc.want {
			t.Fatalf("db = %#v, want %q", resolved["db"], tc.want)
		}
	}
}

// Goptivum's shape: a secret whose schema default is an optional env reference that is unset.
func TestAMissedOptionalSecretDefaultDecodesUnset(t *testing.T) {
	type cfg struct {
		Passphrase s99config.Secret `json:"passphrase"`
	}
	for name, file := range map[string]string{
		"no file value":      `{}`,
		"written by default": `{"passphrase":"@{env?:S99_MISSING_PASS}"}`,
	} {
		loader, err := s99config.New(
			[]byte(`#Config: {passphrase: string | *"@{env?:S99_MISSING_PASS}"}`),
			s99config.WithReferences(s99config.ReferenceOptions{}),
		)
		if err != nil {
			t.Fatal(err)
		}
		if err := loader.LoadJSON("config.json", []byte(file)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var got cfg
		if err := loader.Decode(&got); err != nil {
			t.Fatalf("%s: Decode: %v", name, err)
		}
		if got.Passphrase == nil || got.Passphrase.IsSet() {
			t.Fatalf("%s: passphrase = %#v, want an unset handle", name, got.Passphrase)
		}
		resolved, _ := loader.Map()
		if _, ok := resolved["passphrase"].(s99config.Secret); !ok {
			t.Fatalf("%s: Map passphrase = %#v, want a Secret", name, resolved["passphrase"])
		}
	}
}

// A missed optional value over a reference default resolves the default.
func TestAMissedOptionalValueFallsToAReferenceDefault(t *testing.T) {
	t.Setenv("S99_DEFAULT_REGION", "eu")
	loader, err := s99config.New(
		[]byte(`#Config: {region: string | *"@{pubenv:S99_DEFAULT_REGION}"}`),
		s99config.WithReferences(s99config.ReferenceOptions{}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := loader.LoadJSON("config.json", []byte(`{"region":"@{pubenv?:S99_MISSING_REGION}"}`)); err != nil {
		t.Fatal(err)
	}
	resolved, _ := loader.Map()
	if resolved["region"] != "eu" {
		t.Fatalf("region = %#v, want the default's value", resolved["region"])
	}
}

// A missed optional secret in a list stays a secret, unset.
func TestAMissedOptionalSecretInAListIsUnset(t *testing.T) {
	loader, err := s99config.New(
		[]byte(`#Config: {tokens: [...string]}`),
		s99config.WithReferences(s99config.ReferenceOptions{}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := loader.LoadJSON("config.json", []byte(`{"tokens":["@{env?:S99_MISSING_TOKEN}"]}`)); err != nil {
		t.Fatal(err)
	}
	resolved, _ := loader.Map()
	tokens, _ := resolved["tokens"].([]any)
	if len(tokens) != 1 {
		t.Fatalf("tokens = %#v", resolved["tokens"])
	}
	if secret, ok := tokens[0].(s99config.Secret); !ok || secret.IsSet() {
		t.Fatalf("tokens[0] = %#v, want an unset Secret", tokens[0])
	}
}
