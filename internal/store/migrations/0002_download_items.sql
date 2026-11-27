-- The movies and episodes each download is expected to contain, for every
-- kind of download: a movie, a multi-movie pack, episodes or season packs.
-- Replaces download_episodes. downloads.movie_id / series_id stay as the
-- title a download was grabbed for (its name and art in the queue).
CREATE TABLE download_items (
    download_id INTEGER NOT NULL REFERENCES downloads (id) ON DELETE CASCADE,
    movie_id    INTEGER REFERENCES movies (id) ON DELETE CASCADE,
    episode_id  INTEGER REFERENCES episodes (id) ON DELETE CASCADE,
    CHECK ((movie_id IS NULL) <> (episode_id IS NULL)),
    UNIQUE (download_id, movie_id),
    UNIQUE (download_id, episode_id)
);
CREATE INDEX download_items_movie ON download_items (movie_id);
CREATE INDEX download_items_episode ON download_items (episode_id);

INSERT INTO download_items (download_id, episode_id)
SELECT download_id, episode_id FROM download_episodes;

INSERT INTO download_items (download_id, movie_id)
SELECT id, movie_id FROM downloads WHERE movie_id IS NOT NULL;

DROP TABLE download_episodes;
