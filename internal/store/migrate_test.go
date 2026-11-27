package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"generic-arr-project/internal/testutil"
)

// v1Data is what a database created by the first release may hold: a
// movie download and a season-pack download with its expected episodes.
const v1Data = `
INSERT INTO root_folders (id, media_type, path) VALUES (1, 'movie', '/m'), (2, 'tv', '/t');
INSERT INTO movies (id, tmdb_id, title, root_folder_id) VALUES (10, 900001, 'The Glass Orchard', 1);
INSERT INTO series (id, tmdb_id, title, root_folder_id) VALUES (20, 800001, 'Harbor Watch', 2);
INSERT INTO episodes (id, series_id, season, episode) VALUES (31, 20, 1, 1), (32, 20, 1, 2);
INSERT INTO downloads (id, movie_id, info_hash, release_title, quality, magnet, incomplete_dir, complete_dir)
VALUES (1, 10, 'aaaa', 'The.Glass.Orchard.1999.1080p.BluRay.x264-GRP', 'Bluray-1080p', 'magnet:?a', '/i', '/c');
INSERT INTO downloads (id, series_id, info_hash, release_title, quality, magnet, incomplete_dir, complete_dir)
VALUES (2, 20, 'bbbb', 'Harbor.Watch.S01.1080p.WEB-DL.x264-QUILL', 'WEBDL-1080p', 'magnet:?b', '/i', '/c');
INSERT INTO download_episodes (download_id, episode_id) VALUES (2, 31), (2, 32);
`

// TestUpgradeFromVersion1 opens a database created by the first release
// (only 0001 applied) and checks that later migrations carry its downloads
// over: every download lists the movie or episodes it holds.
func TestUpgradeFromVersion1(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "v1.db")

	conn, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	if len(migrations) < 2 || migrations[0].version != 1 {
		t.Fatalf("migrations = %d, want 0001 and later", len(migrations))
	}
	if _, err := conn.ExecContext(ctx, `CREATE TABLE schema_migrations (
		version    INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
	)`); err != nil {
		t.Fatal(err)
	}
	if err := applyMigration(ctx, conn, migrations[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, v1Data); err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}

	st, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("open v1 database: %v", err)
	}
	t.Cleanup(func() { testutil.Close(t, st) })

	var version int
	if err := st.DB.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_migrations`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if want := migrations[len(migrations)-1].version; version != want {
		t.Errorf("schema version = %d, want %d", version, want)
	}

	movies, err := st.ListDownloadItemMovies(ctx, 1)
	if err != nil || len(movies) != 1 || movies[0].ID != 10 {
		t.Errorf("movie download items = %+v, %v", movies, err)
	}
	eps, err := st.ListDownloadItemEpisodes(ctx, 2)
	if err != nil || len(eps) != 2 || eps[0].ID != 31 || eps[1].ID != 32 {
		t.Errorf("episode download items = %+v, %v", eps, err)
	}
	if rest, _ := st.ListDownloadItemMovies(ctx, 2); len(rest) != 0 {
		t.Errorf("TV download lists movies %+v", rest)
	}
	var tables int
	if err := st.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE name = 'download_episodes'`).Scan(&tables); err != nil || tables != 0 {
		t.Errorf("download_episodes still exists (%d, %v)", tables, err)
	}
}
