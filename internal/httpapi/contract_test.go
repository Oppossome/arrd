package httpapi

import (
	"bytes"
	"io"
	"net/http"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
)

// contractChecker is an http.RoundTripper that fails the test when a
// response does not match tools/openapi.yaml. Every test server's client uses it,
// so all API tests double as contract tests.
type contractChecker struct {
	t      *testing.T
	router routers.Router
	next   http.RoundTripper
}

func checkContract(t *testing.T, client *http.Client) {
	t.Helper()
	_, router, err := loadSpec()
	if err != nil {
		t.Fatal(err)
	}
	client.Transport = &contractChecker{t: t, router: router, next: client.Transport}
}

func (c *contractChecker) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := c.next.RoundTrip(req)
	if err != nil || !strings.HasPrefix(req.URL.Path, "/api/") {
		return resp, err
	}
	route, params, err := c.router.FindRoute(req)
	if err != nil {
		// Not an API operation (e.g. a wrong method): the mux answered.
		return resp, nil
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	in := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{Request: req, PathParams: params, Route: route},
		Status:                 resp.StatusCode,
		Header:                 resp.Header,
		Body:                   io.NopCloser(bytes.NewReader(body)),
		Options:                &openapi3filter.Options{IncludeResponseStatus: true},
	}
	if err := openapi3filter.ValidateResponse(req.Context(), in); err != nil {
		c.t.Errorf("%s %s: response %d breaks the OpenAPI contract: %v\nbody: %s", req.Method, req.URL.Path, resp.StatusCode, err, body)
	}
	return resp, nil
}

// guard is who may call an operation, as documented by its security.
type guard int

const (
	guardPublic      guard = iota // security: []
	guardSetup                    // setup token only, until onboarding completes
	guardSetupOrAuth              // setup token before onboarding, credentials after
	guardAuth                     // API key or session (the spec default)
)

func operationGuard(spec *openapi3.T, op *openapi3.Operation) guard {
	reqs := spec.Security
	if op.Security != nil {
		reqs = *op.Security
	}
	if len(reqs) == 0 {
		return guardPublic
	}
	var schemes []string
	for _, r := range reqs {
		for name := range r {
			schemes = append(schemes, name)
		}
	}
	switch {
	case slices.Contains(schemes, "setupToken") && len(schemes) == 1:
		return guardSetup
	case slices.Contains(schemes, "setupToken"):
		return guardSetupOrAuth
	default:
		return guardAuth
	}
}

// examplePath fills a spec path's parameters with plausible values.
func examplePath(path string) string {
	r := strings.NewReplacer("{id}", "1", "{season}", "1", "{field}", "region", "{size}", "w342", "{file}", "x.jpg")
	return r.Replace(path)
}

// TestGuardsMatchSpecSecurity checks that each operation is reachable by
// exactly whom its documented security says, before and after onboarding.
func TestGuardsMatchSpecSecurity(t *testing.T) {
	spec, _, err := loadSpec()
	if err != nil {
		t.Fatal(err)
	}
	type op struct {
		method, path string
		guard        guard
	}
	var ops []op
	for path, item := range spec.Paths.Map() {
		for method, o := range item.Operations() {
			ops = append(ops, op{method, examplePath(path), operationGuard(spec, o)})
		}
	}
	sort.Slice(ops, func(i, j int) bool { return ops[i].path+ops[i].method < ops[j].path+ops[j].method })

	before := newUnconfiguredServer(t)
	for _, o := range ops {
		code, _ := rawDo(t, before, o.method, o.path, "", nil)
		var ok bool
		switch o.guard {
		case guardPublic:
			ok = code != http.StatusUnauthorized
		case guardSetup, guardSetupOrAuth:
			ok = code == http.StatusUnauthorized
		case guardAuth:
			ok = code == http.StatusServiceUnavailable
		}
		if !ok {
			t.Errorf("before setup, %s %s without credentials = %d (guard %d)", o.method, o.path, code, o.guard)
		}
	}

	after, _ := newTestServer(t)
	for _, o := range ops {
		code, _ := rawDo(t, after, o.method, o.path, "", nil)
		var ok bool
		switch o.guard {
		case guardPublic:
			ok = code != http.StatusUnauthorized
		case guardSetup:
			ok = code == http.StatusConflict
		case guardSetupOrAuth, guardAuth:
			ok = code == http.StatusUnauthorized
		}
		if !ok {
			t.Errorf("after setup, %s %s without credentials = %d (guard %d)", o.method, o.path, code, o.guard)
		}
	}
}

// TestSpecCoversRoutes checks that every documented operation is served:
// none falls through to the SPA or the mux's 404/405.
func TestSpecCoversRoutes(t *testing.T) {
	spec, _, err := loadSpec()
	if err != nil {
		t.Fatal(err)
	}
	srv, _ := newTestServer(t)
	for path, item := range spec.Paths.Map() {
		for method := range item.Operations() {
			if method == http.MethodPost && path == "/api/v1/auth/logout" {
				continue // needs Origin; covered by the browser tests
			}
			code, body := rawDo(t, srv, method, examplePath(path), "", map[string]string{"X-Api-Key": srv.apiKey})
			if code == http.StatusMethodNotAllowed || (code == http.StatusNotFound && !strings.Contains(body, `"error"`)) || strings.Contains(body, "<html") {
				t.Errorf("%s %s is not served: %d %s", method, path, code, body)
			}
		}
	}
}
