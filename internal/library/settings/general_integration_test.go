package settings_test

import (
	"context"
	"generic-arr-project/internal/library/libtest"
	"generic-arr-project/internal/library/settings"
	"generic-arr-project/internal/testutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGeneralSettingsValidateTMDBKey(t *testing.T) {
	ctx := context.Background()
	tmdb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/configuration" && r.URL.Query().Get("api_key") == "good" {
			testutil.Write(t, w, `{}`)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		testutil.Write(t, w, `{"status_message":"Invalid API key"}`)
	}))
	defer tmdb.Close()
	base := t.TempDir()
	env := libtest.NewEnv(t, base, tmdb.URL, libtest.BasicSetup(base), nil)

	if _, err := env.Svc.Settings.UpdateGeneralSettings(ctx, settings.GeneralSettingsInput{TMDBAPIKey: "bad"}); err == nil {
		t.Error("rejected TMDB key saved")
	}
	if gs, err := env.Svc.Settings.UpdateGeneralSettings(ctx, settings.GeneralSettingsInput{TMDBAPIKey: "good"}); err != nil || !gs.TMDBAPIKeySet {
		t.Errorf("update = %+v, %v", gs, err)
	}
	app, _ := env.Store.GetAppSettings(ctx)
	if app.TMDBAPIKey != "good" {
		t.Errorf("stored key = %q", app.TMDBAPIKey)
	}
	gs, err := env.Svc.Settings.GeneralSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if gs.TMDBAPIKeyHint != "****************" || gs.APIKeyHint != app.APIKey[:2]+"****************"+app.APIKey[len(app.APIKey)-2:] || gs.APIKeyHint == app.APIKey {
		t.Errorf("hints = %q, %q", gs.TMDBAPIKeyHint, gs.APIKeyHint)
	}
}
