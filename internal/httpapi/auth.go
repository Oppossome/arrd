package httpapi

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"generic-arr-project/internal/httpapi/apigen"
)

const (
	sessionCookieName = "arrd_session"
	sessionCookieAge  = 30 * 24 * time.Hour
)

func sessionCookie(r *http.Request) string {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	return c.Value
}

// secureRequest reports whether the client reached us over HTTPS, directly
// or through a TLS-terminating reverse proxy.
func secureRequest(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// sessionCookieFor returns the cookie that signs the client in with token.
func sessionCookieFor(r *http.Request, token string) *http.Cookie {
	return &http.Cookie{
		Name: sessionCookieName, Value: token, Path: "/",
		MaxAge: int(sessionCookieAge.Seconds()), HttpOnly: true,
		SameSite: http.SameSiteStrictMode, Secure: secureRequest(r),
	}
}

// expiredSessionCookie returns the cookie that signs the client out.
func expiredSessionCookie(r *http.Request) *http.Cookie {
	return &http.Cookie{
		Name: sessionCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true,
		SameSite: http.SameSiteStrictMode, Secure: secureRequest(r),
	}
}

// clientIP is the address used for login rate limiting. Forwarding headers
// are ignored on purpose: they are trivially spoofed.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// Session reports what the UI should show: setup, login or the app.
func (h *handlers) Session(ctx context.Context, _ apigen.SessionRequestObject) (apigen.SessionResponseObject, error) {
	out := apigen.Session200JSONResponse{SetupComplete: h.svc.SetupComplete()}
	if out.SetupComplete {
		ok, err := h.svc.ValidateSession(ctx, sessionCookie(httpRequest(ctx)))
		if err != nil {
			return nil, err
		}
		out.Authenticated = ok
	}
	return out, nil
}

func (h *handlers) Login(ctx context.Context, req apigen.LoginRequestObject) (apigen.LoginResponseObject, error) {
	if !h.svc.SetupComplete() {
		return nil, errSetupRequired
	}
	r := httpRequest(ctx)
	token, err := h.svc.Login(ctx, req.Body.Password, clientIP(r))
	if err != nil {
		return nil, err
	}
	cookie := sessionCookieFor(r, token).String()
	return apigen.Login204Response{Headers: apigen.Login204ResponseHeaders{SetCookie: &cookie}}, nil
}

func (h *handlers) Logout(ctx context.Context, _ apigen.LogoutRequestObject) (apigen.LogoutResponseObject, error) {
	r := httpRequest(ctx)
	if err := h.svc.Logout(ctx, sessionCookie(r)); err != nil {
		return nil, err
	}
	cookie := expiredSessionCookie(r).String()
	return apigen.Logout204Response{Headers: apigen.Logout204ResponseHeaders{SetCookie: &cookie}}, nil
}

// ChangePassword keeps the caller's own session and ends all others.
func (h *handlers) ChangePassword(ctx context.Context, req apigen.ChangePasswordRequestObject) (apigen.ChangePasswordResponseObject, error) {
	err := h.svc.ChangePassword(ctx, req.Body.CurrentPassword, req.Body.NewPassword, sessionCookie(httpRequest(ctx)))
	if err != nil {
		return nil, err
	}
	return apigen.ChangePassword204Response{}, nil
}
