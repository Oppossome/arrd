-- arrd schema. Nothing has been released yet, so this file is edited in
-- place; once a release ships, changes must go in new numbered files.

-- Application-wide settings and secrets: exactly one row, created by
-- onboarding (POST /api/v1/setup).
CREATE TABLE app_settings (
    id                 INTEGER PRIMARY KEY CHECK (id = 1),
    setup_completed_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    api_key            TEXT    NOT NULL,
    tmdb_api_key       TEXT    NOT NULL DEFAULT '',
    password_hash      TEXT    NOT NULL, -- argon2id PHC string for the admin login
    -- Serve the web UI on all interfaces and announce it over mDNS.
    expose_lan         BOOLEAN NOT NULL DEFAULT 0 CHECK (expose_lan IN (0, 1))
);

-- The VPN, for any provider. settings is a JSON object of the provider's
-- field values, secrets included (a WireGuard private key, a provider
-- password); the API never returns secret values. At most one row: one VPN
-- at a time. When it exists, all indexer and download traffic goes through
-- it (and stops while the tunnel is down); otherwise traffic goes direct.
CREATE TABLE vpn_settings (
    id       INTEGER PRIMARY KEY CHECK (id = 1),
    provider TEXT    NOT NULL,
    settings TEXT    NOT NULL CHECK (json_valid(settings) AND json_type(settings) = 'object')
);

-- Download storage settings: exactly one row.
CREATE TABLE download_settings (
    id             INTEGER PRIMARY KEY CHECK (id = 1),
    incomplete_dir TEXT    NOT NULL,
    complete_dir   TEXT    NOT NULL,
    import_mode    TEXT    NOT NULL DEFAULT 'hardlink' CHECK (import_mode IN ('hardlink', 'copy', 'move'))
);

-- Library root folders per media type.
CREATE TABLE root_folders (
    id         INTEGER PRIMARY KEY,
    media_type TEXT    NOT NULL CHECK (media_type IN ('movie', 'tv')),
    path       TEXT    NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT 0 CHECK (is_default IN (0, 1)),
    UNIQUE (media_type, path)
);
-- At most one default root per media type.
CREATE UNIQUE INDEX root_folders_one_default ON root_folders (media_type) WHERE is_default = 1;

-- Indexers are sites arrd searches itself, each described by a Cardigann
-- definition (see internal/indexer): the user's settings for one site,
-- switched on and off from the definitions list, so a definition has at
-- most one indexer. settings is a JSON object of the definition's field
-- values, secrets included; the API never returns secret values.
CREATE TABLE indexers (
    id               INTEGER PRIMARY KEY,
    name             TEXT    NOT NULL UNIQUE,
    definition       TEXT    NOT NULL UNIQUE CHECK (definition <> ''),  -- definition id, e.g. "examplebay"
    settings         TEXT    NOT NULL DEFAULT '{}' CHECK (json_valid(settings) AND json_type(settings) = 'object'),
    movie_categories TEXT    NOT NULL DEFAULT '2000',  -- comma-separated Newznab category IDs
    tv_categories    TEXT    NOT NULL DEFAULT '5000',
    enabled          BOOLEAN NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    enable_rss       BOOLEAN NOT NULL DEFAULT 1 CHECK (enable_rss IN (0, 1)),
    enable_search    BOOLEAN NOT NULL DEFAULT 1 CHECK (enable_search IN (0, 1)),
    priority         INTEGER NOT NULL DEFAULT 25,
    min_seeders      INTEGER NOT NULL DEFAULT 1 CHECK (min_seeders >= 0),
    caps_json        TEXT,              -- cached capabilities, NULL until tested
    last_error       TEXT,
    last_error_at    TEXT
);

-- Global quality limits used when choosing a release: exactly one row.
CREATE TABLE quality_settings (
    id              INTEGER PRIMARY KEY CHECK (id = 1),
    min_resolution  INTEGER NOT NULL CHECK (min_resolution IN (480, 720, 1080, 2160)),
    max_resolution  INTEGER NOT NULL CHECK (max_resolution IN (480, 720, 1080, 2160)),
    max_size_mb     INTEGER NOT NULL CHECK (max_size_mb >= 0), -- 0 = unlimited
    CHECK (min_resolution <= max_resolution)
);
INSERT INTO quality_settings (id, min_resolution, max_resolution, max_size_mb) VALUES (1, 720, 1080, 0);

