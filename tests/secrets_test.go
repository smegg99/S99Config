package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/smegg99/s99config"
)

// partialSecret presents a shortened secret in tests.
type partialSecret struct {
	s99config.PresentedSecret
}

// newPartialSecret builds a test secret presenter.
func newPartialSecret(value s99config.SecretValue) partialSecret {
	return partialSecret{PresentedSecret: s99config.NewPresentedSecret(value, func(value s99config.SecretValue) string {
		plain := value.Reveal()
		if len(plain) <= 5 {
			return "..."
		}
		return plain[:3] + "..." + plain[len(plain)-2:]
	})}
}

// TestSecretHandlesRedactDecodedAndMapValues checks default redaction.
func TestSecretHandlesRedactDecodedAndMapValues(t *testing.T) {
	t.Setenv("S99_TEST_TOKEN", "do-not-log-this")
	loader, err := s99config.New(schema, s99config.WithReferences(s99config.ReferenceOptions{}))
	if err != nil {
		t.Fatal(err)
	}
	if err := loader.LoadJSON("config.json", []byte(`{"app":{"name":"demo"},"token":"@{env:S99_TEST_TOKEN}"}`)); err != nil {
		t.Fatal(err)
	}

	var got Config
	if err := loader.Decode(&got); err != nil {
		t.Fatal(err)
	}
	if !got.Token.IsSet() || got.Token.Reveal() != "do-not-log-this" {
		t.Fatal("decoded secret handle does not reveal its loaded value")
	}
	jsonValue, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	logged := fmt.Sprintf("%+v %#v %s %s", got, got.Token, got.Token, jsonValue)
	var logBuffer bytes.Buffer
	slog.New(slog.NewTextHandler(&logBuffer, nil)).Info("config", "token", got.Token)
	logged += logBuffer.String()

	resolved, err := loader.Map()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := loader.RawMap()
	if err != nil {
		t.Fatal(err)
	}
	for name, view := range map[string]map[string]any{"Map": resolved, "RawMap": raw} {
		secret, ok := view["token"].(s99config.Secret)
		if !ok || secret.Reveal() != "do-not-log-this" {
			t.Fatalf("%s token is not a secret handle: %#v", name, view["token"])
		}
	}
	var generic map[string]any
	if err := loader.Decode(&generic); err != nil {
		t.Fatal(err)
	}
	genericSecret, ok := generic["token"].(s99config.Secret)
	if !ok || genericSecret.Reveal() != "do-not-log-this" {
		t.Fatalf("decoded map token is not a secret handle: %#v", generic["token"])
	}
	mapJSON, err := json.Marshal(resolved)
	if err != nil {
		t.Fatal(err)
	}
	logged += fmt.Sprintf(" map=%+v loader=%#v json=%s", resolved, loader, mapJSON)
	if strings.Contains(logged, "do-not-log-this") || !strings.Contains(logged, "[redacted]") {
		t.Fatalf("formatted secret was not redacted: %s", logged)
	}
}

