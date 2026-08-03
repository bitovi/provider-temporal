package config

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestAccountIDFromAPIKey(t *testing.T) {
	payload, _ := json.Marshal(map[string]string{"account_id": "abc12"})
	jwt := "hdr." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
	if got := accountIDFromAPIKey(jwt); got != "abc12" {
		t.Fatalf("got %q want abc12", got)
	}
	if got := accountIDFromAPIKey("not-a-jwt"); got != "" {
		t.Fatalf("expected empty for non-jwt, got %q", got)
	}
}

func TestTemporalCloudNamespaceID(t *testing.T) {
	payload, _ := json.Marshal(map[string]string{"account_id": "x72yu"})
	jwt := "hdr." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
	params := map[string]any{"name": "my-local-dev"}

	// Named map type like terraform.ProviderConfiguration
	type namedCfg map[string]any
	setupNamed := map[string]any{
		"configuration": namedCfg{"api_key": jwt},
	}
	got, err := temporalCloudNamespaceID(context.Background(), "", params, setupNamed)
	if err != nil {
		t.Fatal(err)
	}
	if got != "my-local-dev.x72yu" {
		t.Fatalf("named config type: got %q", got)
	}

	setupPlain := map[string]any{
		"configuration": map[string]any{"api_key": jwt},
	}
	got, err = temporalCloudNamespaceID(context.Background(), "", params, setupPlain)
	if err != nil {
		t.Fatal(err)
	}
	if got != "my-local-dev.x72yu" {
		t.Fatalf("plain map: got %q", got)
	}

	got, err = temporalCloudNamespaceID(context.Background(), "already.set", params, setupPlain)
	if err != nil {
		t.Fatal(err)
	}
	if got != "already.set" {
		t.Fatalf("external-name should win: got %q", got)
	}
}
