// Package httpapi exposes the library service as a JSON REST API under
// /api/v1, and serves the embedded web UI everywhere else.
//
// The API contract is tools/openapi.yaml; package apigen (routes, request
// parsing and wire types) is generated from it. After editing the spec, run
// `go generate ./tools`.
package httpapi

import (
	"errors"
	"io/fs"
	"net/http"

	"generic-arr-project/internal/httpapi/apigen"
	"generic-arr-project/internal/library"

	"github.com/rs/zerolog"
)

// New returns the HTTP handler. The API routes come from the generated
// apigen package; New puts them behind the authentication guards:
//
//   - /api/v1/health, /api/v1/setup/status, /api/v1/auth/session and
//     /api/v1/auth/login are public.
//   - Other /api/v1/setup routes need setupToken in X-Setup-Token, and only
//     work until onboarding completes.
//   - Everything else under /api/ needs the API key (X-Api-Key header or
//     apikey query parameter) or a session cookie, and returns 503 until
//     onboarding completes.
//   - All other paths serve the web UI from ui (may be nil).
//
// setupToken may be empty when setup is already complete.
func New(svc *library.Services, setupToken string, log *zerolog.Logger, ui fs.FS) (http.Handler, error) {
	if svc == nil {
		return nil, errors.New("httpapi: service is required")
	}
	if !svc.Auth.SetupComplete() && setupToken == "" {
		return nil, errors.New("httpapi: a setup token is required until setup is complete")
	}
	logger := log.With().Str("component", "httpapi").Logger()
	h := &handlers{svc: svc, log: &logger}

	_, router, err := loadSpec()
	if err != nil {
		return nil, err
	}
	strict := apigen.NewStrictHandlerWithOptions(h, []apigen.StrictMiddlewareFunc{withRequest}, apigen.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  h.badRequest,
		ResponseErrorHandlerFunc: h.writeErr,
	})
	// Every generated route; the guards below decide who reaches which.
	api := validateRequests(router, apigen.HandlerWithOptions(strict, apigen.StdHTTPServerOptions{ErrorHandlerFunc: h.badRequest}))

	authed := http.NewServeMux()
	authed.HandleFunc("GET /api/v1/posters/{size}/{file}", h.poster)
	authed.Handle("/", api)

	root := http.NewServeMux()
	for _, public := range []string{"GET /api/v1/health", "GET /api/v1/setup/status", "GET /api/v1/auth/session", "POST /api/v1/auth/login"} {
		root.Handle(public, api)
	}
	root.Handle("POST /api/v1/auth/logout", sameOrigin(api))

	guardedSetup := requireSetupToken(svc, setupToken, api)
	root.Handle("/api/v1/setup", guardedSetup)
	root.Handle("/api/v1/setup/", guardedSetup)

	// The folder picker, the VPN provider and indexer site forms, and
	// managing site definitions are needed both by the setup wizard and
	// afterwards.
	setupOrAuth := setupTokenOrAuth(svc, setupToken, api)
	root.Handle("GET /api/v1/fs/dirs", setupOrAuth)
	root.Handle("GET /api/v1/vpn/providers", setupOrAuth)
	root.Handle("GET /api/v1/vpn/providers/{id}/options/{field}", setupOrAuth)
	root.Handle("GET /api/v1/indexers/definitions", setupOrAuth)
	root.Handle("POST /api/v1/indexers/definitions", setupOrAuth)
	root.Handle("POST /api/v1/indexers/definitions/reload", setupOrAuth)
	root.Handle("DELETE /api/v1/indexers/definitions/{definitionId}", setupOrAuth)

	root.Handle("/api/", requireAuth(svc, authed))
	root.Handle("/", spaHandler(ui))
	return root, nil
}

// handlers implements the generated API.
type handlers struct {
	svc *library.Services
	log *zerolog.Logger
}

var _ apigen.StrictServerInterface = (*handlers)(nil)
