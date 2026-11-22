package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"

	"generic-arr-project/internal/httpapi/apigen"
)

const maxBodyBytes = 1 << 20

// loadSpec returns the embedded OpenAPI spec and a router over it.
func loadSpec() (*openapi3.T, routers.Router, error) {
	spec, err := apigen.GetSwagger()
	if err != nil {
		return nil, nil, fmt.Errorf("httpapi: load OpenAPI spec: %w", err)
	}
	router, err := gorillamux.NewRouter(spec)
	if err != nil {
		return nil, nil, fmt.Errorf("httpapi: build OpenAPI router: %w", err)
	}
	return spec, router, nil
}

// validateRequests rejects requests that do not match the OpenAPI spec
// (unknown or missing fields, wrong types, values outside an enum) with a
// 400 before they reach a handler. Authentication is not checked here: the
// route guards in New do that first. Requests for routes the spec does not
// know pass through, so the mux answers them with 404 or 405.
func validateRequests(router routers.Router, next http.Handler) http.Handler {
	opts := &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route, params, err := router.FindRoute(r)
		if err != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
			next.ServeHTTP(w, r)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, bodyLimit(route.Operation))
		// The API only speaks JSON and has always parsed bodies as JSON
		// whatever the header says; keep accepting clients that omit it.
		if r.Header.Get("Content-Type") == "" && r.ContentLength != 0 {
			r.Header.Set("Content-Type", "application/json")
		}
		in := &openapi3filter.RequestValidationInput{Request: r, PathParams: params, Route: route, Options: opts}
		if err := openapi3filter.ValidateRequest(r.Context(), in); err != nil {
			writeJSON(w, http.StatusBadRequest, requestErrorBody(err))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// bodyLimit is the largest request body an operation accepts: its
// x-max-body-bytes extension (e.g. for uploads), or maxBodyBytes.
func bodyLimit(op *openapi3.Operation) int64 {
	if op == nil {
		return maxBodyBytes
	}
	switch v := op.Extensions["x-max-body-bytes"].(type) {
	case float64:
		return int64(v)
	case int:
		return int64(v)
	case int64:
		return v
	}
	return maxBodyBytes
}

// requestErrorBody turns a validation failure into a short message, naming
// the offending parameter or body field when there is one.
func requestErrorBody(err error) errorBody {
	var re *openapi3filter.RequestError
	if !errors.As(err, &re) {
		return errorBody{Error: "invalid request: " + err.Error()}
	}
	reason := re.Reason
	field := ""
	var se *openapi3.SchemaError
	if errors.As(re.Err, &se) {
		reason = se.Reason
		field = strings.Join(se.JSONPointer(), ".")
	} else if re.Err != nil && reason == "" {
		reason = re.Err.Error()
	}
	if re.Parameter != nil {
		return errorBody{Error: fmt.Sprintf("invalid %s %q: %s", re.Parameter.In, re.Parameter.Name, reason), Field: re.Parameter.Name}
	}
	if field != "" {
		return errorBody{Error: fmt.Sprintf("invalid request body: %s: %s", field, reason), Field: field}
	}
	return errorBody{Error: "invalid request body: " + reason}
}