CREATE TABLE movies (
    id             INTEGER PRIMARY KEY,
    tmdb_id        INTEGER NOT NULL UNIQUE,
    imdb_id        TEXT,
    title          TEXT    NOT NULL,
    original_title TEXT    NOT NULL DEFAULT '',
    year           INTEGER NOT NULL DEFAULT 0, -- 0 = unknown
    overview       TEXT    NOT NULL DEFAULT '',
    poster_path    TEXT,              -- TMDB image file, e.g. "/abc.jpg"
    backdrop_path  TEXT    NOT NULL DEFAULT '', -- TMDB wide art, for active-download cards
    root_folder_id INTEGER NOT NULL REFERENCES root_folders (id) ON DELETE RESTRICT,
    monitored      BOOLEAN NOT NULL DEFAULT 1 CHECK (monitored IN (0, 1)),
    file_path      TEXT,               -- imported file, NULL until imported
    file_quality   TEXT,
    added_at       TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE TABLE series (
    id                INTEGER PRIMARY KEY,
    tmdb_id           INTEGER NOT NULL UNIQUE,
    tvdb_id           INTEGER,
    imdb_id           TEXT,
    title             TEXT    NOT NULL,
    original_title    TEXT    NOT NULL DEFAULT '',
    year              INTEGER NOT NULL DEFAULT 0, -- first air year; 0 = unknown
    overview          TEXT    NOT NULL DEFAULT '',
    status            TEXT    NOT NULL DEFAULT '', -- TMDB status, e.g. "Returning Series"
    poster_path       TEXT,
    backdrop_path     TEXT    NOT NULL DEFAULT '',
    root_folder_id    INTEGER NOT NULL REFERENCES root_folders (id) ON DELETE RESTRICT,
    monitored         BOOLEAN NOT NULL DEFAULT 1 CHECK (monitored IN (0, 1)),
    added_at          TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    last_refreshed_at TEXT
);

CREATE TABLE episodes (
    id           INTEGER PRIMARY KEY,
    series_id    INTEGER NOT NULL REFERENCES series (id) ON DELETE CASCADE,
    season       INTEGER NOT NULL CHECK (season >= 0), -- 0 = specials
    episode      INTEGER NOT NULL CHECK (episode >= 0),
    title        TEXT    NOT NULL DEFAULT '',
    air_date     TEXT,              -- YYYY-MM-DD; NULL if unknown
    monitored    BOOLEAN NOT NULL DEFAULT 1 CHECK (monitored IN (0, 1)),
    file_path    TEXT,
    file_quality TEXT,
    UNIQUE (series_id, season, episode)
);
CREATE INDEX episodes_wanted ON episodes (monitored, file_path, air_date);

-- Torrents grabbed by arrd. Paths are captured at grab time so changing
-- download settings only affects new downloads.
CREATE TABLE downloads (
    id             INTEGER PRIMARY KEY,
    movie_id       INTEGER REFERENCES movies (id) ON DELETE SET NULL,
    series_id      INTEGER REFERENCES series (id) ON DELETE SET NULL,
    indexer_id     INTEGER REFERENCES indexers (id) ON DELETE SET NULL,
    info_hash      TEXT    NOT NULL UNIQUE,
    release_title  TEXT    NOT NULL,
    quality        TEXT    NOT NULL,
    size           INTEGER NOT NULL DEFAULT 0,
    magnet         TEXT,
    metainfo       BLOB,               -- .torrent bytes; filled once metadata is known
    incomplete_dir TEXT    NOT NULL,
    complete_dir   TEXT    NOT NULL,
    status         TEXT    NOT NULL DEFAULT 'downloading'
                   CHECK (status IN ('downloading', 'completed', 'imported', 'failed')),
    -- Separate from status: only a 'downloading' row is ever paused, and
    -- resuming returns it to where it was.
    paused         BOOLEAN NOT NULL DEFAULT 0 CHECK (paused IN (0, 1)),
    error          TEXT,
    added_at       TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    completed_at   TEXT,
    imported_at    TEXT,
    CHECK (magnet IS NOT NULL OR metainfo IS NOT NULL)
);
CREATE INDEX downloads_movie ON downloads (movie_id);
CREATE INDEX downloads_status ON downloads (status);
CREATE INDEX downloads_series ON downloads (series_id);

-- Episodes a TV download is expected to contain.
CREATE TABLE download_episodes (
    download_id INTEGER NOT NULL REFERENCES downloads (id) ON DELETE CASCADE,
    episode_id  INTEGER NOT NULL REFERENCES episodes (id) ON DELETE CASCADE,
    PRIMARY KEY (download_id, episode_id)
);
CREATE INDEX download_episodes_episode ON download_episodes (episode_id);

-- Browser login sessions. Only a SHA-256 hash of the token is stored.
CREATE TABLE sessions (
    id_hash      TEXT PRIMARY KEY,
    created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    expires_at   TEXT NOT NULL,
    last_seen_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
CREATE INDEX sessions_expiry ON sessions (expires_at);
