package httpapi

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestIndexerDefinitionRoutes(t *testing.T) {
	srv := newUnconfiguredServer(t)
	tok := map[string]string{"X-Setup-Token": testSetupToken}

	// The setup wizard reads the site forms with the setup token.
	if code, _ := rawDo(t, srv, "GET", "/api/v1/indexers/definitions", "", nil); code != http.StatusUnauthorized {
		t.Errorf("definitions without token = %d", code)
	}
	code, body := rawDo(t, srv, "GET", "/api/v1/indexers/definitions", "", tok)
	var defs []struct {
		ID     string `json:"id"`
		RSS    bool   `json:"rss"`
		Fields []struct {
			Key      string `json:"key"`
			Kind     string `json:"kind"`
			Required bool   `json:"required"`
			Secret   bool   `json:"secret"`
			Choices  []struct {
				Value string `json:"value"`
			} `json:"choices"`
		} `json:"fields"`
	}
	if code != http.StatusOK || json.Unmarshal([]byte(body), &defs) != nil || len(defs) != 1 || defs[0].ID != "examplesite" || !defs[0].RSS {
		t.Fatalf("definitions = %d %s", code, body)
	}
	f := defs[0].Fields
	if len(f) != 2 || f[0].Key != "base_url" || f[0].Kind != "select" || !f[0].Required || len(f[0].Choices) != 1 || f[1].Key != "apikey" || !f[1].Secret {
		t.Errorf("fields = %+v", f)
	}
	// Other indexer routes are not opened by the setup token.
	if code, _ := rawDo(t, srv, "GET", "/api/v1/indexers", "", tok); code == http.StatusOK {
		t.Errorf("indexers with setup token = %d", code)
	}
}

func TestReloadIndexerDefinitions(t *testing.T) {
	srv, _ := newTestServer(t)
	code, body := do(t, srv, "POST", "/api/v1/indexers/definitions/reload", "", true)
	defs, _ := body["definitions"].([]any)
	errs, _ := body["errors"].([]any)
	if code != http.StatusOK || len(defs) != 1 || errs == nil || len(errs) != 0 {
		t.Fatalf("reload = %d %v", code, body)
	}
	if code, _ := do(t, srv, "POST", "/api/v1/indexers/definitions/reload", "", false); code != http.StatusUnauthorized {
		t.Errorf("reload without auth = %d", code)
	}
}

func TestUploadAndDeleteIndexerDefinitions(t *testing.T) {
	srv := newUnconfiguredServer(t)
	tok := map[string]string{"X-Setup-Token": testSetupToken, "Content-Type": "application/octet-stream"}
	other := strings.NewReplacer("id: examplesite", "id: othersite", "name: Example Site", "name: Other Site").Replace(testSiteDefinition)

	// A zip larger than the API's usual 1 MiB body limit, padded with a
	// file that is not a definition.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range map[string]string{"defs/v11/othersite.yml": other, "defs/v11/broken.yml": "id: [unclosed", "defs/padding.bin": randomText(2 << 20)} {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(w, body); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	if code, _ := rawDo(t, srv, "POST", "/api/v1/indexers/definitions", buf.String(), map[string]string{"Content-Type": "application/octet-stream"}); code != http.StatusUnauthorized {
		t.Errorf("upload without token = %d", code)
	}
	code, body := rawDo(t, srv, "POST", "/api/v1/indexers/definitions", buf.String(), tok)
	var up struct {
		Saved       []string `json:"saved"`
		Errors      []string `json:"errors"`
		Definitions []struct {
			ID string `json:"id"`
		} `json:"definitions"`
	}
	if code != http.StatusOK || json.Unmarshal([]byte(body), &up) != nil {
		t.Fatalf("upload = %d %s", code, body)
	}
	if len(up.Saved) != 1 || up.Saved[0] != "othersite" || len(up.Errors) != 1 || len(up.Definitions) != 2 {
		t.Errorf("upload = %+v", up)
	}

	// Something that is neither a definition nor a zip is a field error.
	code, body = rawDo(t, srv, "POST", "/api/v1/indexers/definitions", "just some text", tok)
	if code != http.StatusBadRequest || !strings.Contains(body, `"field":"file"`) {
		t.Errorf("bad upload = %d %s", code, body)
	}

	if code, body := rawDo(t, srv, "DELETE", "/api/v1/indexers/definitions/othersite", "", tok); code != http.StatusNoContent {
		t.Fatalf("delete = %d %s", code, body)
	}
	if code, _ := rawDo(t, srv, "DELETE", "/api/v1/indexers/definitions/othersite", "", tok); code != http.StatusNotFound {
		t.Errorf("delete again = %d", code)
	}
	code, body = rawDo(t, srv, "GET", "/api/v1/indexers/definitions", "", tok)
	if code != http.StatusOK || strings.Contains(body, "othersite") {
		t.Errorf("definitions after delete = %d %s", code, body)
	}
}

// randomText is n bytes that do not compress.
func randomText(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return string(b)
}
