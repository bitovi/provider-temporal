package config

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/crossplane/upjet/v2/pkg/config"
)

// ExternalNameConfigs contains all external name configurations for this
// provider. Only resources listed here are generated (include list).
var ExternalNameConfigs = map[string]config.ExternalName{
	// Temporal Cloud namespace ID is name.<account_id> after create.
	// Upjet seeds tfstate with GetIDFn(...) before first create so Observe can
	// run. An empty id makes the Temporal TF provider fail with InvalidArgument
	// ("namespace: value is required"). A short name-only id often returns
	// PermissionDenied instead of NotFound. Seed with the fully-qualified id
	// (name.account_id from the API key JWT) so refresh can correctly return
	// NotFound and Create can run. After create the real Cloud id becomes
	// crossplane.io/external-name.
	"temporalcloud_namespace": temporalCloudNamespaceExternalName(),
}

func temporalCloudNamespaceExternalName() config.ExternalName {
	return config.ExternalName{
		SetIdentifierArgumentFn: config.NopSetIdentifierArgument,
		GetExternalNameFn:       config.IDAsExternalName,
		GetIDFn:                 temporalCloudNamespaceID,
		DisableNameInitializer:  true,
	}
}

func temporalCloudNamespaceID(_ context.Context, externalName string, parameters map[string]any, setup map[string]any) (string, error) {
	if externalName != "" {
		return externalName, nil
	}
	name, _ := parameters["name"].(string)
	if name == "" {
		return "", nil
	}
	if accountID := accountIDFromSetup(setup); accountID != "" {
		return name + "." + accountID, nil
	}
	// Fallback: still better than empty for InvalidArgument; may PermissionDenied.
	return name, nil
}

func accountIDFromSetup(setup map[string]any) string {
	if setup == nil {
		return ""
	}
	// setup comes from terraform.Setup.Map(); configuration may be a named map
	// type (ProviderConfiguration), not map[string]any.
	cfg := asStringMap(setup["configuration"])
	if cfg == nil {
		return ""
	}
	apiKey, _ := cfg["api_key"].(string)
	return accountIDFromAPIKey(apiKey)
}

// asStringMap coerces map-like values (including named map types) to map[string]any.
func asStringMap(v any) map[string]any {
	if v == nil {
		return nil
	}
	if m, ok := v.(map[string]any); ok {
		return m
	}
	// terraform.ProviderConfiguration is type map[string]any with a distinct name;
	// type-assert fails — round-trip through JSON.
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil
	}
	return m
}

// accountIDFromAPIKey decodes the Temporal Cloud API key JWT payload and reads
// account_id. No signature verification — used only to form provisional resource IDs.
func accountIDFromAPIKey(apiKey string) string {
	apiKey = strings.TrimSpace(apiKey)
	parts := strings.Split(apiKey, ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return ""
		}
	}
	var claims struct {
		AccountID string `json:"account_id"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	return claims.AccountID
}

// ExternalNameConfigurations applies all external name configs listed in the
// table ExternalNameConfigs and sets the version of those resources to v1beta1
// assuming they will be tested.
func ExternalNameConfigurations() config.ResourceOption {
	return func(r *config.Resource) {
		if e, ok := ExternalNameConfigs[r.Name]; ok {
			r.ExternalName = e
		}
	}
}

// ExternalNameConfigured returns the list of all resources whose external name
// is configured manually.
func ExternalNameConfigured() []string {
	l := make([]string, len(ExternalNameConfigs))
	i := 0
	for name := range ExternalNameConfigs {
		// $ is added to match the exact string since the format is regex.
		l[i] = name + "$"
		i++
	}
	return l
}
