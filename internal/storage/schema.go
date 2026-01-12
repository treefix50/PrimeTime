package storage

import "fmt"

const schemaMediaItems = `
CREATE TABLE IF NOT EXISTS media_items (
	id TEXT PRIMARY KEY,
	path TEXT NOT NULL UNIQUE,
	title TEXT,
	size INTEGER,
	modified INTEGER,
	nfo_path TEXT
);`

const schemaMediaItemsIndexes = `
CREATE INDEX IF NOT EXISTS idx_media_items_title ON media_items(title);
CREATE INDEX IF NOT EXISTS idx_media_items_path ON media_items(path);
CREATE INDEX IF NOT EXISTS idx_media_items_modified ON media_items(modified);
`

const schemaNFO = `
CREATE TABLE IF NOT EXISTS nfo (
	media_id TEXT NOT NULL PRIMARY KEY,
	type TEXT,
	title TEXT,
	original_title TEXT,
	plot TEXT,
	year INTEGER CHECK (year IS NULL OR (year >= 1800 AND year <= 3000)),
	rating REAL CHECK (rating IS NULL OR (rating >= 0 AND rating <= 10)),
	genres TEXT,
	season INTEGER CHECK (season IS NULL OR season >= 0),
	episode INTEGER CHECK (episode IS NULL OR episode >= 0),
	show_title TEXT,
	raw_root TEXT,
	FOREIGN KEY (media_id) REFERENCES media_items(id) ON DELETE CASCADE
);`

const schemaNFOIndexes = `
CREATE INDEX IF NOT EXISTS idx_nfo_show_title ON nfo(show_title);
CREATE INDEX IF NOT EXISTS idx_nfo_type ON nfo(type);`

const schemaNFOQueryIndexes = `
CREATE INDEX IF NOT EXISTS idx_nfo_year ON nfo(year);
CREATE INDEX IF NOT EXISTS idx_nfo_genres ON nfo(genres);
CREATE INDEX IF NOT EXISTS idx_nfo_title ON nfo(title);`

const schemaPlaybackState = `
CREATE TABLE IF NOT EXISTS playback_state (
	media_id TEXT NOT NULL,
	position_seconds INTEGER NOT NULL,
	duration_seconds INTEGER NOT NULL,
	updated_at INTEGER NOT NULL,
	client_id TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (media_id, client_id),
	FOREIGN KEY (media_id) REFERENCES media_items(id) ON DELETE CASCADE
);`

const schemaPlaybackStateIndexes = `
CREATE INDEX IF NOT EXISTS idx_playback_state_client_id ON playback_state(client_id, last_played_at DESC);
CREATE INDEX IF NOT EXISTS idx_playback_state_last_played ON playback_state(last_played_at DESC);`

const schemaLibraryRoots = `
CREATE TABLE IF NOT EXISTS library_roots (
	id TEXT PRIMARY KEY,
	path TEXT NOT NULL UNIQUE,
	type TEXT NOT NULL,
	created_at INTEGER NOT NULL
);`

const schemaLibraryRootsIndexes = `
CREATE INDEX IF NOT EXISTS idx_library_roots_path ON library_roots(path);`

const schemaScanRuns = `
CREATE TABLE IF NOT EXISTS scan_runs (
	id TEXT PRIMARY KEY,
	root_id TEXT NOT NULL,
	started_at INTEGER NOT NULL,
	finished_at INTEGER,
	status TEXT NOT NULL,
	error TEXT,
	FOREIGN KEY (root_id) REFERENCES library_roots(id) ON DELETE CASCADE
);`

const schemaMigrations = `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version INTEGER PRIMARY KEY
);`

type migration struct {
	version    int
	statements []string
}

