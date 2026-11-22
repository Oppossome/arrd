package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestVPNProviderRoutes(t *testing.T) {
	srv := newUnconfiguredServer(t)
	tok := map[string]string{"X-Setup-Token": testSetupToken}

	// The setup wizard reads provider forms with the setup token.
	if code, _ := rawDo(t, srv, "GET", "/api/v1/vpn/providers", "", nil); code != http.StatusUnauthorized {
		t.Errorf("providers without token = %d", code)
	}
	code, body := rawDo(t, srv, "GET", "/api/v1/vpn/providers", "", tok)
	var defs []struct {
		ID     string `json:"id"`
		Fields []struct {
			Key    string `json:"key"`
			Secret bool   `json:"secret"`
		} `json:"fields"`
	}
	if code != http.StatusOK || json.Unmarshal([]byte(body), &defs) != nil || len(defs) != 2 || defs[0].ID != "wireguard" || defs[1].ID != "pia" {
		t.Fatalf("providers = %d %s", code, body)
	}
	if code, _ := rawDo(t, srv, "GET", "/api/v1/vpn/providers/nowhere/options/region", "", tok); code != http.StatusNotFound {
		t.Errorf("options for unknown provider = %d", code)
	}
	if code, _ := rawDo(t, srv, "GET", "/api/v1/vpn/providers/pia/options/username", "", tok); code != http.StatusNotFound {
		t.Errorf("options for a text field = %d", code)
	}

	// After setup the token no longer works; the API key does.
	if code, body := doWith(t, srv, "POST", "/api/v1/setup", setupBody(srv.base), tok); code != http.StatusCreated {
		t.Fatalf("setup = %d %v", code, body)
	} else {
		srv.apiKey = body["api_key"].(string)
	}
	if code, _ := rawDo(t, srv, "GET", "/api/v1/vpn/providers", "", tok); code != http.StatusUnauthorized {
		t.Errorf("providers with setup token after setup = %d", code)
	}
	if code, _ := rawDo(t, srv, "GET", "/api/v1/vpn/providers", "", map[string]string{"X-Api-Key": srv.apiKey}); code != http.StatusOK {
		t.Errorf("providers with API key = %d", code)
	}
}

const fictionalPrivateKey = "yAnz5TF+lXXJte14tji3zlMNq+hd2rYUIgJBgB3fBmk="

func TestVPNSettingsNeverReturnSecrets(t *testing.T) {
	srv, _ := newTestServer(t)
	cfg := "[Interface]\nPrivateKey = " + fictionalPrivateKey + "\nAddress = 10.64.1.2/32\n\n[Peer]\nPublicKey = xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg=\nAllowedIPs = 0.0.0.0/0\nEndpoint = 198.51.100.7:51820\n"
	in, _ := json.Marshal(map[string]any{"provider": "wireguard", "values": map[string]string{"config": cfg}})
	key := map[string]string{"X-Api-Key": srv.apiKey}

	code, body := rawDo(t, srv, "PUT", "/api/v1/settings/vpn", string(in), key)
	if code != http.StatusOK || strings.Contains(body, fictionalPrivateKey) {
		t.Fatalf("put vpn = %d %s", code, body)
	}
	code, body = rawDo(t, srv, "GET", "/api/v1/settings/vpn", "", key)
	var vs struct {
		Provider   string            `json:"provider"`
		Values     map[string]string `json:"values"`
		SecretsSet []string          `json:"secrets_set"`
	}
	if code != http.StatusOK || json.Unmarshal([]byte(body), &vs) != nil {
		t.Fatalf("get vpn = %d %s", code, body)
	}
	if strings.Contains(body, fictionalPrivateKey) || vs.Provider != "wireguard" || len(vs.SecretsSet) != 1 || vs.SecretsSet[0] != "config" {
		t.Errorf("get vpn leaks or misreports secrets: %s", body)
	}

	// Saving again with the secret left empty keeps it.
	if code, body := rawDo(t, srv, "PUT", "/api/v1/settings/vpn", `{"provider":"wireguard","values":{}}`, key); code != http.StatusOK {
		t.Errorf("put without secret = %d %s", code, body)
	}
	if code, body := doWith(t, srv, "PUT", "/api/v1/settings/vpn", `{"provider":"nowhere","values":{}}`, key); code != http.StatusBadRequest || body["field"] != "provider" {
		t.Errorf("unknown provider = %d %v", code, body)
	}
	if code, _ := rawDo(t, srv, "DELETE", "/api/v1/settings/vpn", "", key); code != http.StatusNoContent {
		t.Errorf("delete vpn = %d", code)
	}
}
