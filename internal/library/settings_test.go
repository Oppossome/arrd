package library

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"generic-arr-project/internal/formspec"
)

func TestSettingsValidation(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	env := newEnv(t, base, "http://unused", nil, nil)
	svc := env.svc

	// Onboarding with the suggested defaults (created on demand).
	suggested := svc.SetupStatus().Suggested
	setup := SetupInput{
		AdminPassword: testPassword,
		TMDBAPIKey:    "k",
		Downloads:     DownloadSettings{IncompleteDir: suggested.IncompleteDir, CompleteDir: suggested.CompleteDir},
		MovieRoot:     filepath.Join(base, "m1"),
		CreateMissing: true,
	}
	if _, err := svc.CompleteSetup(ctx, setup); err != nil {
		t.Fatal(err)
	}
	set, err := svc.DownloadSettings(ctx)
	if err != nil || set.IncompleteDir != filepath.Join(env.dataDir, "downloads", "incomplete") || set.ImportMode != "hardlink" {
		t.Fatalf("settings after setup = %+v, %v", set, err)
	}

	bad := set
	bad.CompleteDir = filepath.Join(base, "missing")
	assertField(t, func() error { _, err := svc.UpdateDownloadSettings(ctx, bad); return err }, "complete_dir")
	bad = set
	bad.ImportMode = "symlink"
	assertField(t, func() error { _, err := svc.UpdateDownloadSettings(ctx, bad); return err }, "import_mode")

	newDir := filepath.Join(base, "dl2")
	if err := os.Mkdir(newDir, 0o755); err != nil {
		t.Fatal(err)
	}
	good := set
	good.CompleteDir = newDir
	upd, err := svc.UpdateDownloadSettings(ctx, good)
	if err != nil || upd.Settings.CompleteDir != newDir {
		t.Fatalf("update = %+v, %v", upd, err)
	}

	assertField(t, func() error {
		_, err := svc.UpdateQualitySettings(ctx, QualitySettings{MinResolution: 2160, MaxResolution: 720})
		return err
	}, "min_resolution")

	// Root folders: first is default, deleting it promotes the next.
	roots, _ := svc.RootFolders(ctx, MediaMovie)
	if len(roots) != 1 || !roots[0].IsDefault {
		t.Fatalf("seeded roots = %+v", roots)
	}
	m2 := filepath.Join(base, "m2")
	if err := os.Mkdir(m2, 0o755); err != nil {
		t.Fatal(err)
	}
	second, err := svc.AddRootFolder(ctx, MediaMovie, m2)
	if err != nil || second.IsDefault {
		t.Fatalf("second root = %+v, %v", second, err)
	}
	if _, err := svc.AddRootFolder(ctx, MediaMovie, m2); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("duplicate root: %v", err)
	}
	if err := svc.DeleteRootFolder(ctx, roots[0].ID); err != nil {
		t.Fatal(err)
	}
	roots, _ = svc.RootFolders(ctx, MediaMovie)
	if len(roots) != 1 || roots[0].ID != second.ID || !roots[0].IsDefault {
		t.Errorf("after delete = %+v", roots)
	}

	// Indexers: validation, write-only secrets, keep secrets on update.
	assertField(t, func() error {
		_, err := svc.CreateIndexer(ctx, siteIndexer("x", "http://elsewhere.invalid/"))
		return err
	}, "settings.base_url")
	assertField(t, func() error {
		_, err := svc.CreateIndexer(ctx, IndexerInput{Name: "x", Definition: testSiteID})
		return err
	}, "settings.base_url")
	assertField(t, func() error {
		in := siteIndexer("x", "http://site-a.invalid/")
		in.Settings["nonsense"] = "1"
		_, err := svc.CreateIndexer(ctx, in)
		return err
	}, "settings.nonsense")
	assertField(t, func() error {
		_, err := svc.CreateIndexer(ctx, IndexerInput{Name: "x", Settings: formspec.Values{"base_url": "http://site-a.invalid/"}})
		return err
	}, "definition")
	assertField(t, func() error {
		in := siteIndexer("x", "http://site-a.invalid/")
		in.Definition = "nowhere"
		_, err := svc.CreateIndexer(ctx, in)
		return err
	}, "definition")
	in := siteIndexerKey("x", "http://site-a.invalid/", "secret")
	in.MovieCategories = []int{2000, 2040}
	ix, err := svc.CreateIndexer(ctx, in)
	if err != nil || ix.Definition != testSiteID || len(ix.MovieCategories) != 2 {
		t.Fatalf("indexer = %+v, %v", ix, err)
	}
	if !slices.Equal(ix.SecretsSet, []string{"apikey"}) || ix.Settings["apikey"] != "" || ix.Settings["base_url"] != "http://site-a.invalid/" {
		t.Errorf("secret shown or not marked set: settings %v, secrets_set %v", ix.Settings, ix.SecretsSet)
	}
	disabled := false
	// Definition omitted on update keeps the saved one; an empty secret keeps the saved one.
	ix, err = svc.UpdateIndexer(ctx, ix.ID, IndexerInput{Name: "x2", Settings: formspec.Values{"base_url": "http://site-b.invalid/"}, Enabled: &disabled})
	if err != nil || ix.Name != "x2" || ix.Enabled || ix.Definition != testSiteID || ix.Settings["base_url"] != "http://site-b.invalid/" {
		t.Fatalf("updated indexer = %+v, %v", ix, err)
	}
	row, _ := env.store.GetIndexer(ctx, ix.ID)
	if !strings.Contains(row.Settings, `"apikey":"secret"`) {
		t.Errorf("empty apikey on update cleared the stored key: %s", row.Settings)
	}
	var ce *ConflictError
	if _, err := svc.CreateIndexer(ctx, onSite(siteIndexer("x2", "http://site-b.invalid/"), otherSiteID)); !errors.As(err, &ce) {
		t.Errorf("duplicate indexer name = %v, want conflict", err)
	}
	if _, err := svc.CreateIndexer(ctx, siteIndexer("another", "http://site-a.invalid/")); !errors.As(err, &ce) || !strings.Contains(err.Error(), "already has an indexer") {
		t.Errorf("second indexer for one site = %v, want conflict", err)
	}

	// Switching off and on keeps the settings; the name defaults to the site's.
	enabled := true
	ix, err = svc.UpdateIndexer(ctx, ix.ID, IndexerInput{Enabled: &enabled})
	if err != nil || !ix.Enabled || ix.Name != "x2" || ix.Settings["base_url"] != "http://site-b.invalid/" {
		t.Fatalf("switched on = %+v, %v", ix, err)
	}
	row, _ = env.store.GetIndexer(ctx, ix.ID)
	if !strings.Contains(row.Settings, `"apikey":"secret"`) {
		t.Errorf("switching on cleared the stored key: %s", row.Settings)
	}
	other, err := svc.CreateIndexer(ctx, IndexerInput{Definition: otherSiteID, Settings: formspec.Values{"base_url": "http://site-a.invalid/"}})
	if err != nil || other.Name != "Other Site" {
		t.Errorf("indexer without a name = %+v, %v", other, err)
	}
}

func assertField(t *testing.T, f func() error, field string) {
	t.Helper()
	err := f()
	ve, ok := err.(*ValidationError)
	if !ok || ve.Field != field {
		t.Errorf("err = %v, want validation error on %q", err, field)
	}
}
