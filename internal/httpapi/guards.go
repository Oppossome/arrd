package httpapi

import (
	"crypto/subtle"
	"net/http"
	"net/url"
	"strings"

	"generic-arr-project/internal/library"
)

// requireAuth accepts the API key or a session cookie, after onboarding.
// Cookie-authenticated requests that change state must come from this
// origin (CSRF protection); API-key requests are exempt because browsers
// never attach the key on their own.
func requireAuth(svc *library.Service, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !svc.SetupComplete() {
			writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: setupRequiredMessage})
			return
		}
		key := r.Header.Get("X-Api-Key")
		if key == "" {
			key = r.URL.Query().Get("apikey")
		}
		if key != "" {
			if !svc.CheckAPIKey(key) {
				writeJSON(w, http.StatusUnauthorized, errorBody{Error: "missing or invalid API key"})
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		ok, err := svc.ValidateSession(r.Context(), sessionCookie(r))
		if err != nil || !ok {
			writeJSON(w, http.StatusUnauthorized, errorBody{Error: "not signed in"})
			return
		}
		sameOrigin(next).ServeHTTP(w, r)
	})
}

// requireSetupToken admits onboarding requests carrying the token, only
// while setup is incomplete.
func requireSetupToken(svc *library.Service, token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if svc.SetupComplete() {
			writeJSON(w, http.StatusConflict, errorBody{Error: "setup is already complete"})
			return
		}
		if !validSetupToken(r, token) {
			writeJSON(w, http.StatusUnauthorized, errorBody{Error: "missing or invalid setup token (printed in the arrd log)"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// setupTokenOrAuth uses the setup token before onboarding and normal
// authentication after it.
func setupTokenOrAuth(svc *library.Service, token string, next http.Handler) http.Handler {
	authed := requireAuth(svc, next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if svc.SetupComplete() {
			authed.ServeHTTP(w, r)
			return
		}
		if !validSetupToken(r, token) {
			writeJSON(w, http.StatusUnauthorized, errorBody{Error: "missing or invalid setup token (printed in the arrd log)"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func validSetupToken(r *http.Request, token string) bool {
	got := r.Header.Get("X-Setup-Token")
	return token != "" && subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1
}

// sameOrigin rejects state-changing requests whose Origin (or, failing
// that, Referer) is not this host. Safe methods pass through.
func sameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		source := r.Header.Get("Origin")
		if source == "" {
			source = r.Header.Get("Referer")
		}
		u, err := url.Parse(source)
		if source == "" || err != nil || !strings.EqualFold(u.Host, r.Host) {
			writeJSON(w, http.StatusForbidden, errorBody{Error: "cross-origin request refused"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