var migrations = []migration{
	{
		version: 1,
		statements: []string{
			schemaMediaItems,
			schemaMediaItemsIndexes,
			schemaNFO,
			schemaNFOIndexes,
			schemaPlaybackState,
			schemaLibraryRoots,
			schemaScanRuns,
		},
	},
	{
		version: 2,
		statements: []string{
			schemaNFOQueryIndexes,
			schemaLibraryRootsIndexes,
		},
	},
	{
		version: 3,
		statements: []string{
			`ALTER TABLE playback_state ADD COLUMN last_played_at INTEGER NOT NULL DEFAULT 0;`,
			`ALTER TABLE playback_state ADD COLUMN percent_complete REAL;`,
			`UPDATE playback_state SET last_played_at = updated_at WHERE last_played_at = 0;`,
		},
	},
	{
		version: 4,
		statements: []string{
			`ALTER TABLE media_items ADD COLUMN stable_key TEXT;`,
			`UPDATE media_items SET stable_key = id WHERE stable_key IS NULL;`,
			`CREATE INDEX IF NOT EXISTS idx_media_items_stable_key ON media_items(stable_key);`,
		},
	},
	{
		version: 5,
		statements: []string{
			// Verbesserung 1: Erweiterte NFO-Felder
			`ALTER TABLE nfo ADD COLUMN actors TEXT;`,
			`ALTER TABLE nfo ADD COLUMN directors TEXT;`,
			`ALTER TABLE nfo ADD COLUMN studios TEXT;`,
			`ALTER TABLE nfo ADD COLUMN runtime INTEGER CHECK (runtime IS NULL OR runtime >= 0);`,
			`ALTER TABLE nfo ADD COLUMN imdb_id TEXT;`,
			`ALTER TABLE nfo ADD COLUMN tmdb_id TEXT;`,
			// Verbesserung 2: Playback-State-Indizes für Batch-Operations
			schemaPlaybackStateIndexes,
		},
	},
	{
		version: 6,
		statements: []string{
			// Erweiterung 1: Watched/Unwatched Status
			`CREATE TABLE IF NOT EXISTS watched_items (
				media_id TEXT PRIMARY KEY,
				watched_at INTEGER NOT NULL,
				FOREIGN KEY (media_id) REFERENCES media_items(id) ON DELETE CASCADE
			);`,
			`CREATE INDEX IF NOT EXISTS idx_watched_items_watched_at ON watched_items(watched_at DESC);`,

			// Erweiterung 2: Favorites/Bookmarks
			`CREATE TABLE IF NOT EXISTS favorites (
				media_id TEXT PRIMARY KEY,
				added_at INTEGER NOT NULL,
				FOREIGN KEY (media_id) REFERENCES media_items(id) ON DELETE CASCADE
			);`,
			`CREATE INDEX IF NOT EXISTS idx_favorites_added_at ON favorites(added_at DESC);`,

			// Erweiterung 4: Collections/Playlists
			`CREATE TABLE IF NOT EXISTS collections (
				id TEXT PRIMARY KEY,
				name TEXT NOT NULL,
				description TEXT,
				created_at INTEGER NOT NULL,
				updated_at INTEGER NOT NULL
			);`,
			`CREATE INDEX IF NOT EXISTS idx_collections_name ON collections(name);`,
			`CREATE INDEX IF NOT EXISTS idx_collections_created_at ON collections(created_at DESC);`,

			`CREATE TABLE IF NOT EXISTS collection_items (
				collection_id TEXT NOT NULL,
				media_id TEXT NOT NULL,
				position INTEGER NOT NULL DEFAULT 0,
				added_at INTEGER NOT NULL,
				PRIMARY KEY (collection_id, media_id),
				FOREIGN KEY (collection_id) REFERENCES collections(id) ON DELETE CASCADE,
				FOREIGN KEY (media_id) REFERENCES media_items(id) ON DELETE CASCADE
			);`,
			`CREATE INDEX IF NOT EXISTS idx_collection_items_collection_id ON collection_items(collection_id, position);`,
			`CREATE INDEX IF NOT EXISTS idx_collection_items_media_id ON collection_items(media_id);`,

			// Erweiterung 5: Poster/Thumbnail Support
			`ALTER TABLE media_items ADD COLUMN poster_path TEXT;`,
			`CREATE INDEX IF NOT EXISTS idx_media_items_poster_path ON media_items(poster_path);`,
		},
	},
	{
		version: 7,
		statements: []string{
			// Verbesserung 1: Multi-User-Support
			`CREATE TABLE IF NOT EXISTS users (
				id TEXT PRIMARY KEY,
				name TEXT NOT NULL UNIQUE,
				created_at INTEGER NOT NULL,
				last_active INTEGER NOT NULL DEFAULT 0
			);`,
			`CREATE INDEX IF NOT EXISTS idx_users_name ON users(name);`,
			`CREATE INDEX IF NOT EXISTS idx_users_last_active ON users(last_active DESC);`,

			`CREATE TABLE IF NOT EXISTS user_preferences (
				user_id TEXT NOT NULL,
				key TEXT NOT NULL,
				value TEXT,
				PRIMARY KEY (user_id, key),
				FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
			);`,
			`CREATE INDEX IF NOT EXISTS idx_user_preferences_user_id ON user_preferences(user_id);`,

			// Erweitere bestehende Tabellen um user_id
			`ALTER TABLE playback_state ADD COLUMN user_id TEXT DEFAULT '';`,
			`ALTER TABLE watched_items ADD COLUMN user_id TEXT DEFAULT '';`,
			`ALTER TABLE favorites ADD COLUMN user_id TEXT DEFAULT '';`,

			// Erstelle neue Indizes für user_id
			`CREATE INDEX IF NOT EXISTS idx_playback_state_user_id ON playback_state(user_id, last_played_at DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_watched_items_user_id ON watched_items(user_id, watched_at DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_favorites_user_id ON favorites(user_id, added_at DESC);`,
		},
	},
	{
		version: 8,
		statements: []string{
			// Verbesserung 2: Transkodierungs-Profile
			`CREATE TABLE IF NOT EXISTS transcoding_profiles (
				id TEXT PRIMARY KEY,
				name TEXT NOT NULL UNIQUE,
				video_codec TEXT NOT NULL,
				audio_codec TEXT NOT NULL,
				resolution TEXT,
				max_bitrate INTEGER,
				container TEXT NOT NULL DEFAULT 'mp4',
				created_at INTEGER NOT NULL
			);`,
			`CREATE INDEX IF NOT EXISTS idx_transcoding_profiles_name ON transcoding_profiles(name);`,

			// Standard-Profile einfügen
			`INSERT INTO transcoding_profiles (id, name, video_codec, audio_codec, resolution, max_bitrate, container, created_at)
			VALUES 
				('profile_original', 'original', 'copy', 'copy', NULL, NULL, 'mp4', strftime('%s', 'now')),
				('profile_mobile', 'mobile', 'libx264', 'aac', '720x480', 1000000, 'mp4', strftime('%s', 'now')),
				('profile_720p', '720p', 'libx264', 'aac', '1280x720', 2500000, 'mp4', strftime('%s', 'now')),
				('profile_1080p', '1080p', 'libx264', 'aac', '1920x1080', 5000000, 'mp4', strftime('%s', 'now'))
			ON CONFLICT(id) DO NOTHING;`,

			`CREATE TABLE IF NOT EXISTS transcoding_cache (
				id TEXT PRIMARY KEY,
				media_id TEXT NOT NULL,
				profile_id TEXT NOT NULL,
				cache_path TEXT NOT NULL,
				created_at INTEGER NOT NULL,
				last_accessed INTEGER NOT NULL,
				size_bytes INTEGER NOT NULL DEFAULT 0,
				FOREIGN KEY (media_id) REFERENCES media_items(id) ON DELETE CASCADE,
				FOREIGN KEY (profile_id) REFERENCES transcoding_profiles(id) ON DELETE CASCADE
			);`,
			`CREATE INDEX IF NOT EXISTS idx_transcoding_cache_media_profile ON transcoding_cache(media_id, profile_id);`,
			`CREATE INDEX IF NOT EXISTS idx_transcoding_cache_last_accessed ON transcoding_cache(last_accessed);`,
		},
	},
	{
		version: 9,
		statements: []string{
			// Verbesserung 3: Serien-Verwaltung (TV Shows)
			`CREATE TABLE IF NOT EXISTS tv_shows (
				id TEXT PRIMARY KEY,
				title TEXT NOT NULL,
				original_title TEXT,
				plot TEXT,
				poster_path TEXT,
				year INTEGER,
				genres TEXT,
				created_at INTEGER NOT NULL,
				updated_at INTEGER NOT NULL
			);`,
			`CREATE INDEX IF NOT EXISTS idx_tv_shows_title ON tv_shows(title);`,
			`CREATE INDEX IF NOT EXISTS idx_tv_shows_year ON tv_shows(year);`,

			`CREATE TABLE IF NOT EXISTS seasons (
				id TEXT PRIMARY KEY,
				show_id TEXT NOT NULL,
				season_number INTEGER NOT NULL,
				title TEXT,
				plot TEXT,
				poster_path TEXT,
				episode_count INTEGER NOT NULL DEFAULT 0,
				created_at INTEGER NOT NULL,
				FOREIGN KEY (show_id) REFERENCES tv_shows(id) ON DELETE CASCADE,
				UNIQUE(show_id, season_number)
			);`,
			`CREATE INDEX IF NOT EXISTS idx_seasons_show_id ON seasons(show_id, season_number);`,

			`CREATE TABLE IF NOT EXISTS episodes (
				id TEXT PRIMARY KEY,
				season_id TEXT NOT NULL,
				episode_number INTEGER NOT NULL,
				media_id TEXT NOT NULL UNIQUE,
				title TEXT,
				plot TEXT,
				air_date TEXT,
				created_at INTEGER NOT NULL,
				FOREIGN KEY (season_id) REFERENCES seasons(id) ON DELETE CASCADE,
				FOREIGN KEY (media_id) REFERENCES media_items(id) ON DELETE CASCADE,
				UNIQUE(season_id, episode_number)
			);`,
			`CREATE INDEX IF NOT EXISTS idx_episodes_season_id ON episodes(season_id, episode_number);`,
			`CREATE INDEX IF NOT EXISTS idx_episodes_media_id ON episodes(media_id);`,
		},
	},
}

