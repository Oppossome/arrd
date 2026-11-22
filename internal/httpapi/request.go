package httpapi

import (
	"context"
	"net/http"

	"generic-arr-project/internal/httpapi/apigen"
)

type requestKey struct{}

// withRequest makes the HTTP request available to the few handlers that
// need more than the typed request object (cookies, client address, TLS).
func withRequest(next apigen.StrictHandlerFunc, _ string) apigen.StrictHandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
		return next(context.WithValue(ctx, requestKey{}, r), w, r, request)
	}
}

// httpRequest returns the request stored by withRequest.
func httpRequest(ctx context.Context) *http.Request {
	r, ok := ctx.Value(requestKey{}).(*http.Request)
	if !ok {
		panic("httpapi: no HTTP request in context; is the withRequest middleware installed?")
	}
	return r
}
