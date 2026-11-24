package setup_test

import (
	"context"
	"errors"
	"generic-arr-project/internal/library/apperr"
	"generic-arr-project/internal/library/indexers"
	"generic-arr-project/internal/library/libtest"
	"generic-arr-project/internal/library/settings"
	"generic-arr-project/internal/library/setup"
	"generic-arr-project/internal/library/vpnconfig"
	"slices"
	"testing"
)

func TestSetupRequiresPasswordAndStartsSession(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	env := libtest.NewEnv(t, base, "http://unused", nil, nil)
	in := *libtest.BasicSetup(base)
	in.AdminPassword = "short"
	if _, err := env.Svc.Setup.CompleteSetup(ctx, in); err == nil {
		t.Fatal("short admin password accepted")
	}
	in.AdminPassword = libtest.TestPassword
	res, err := env.Svc.Setup.CompleteSetup(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := env.Svc.Auth.ValidateSession(ctx, res.SessionToken); !ok {
		t.Error("setup did not start a session")
	}
}

func TestSetupValidationAndOnce(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	env := libtest.NewEnv(t, base, "http://unused", nil, nil)
	svc := env.Svc

	// Before setup: no engine, nothing processed, grabs refused.
	if svc.Auth.SetupComplete() || svc.Engine.RuntimeError() == nil {
		t.Fatal("service claims to be set up")
	}
	if err := svc.Acquire.ProcessDownloads(ctx); err != nil {
		t.Errorf("ProcessDownloads before setup = %v, want no-op", err)
	}
	if svc.Auth.CheckAPIKey("") || svc.Auth.CheckAPIKey("anything") {
		t.Error("API key accepted before setup")
	}

	mutate := func(f func(*setup.SetupInput)) setup.SetupInput {
		in := *libtest.BasicSetup(base)
		f(&in)
		return in
	}
	cases := []struct {
		field string
		in    setup.SetupInput
	}{
		{"tmdb_api_key", mutate(func(in *setup.SetupInput) { in.TMDBAPIKey = " " })},
		{"vpn.config", mutate(func(in *setup.SetupInput) { v := libtest.WGInput("[Interface]\nPrivateKey = nope*\n"); in.VPN = &v })},
		{"vpn.provider", mutate(func(in *setup.SetupInput) { in.VPN = &vpnconfig.VPNInput{Provider: "nowhere-vpn"} })},
		{"movie_root", mutate(func(in *setup.SetupInput) { in.MovieRoot, in.TVRoot = "", "" })},
		{"downloads.incomplete_dir", mutate(func(in *setup.SetupInput) { in.Downloads.IncompleteDir = "relative/dir" })},
		{"indexers[1].definition", mutate(func(in *setup.SetupInput) {
			in.Indexers = []indexers.IndexerInput{libtest.SiteIndexer("a", "http://site-a.invalid/"), libtest.SiteIndexer("b", "http://site-b.invalid/")}
		})},
		{"indexers[1].name", mutate(func(in *setup.SetupInput) {
			in.Indexers = []indexers.IndexerInput{libtest.SiteIndexer("a", "http://site-a.invalid/"), libtest.OnSite(libtest.SiteIndexer("a", "http://site-b.invalid/"), libtest.OtherSiteID)}
		})},
	}
	for _, c := range cases {
		_, err := svc.Setup.CompleteSetup(ctx, c.in)
		var ve *apperr.ValidationError
		if !errors.As(err, &ve) || ve.Field != c.field {
			t.Errorf("want validation error on %s, got %v", c.field, err)
		}
	}
	if svc.Auth.SetupComplete() {
		t.Fatal("a failed setup marked setup complete")
	}

	in := mutate(func(in *setup.SetupInput) {
		in.Indexers = []indexers.IndexerInput{libtest.SiteIndexerKey("idx", "http://site-a.invalid/", "s")}
	})
	res, err := svc.Setup.CompleteSetup(ctx, in)
	if err != nil || res.APIKey == "" || res.RuntimeError != "" {
		t.Fatalf("setup = %+v, %v", res, err)
	}
	if !svc.Auth.SetupComplete() || svc.Engine.RuntimeError() != nil || svc.Engine.CurrentEngine() == nil {
		t.Fatal("engine not running after setup")
	}
	if !svc.Auth.CheckAPIKey(res.APIKey) || svc.Auth.CheckAPIKey(res.APIKey+"x") {
		t.Error("API key check wrong")
	}
	if ixs, _ := svc.Indexers.Indexers(ctx); len(ixs) != 1 || !slices.Equal(ixs[0].SecretsSet, []string{"apikey"}) {
		t.Errorf("indexers after setup = %+v", ixs)
	}
	roots, _ := svc.Settings.RootFolders(ctx, settings.MediaTV)
	if len(roots) != 1 || !roots[0].IsDefault {
		t.Errorf("tv roots = %+v", roots)
	}
	var ce *apperr.ConflictError
	if _, err := svc.Setup.CompleteSetup(ctx, in); !errors.As(err, &ce) {
		t.Errorf("second setup = %v, want conflict", err)
	}

	// Rotating the key invalidates the old one.
	newKey, err := svc.Auth.RegenerateAPIKey(ctx)
	if err != nil || svc.Auth.CheckAPIKey(res.APIKey) || !svc.Auth.CheckAPIKey(newKey) {
		t.Errorf("regenerate: %v", err)
	}

	// A restarted service picks the saved state up.
	svc.Stop()
	env2 := libtest.NewEnv(t, base, "http://unused", nil, env.Store)
	if !env2.Svc.Auth.SetupComplete() || !env2.Svc.Auth.CheckAPIKey(newKey) || env2.Svc.Engine.CurrentEngine() == nil {
		t.Error("restart lost setup state")
	}
}