// TestCustomSecretFactoryControlsPresentation checks custom presenters.
func TestCustomSecretFactoryControlsPresentation(t *testing.T) {
	loader, err := s99config.New(
		[]byte(`#Config: {token: string @secret()}`),
		s99config.WithSecretFactory(func(value s99config.SecretValue) s99config.Secret {
			return newPartialSecret(value)
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := loader.LoadJSON("config.json", []byte(`{"token":"configured-secret"}`)); err != nil {
		t.Fatal(err)
	}

	var got struct {
		Token partialSecret `json:"token"`
	}
	if err := loader.Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Token.Reveal() != "configured-secret" || fmt.Sprint(got.Token) != "con...et" {
		t.Fatalf("custom secret = %s reveal=%q", got.Token, got.Token.Reveal())
	}
	jsonValue, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	textValue, err := got.Token.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	var logBuffer bytes.Buffer
	slog.New(slog.NewTextHandler(&logBuffer, nil)).Info("config", "token", got.Token)
	if !strings.Contains(string(jsonValue), "con...et") ||
		string(textValue) != "con...et" ||
		!strings.Contains(logBuffer.String(), "con...et") {
		t.Fatalf("custom secret presentations: json=%s text=%s slog=%s", jsonValue, textValue, logBuffer.String())
	}

	resolved, err := loader.Map()
	if err != nil {
		t.Fatal(err)
	}
	if secret, ok := resolved["token"].(partialSecret); !ok || fmt.Sprint(secret) != "con...et" {
		t.Fatalf("Map custom secret = %#v", resolved["token"])
	}
	var generic map[string]any
	if err := loader.Decode(&generic); err != nil {
		t.Fatal(err)
	}
	if secret, ok := generic["token"].(partialSecret); !ok || fmt.Sprint(secret) != "con...et" {
		t.Fatalf("decoded map custom secret = %#v", generic["token"])
	}

	public, err := loader.PublicJSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(public) != "{}" {
		t.Fatalf("custom presentation changed public config = %s", public)
	}
}

// TestSecretFactoryRejectsNilFactory checks nil factory validation.
func TestSecretFactoryRejectsNilFactory(t *testing.T) {
	_, err := s99config.New([]byte(`#Config: {token: string @secret()}`), s99config.WithSecretFactory(nil))
	if err == nil || !strings.Contains(err.Error(), "secret factory cannot be nil") {
		t.Fatalf("nil secret factory error = %v", err)
	}
}

// TestSecretFactoryTypedNilUsesDefaultPresentation checks typed-nil fallback.
func TestSecretFactoryTypedNilUsesDefaultPresentation(t *testing.T) {
	loader, err := s99config.New(
		[]byte(`#Config: {token: string @secret()}`),
		s99config.WithSecretFactory(func(s99config.SecretValue) s99config.Secret {
			var secret *partialSecret
			return secret
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := loader.LoadJSON("config.json", []byte(`{"token":"never-display"}`)); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Token s99config.Secret `json:"token"`
	}
	if err := loader.Decode(&got); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(got.Token) != "[redacted]" || got.Token.Reveal() != "never-display" {
		t.Fatalf("typed-nil factory fallback = %s reveal=%q", got.Token, got.Token.Reveal())
	}
}

// TestDecodeRejectsPlainStringForProtectedField checks unsafe decode targets.
func TestDecodeRejectsPlainStringForProtectedField(t *testing.T) {
	t.Setenv("S99_TEST_TOKEN", "do-not-decode-as-string")
	loader, err := s99config.New(schema, s99config.WithReferences(s99config.ReferenceOptions{}))
	if err != nil {
		t.Fatal(err)
	}
	if err := loader.LoadJSON("config.json", []byte(`{"app":{"name":"demo"},"token":"@{env:S99_TEST_TOKEN}"}`)); err != nil {
		t.Fatal(err)
	}
	var unsafe struct {
		Token string `json:"token"`
	}
	err = loader.Decode(&unsafe)
	if err == nil || !strings.Contains(err.Error(), "s99config.Secret") {
		t.Fatalf("plain string decode error = %v", err)
	}
}

// TestSecretAnnotationProtectsLiteralValues checks @secret literals.
func TestSecretAnnotationProtectsLiteralValues(t *testing.T) {
	loader, err := s99config.New([]byte(`#Config: {token: string @secret()}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := loader.LoadJSON("config.json", []byte(`{"token":"literal-secret"}`)); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Token s99config.Secret `json:"token"`
	}
	if err := loader.Decode(&got); err != nil {
		t.Fatal(err)
	}
	public, err := loader.PublicJSON()
	if err != nil {
		t.Fatal(err)
	}
	if got.Token.Reveal() != "literal-secret" || strings.Contains(string(public), "token") {
		t.Fatalf("annotated secret was not protected: got=%s public=%s", got.Token, public)
	}
}

// TestSecretAnnotationProtectsQuotedFieldLabels checks escaped field paths.
func TestSecretAnnotationProtectsQuotedFieldLabels(t *testing.T) {
	loader, err := s99config.New([]byte(`#Config: {
		"a.b": string @secret()
		"path/with~characters": string @secret()
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := loader.LoadJSON("config.json", []byte(`{
		"a.b": "dotted-secret",
		"path/with~characters": "escaped-secret"
	}`)); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Dotted  s99config.Secret `json:"a.b"`
		Escaped s99config.Secret `json:"path/with~characters"`
	}
	if err := loader.Decode(&got); err != nil {
		t.Fatal(err)
	}
	public, err := loader.PublicJSON()
	if err != nil {
		t.Fatal(err)
	}
	if got.Dotted.Reveal() != "dotted-secret" || got.Escaped.Reveal() != "escaped-secret" || string(public) != "{}" {
		t.Fatalf("quoted sensitive fields were not protected: dotted=%s escaped=%s public=%s", got.Dotted, got.Escaped, public)
	}
}

// TestPrivatePathsDoNotCollideWithDottedLabels checks path escaping.
func TestPrivatePathsDoNotCollideWithDottedLabels(t *testing.T) {
	t.Setenv("S99_NESTED_SECRET", "nested-secret")
	loader, err := s99config.New(
		[]byte(`#Config: {"a.b": string, a: {b: string}}`),
		s99config.WithReferences(s99config.ReferenceOptions{}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := loader.LoadJSON("config.json", []byte(`{"a.b":"public-value","a":{"b":"@{env:S99_NESTED_SECRET}"}}`)); err != nil {
		t.Fatal(err)
	}
	public, err := loader.PublicJSON()
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(public, &got); err != nil {
		t.Fatal(err)
	}
	if got["a.b"] != "public-value" {
		t.Fatalf("public dotted field was hidden by nested secret: %s", public)
	}
	if strings.Contains(string(public), "nested-secret") {
		t.Fatalf("nested secret exposed: %s", public)
	}
}

// TestPrivateValuesAreRedactedFromValidationErrors checks error redaction.
func TestPrivateValuesAreRedactedFromValidationErrors(t *testing.T) {
	t.Setenv("S99_TEST_TOKEN", "secret-that-must-not-leak")
	loader, err := s99config.New(
		[]byte(`#Config: {token: "allowed"}`),
		s99config.WithReferences(s99config.ReferenceOptions{}),
	)
	if err != nil {
		t.Fatal(err)
	}

	err = loader.LoadJSON("config.json", []byte(`{"token":"@{env:S99_TEST_TOKEN}"}`))
	if err == nil {
		t.Fatal("expected invalid token to be rejected")
	}
	if strings.Contains(err.Error(), "secret-that-must-not-leak") {
		t.Fatalf("validation error exposes private value: %v", err)
	}
	if !strings.Contains(err.Error(), "[redacted]") {
		t.Fatalf("validation error does not show redaction: %v", err)
	}
}

// TestOverlappingPrivateValuesAreFullyRedactedFromErrors checks overlap.
func TestOverlappingPrivateValuesAreFullyRedactedFromErrors(t *testing.T) {
	t.Setenv("S99_TEST_SHORT", "shared")
	t.Setenv("S99_TEST_LONG", "shared-secret-tail")
	loader, err := s99config.New(
		[]byte(`#Config: {short: "allowed", long: "allowed"}`),
		s99config.WithReferences(s99config.ReferenceOptions{}),
	)
	if err != nil {
		t.Fatal(err)
	}

	err = loader.LoadJSON("config.json", []byte(`{"short":"@{env:S99_TEST_SHORT}","long":"@{env:S99_TEST_LONG}"}`))
	if err == nil {
		t.Fatal("expected invalid private values to be rejected")
	}
	if strings.Contains(err.Error(), "shared") || strings.Contains(err.Error(), "secret-tail") {
		t.Fatalf("validation error exposes part of a private value: %v", err)
	}
}

// TestEscapedPrivateValuesAreRedactedFromValidationErrors checks escapes.
func TestEscapedPrivateValuesAreRedactedFromValidationErrors(t *testing.T) {
	secret := "quoted \"secret\"\nwith newline"
	t.Setenv("S99_ESCAPED_SECRET", secret)
	loader, err := s99config.New(
		[]byte(`#Config: {token: "allowed"}`),
		s99config.WithReferences(s99config.ReferenceOptions{}),
	)
	if err != nil {
		t.Fatal(err)
	}

	err = loader.LoadJSON("config.json", []byte(`{"token":"@{env:S99_ESCAPED_SECRET}"}`))
	if err == nil {
		t.Fatal("expected invalid private value to be rejected")
	}
	message := err.Error()
	for _, leaked := range []string{`quoted`, `secret`, `newline`, `\"secret\"`, `\n`} {
		if strings.Contains(message, leaked) {
			t.Fatalf("validation error exposes escaped private value %q: %v", leaked, err)
		}
	}
	if !strings.Contains(message, "[redacted]") {
		t.Fatalf("validation error does not show redaction: %v", err)
	}
}

// TestPublicJSONHidesDerivedPrivateValues checks derived secret output.
func TestPublicJSONHidesDerivedPrivateValues(t *testing.T) {
	t.Setenv("S99_TEST_TOKEN", "secret-that-must-not-leak")
	loader, err := s99config.New(
		[]byte(`#Config: {token: string, echoed: "prefix-\(token)"}`),
		s99config.WithReferences(s99config.ReferenceOptions{}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := loader.LoadJSON("config.json", []byte(`{"token":"@{env:S99_TEST_TOKEN}"}`)); err != nil {
		t.Fatal(err)
	}

	publicJSON, err := loader.PublicJSON()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(publicJSON), "secret-that-must-not-leak") {
		t.Fatalf("public config contains a derived private value: %s", publicJSON)
	}
}

// TestDefaultReferencesRecomputeDerivedSecretFields checks secret defaults.
func TestDefaultReferencesRecomputeDerivedSecretFields(t *testing.T) {
	t.Setenv("S99_DEFAULT_TOKEN", "default-secret")
	loader, err := s99config.New(
		[]byte(`#Config: {
			token:  (string | *"@{env:S99_DEFAULT_TOKEN}")
			header: "Bearer \(token)"
		}`),
		s99config.WithReferences(s99config.ReferenceOptions{}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := loader.LoadJSON("config.json", []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Token  s99config.Secret `json:"token"`
		Header s99config.Secret `json:"header"`
	}
	if err := loader.Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Token.Reveal() != "default-secret" || got.Header.Reveal() != "Bearer default-secret" {
		t.Fatalf("resolved defaults = token:%s header:%s", got.Token, got.Header)
	}
	public, err := loader.PublicJSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(public) != "{}" {
		t.Fatalf("public defaults = %s", public)
	}
}