func (s *Store) EnsureSchema() error {
	return s.MigrateSchema()
}

func (s *Store) MigrateSchema() error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}

	if _, err := s.db.Exec(schemaMigrations); err != nil {
		return fmt.Errorf("storage: create schema_migrations table: %w", err)
	}

	current, err := s.currentSchemaVersion()
	if err != nil {
		return err
	}

	for _, migration := range migrations {
		if migration.version <= current {
			continue
		}
		if err := s.applyMigration(migration); err != nil {
			return err
		}
		current = migration.version
	}

	return nil
}

func (s *Store) currentSchemaVersion() (int, error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("storage: missing database connection")
	}

	var version int
	if err := s.db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&version); err != nil {
		return 0, fmt.Errorf("storage: read schema version: %w", err)
	}
	return version, nil
}

func (s *Store) applyMigration(migration migration) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("storage: start migration %d: %w", migration.version, err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	for _, statement := range migration.statements {
		if _, err = tx.Exec(statement); err != nil {
			return fmt.Errorf("storage: migration %d failed: %w", migration.version, err)
		}
	}

	if _, err = tx.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, migration.version); err != nil {
		return fmt.Errorf("storage: record migration %d: %w", migration.version, err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("storage: commit migration %d: %w", migration.version, err)
	}
	return nil
}
