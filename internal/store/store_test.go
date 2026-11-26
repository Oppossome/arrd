package store

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"generic-arr-project/internal/store/db"
	"generic-arr-project/internal/testutil"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { testutil.Close(t, st) })
	return st
}

func TestDownloadSettingsUpsert(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)

	if _, err := st.GetDownloadSettings(ctx); err == nil {
		t.Fatal("GetDownloadSettings on empty db succeeded, want sql.ErrNoRows")
	}
	params := db.UpsertDownloadSettingsParams{IncompleteDir: "/inc", CompleteDir: "/done", ImportMode: "hardlink"}
	if _, err := st.UpsertDownloadSettings(ctx, params); err != nil {
		t.Fatal(err)
	}
	params.CompleteDir = "/done2"
	got, err := st.UpsertDownloadSettings(ctx, params)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 1 || got.CompleteDir != "/done2" {
		t.Errorf("upsert result = %+v", got)
	}
}

func TestIndexerCRUD(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)

	created, err := st.CreateIndexer(ctx, db.CreateIndexerParams{
		Name: "a", Definition: "examplebay", Settings: `{"base_url":"http://a.invalid/"}`, MovieCategories: "2000", TVCategories: "5000",
		Enabled: true, EnableRSS: false, EnableSearch: true, Priority: 10, MinSeeders: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !created.Enabled || created.EnableRSS || created.CapsJSON != nil {
		t.Errorf("created = %+v", created)
	}

	rss, err := st.ListRSSIndexers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rss) != 0 {
		t.Errorf("ListRSSIndexers = %d rows, want 0 (rss disabled)", len(rss))
	}
	search, err := st.ListSearchIndexers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(search) != 1 {
		t.Errorf("ListSearchIndexers = %d rows, want 1", len(search))
	}

	if err := st.RecordIndexerError(ctx, db.RecordIndexerErrorParams{ID: created.ID, LastError: ptr("timeout")}); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetIndexer(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.LastError == nil || *got.LastError != "timeout" || got.LastErrorAt == nil {
		t.Errorf("after RecordIndexerError: %+v", got)
	}

	n, err := st.DeleteIndexer(ctx, created.ID)
	if err != nil || n != 1 {
		t.Fatalf("DeleteIndexer = %d, %v", n, err)
	}
}

func TestWithTxSwitchesDefaultRootAtomically(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)

	first, err := st.CreateRootFolder(ctx, db.CreateRootFolderParams{MediaType: "movie", Path: "/m1", IsDefault: true})
	if err != nil {
		t.Fatal(err)
	}
	second, err := st.CreateRootFolder(ctx, db.CreateRootFolderParams{MediaType: "movie", Path: "/m2"})
	if err != nil {
		t.Fatal(err)
	}

	// A failing transaction must leave the original default intact.
	errBoom := errors.New("boom")
	err = st.WithTx(ctx, func(q *db.Queries) error {
		if err := q.ClearDefaultRootFolder(ctx, "movie"); err != nil {
			return err
		}
		return errBoom
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("WithTx err = %v, want boom", err)
	}
	if r, _ := st.GetRootFolder(ctx, first.ID); !r.IsDefault {
		t.Fatal("rolled-back transaction cleared the default root")
	}

	err = st.WithTx(ctx, func(q *db.Queries) error {
		if err := q.ClearDefaultRootFolder(ctx, "movie"); err != nil {
			return err
		}
		_, err := q.SetDefaultRootFolder(ctx, second.ID)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	roots, err := st.ListRootFolders(ctx, "movie")
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 2 || roots[0].ID != second.ID || !roots[0].IsDefault || roots[1].IsDefault {
		t.Errorf("roots after switch = %+v", roots)
	}
}

func ptr[T any](v T) *T { return &v }

func TestVPNSettingsMustBeJSONObject(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer testutil.Close(t, st)
	for _, bad := range []string{"not json", `["a"]`} {
		if _, err := st.DB.ExecContext(ctx, `INSERT INTO vpn_settings (id, provider, settings) VALUES (1, 'x', ?)`, bad); err == nil {
			t.Errorf("settings %q accepted", bad)
		}
	}
}

// Updating an indexer to another definition drops its cached capabilities,
// which described the old site; keeping the definition keeps them.
func TestUpdateIndexerDefinitionClearsCaps(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	ix, err := st.CreateIndexer(ctx, db.CreateIndexerParams{
		Name: "a", Definition: "examplebay", Settings: `{}`, MovieCategories: "2000", TVCategories: "5000",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetIndexerCaps(ctx, db.SetIndexerCapsParams{ID: ix.ID, CapsJSON: ptr(`{}`)}); err != nil {
		t.Fatal(err)
	}
	update := func(definition string) db.Indexer {
		t.Helper()
		row, err := st.UpdateIndexer(ctx, db.UpdateIndexerParams{
			ID: ix.ID, Name: "a", Definition: definition, Settings: `{}`, MovieCategories: "2000", TVCategories: "5000",
		})
		if err != nil {
			t.Fatal(err)
		}
		return row
	}
	if row := update("examplebay"); row.CapsJSON == nil {
		t.Error("caps cleared although the definition is unchanged")
	}
	if row := update("other"); row.CapsJSON != nil {
		t.Error("caps kept after the definition changed")
	}
	if _, err := st.DB.ExecContext(ctx, `UPDATE indexers SET settings = '[1]' WHERE id = ?`, ix.ID); err == nil {
		t.Error("non-object settings accepted")
	}
}

// A site definition has at most one indexer.
func TestOneIndexerPerDefinition(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	params := db.CreateIndexerParams{Name: "a", Definition: "examplebay", Settings: `{}`, MovieCategories: "2000", TVCategories: "5000"}
	if _, err := st.CreateIndexer(ctx, params); err != nil {
		t.Fatal(err)
	}
	params.Name = "b"
	if _, err := st.CreateIndexer(ctx, params); err == nil || !strings.Contains(err.Error(), "indexers.definition") {
		t.Errorf("second indexer for one definition = %v, want unique violation on indexers.definition", err)
	}
}

func TestExposeLAN(t *testing.T) {
	ctx := context.Background()
	st := openTestStore(t)
	created, err := st.CreateAppSettings(ctx, db.CreateAppSettingsParams{APIKey: "fictional-api-key", PasswordHash: "fictional-hash"})
	if err != nil {
		t.Fatal(err)
	}
	if created.ExposeLAN {
		t.Error("expose_lan defaults to on")
	}
	if err := st.SetExposeLAN(ctx, true); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetAppSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !got.ExposeLAN {
		t.Error("expose_lan not saved")
	}
	if _, err := st.DB.ExecContext(ctx, `UPDATE app_settings SET expose_lan = 2`); err == nil {
		t.Error("expose_lan accepted 2")
	}
}
