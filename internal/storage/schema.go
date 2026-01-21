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
			`ALTER TABLE nfo ADD COLUMN actors TEXT;`,
			`ALTER TABLE nfo ADD COLUMN directors TEXT;`,
			`ALTER TABLE nfo ADD COLUMN studios TEXT;`,
			`ALTER TABLE nfo ADD COLUMN runtime INTEGER CHECK (runtime IS NULL OR runtime >= 0);`,
			`ALTER TABLE nfo ADD COLUMN imdb_id TEXT;`,
			`ALTER TABLE nfo ADD COLUMN tmdb_id TEXT;`,
			schemaPlaybackStateIndexes,
		},
	},
	{
		version: 6,
		statements: []string{
			`CREATE TABLE IF NOT EXISTS watched_items (
				media_id TEXT PRIMARY KEY,
				watched_at INTEGER NOT NULL,
				FOREIGN KEY (media_id) REFERENCES media_items(id) ON DELETE CASCADE
			);`,
			`CREATE INDEX IF NOT EXISTS idx_watched_items_watched_at ON watched_items(watched_at DESC);`,
			`CREATE TABLE IF NOT EXISTS favorites (
				media_id TEXT PRIMARY KEY,
				added_at INTEGER NOT NULL,
				FOREIGN KEY (media_id) REFERENCES media_items(id) ON DELETE CASCADE
			);`,
			`CREATE INDEX IF NOT EXISTS idx_favorites_added_at ON favorites(added_at DESC);`,
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
			`ALTER TABLE media_items ADD COLUMN poster_path TEXT;`,
			`CREATE INDEX IF NOT EXISTS idx_media_items_poster_path ON media_items(poster_path);`,
		},
	},
	{
		version: 7,
		statements: []string{
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
			`ALTER TABLE playback_state ADD COLUMN user_id TEXT DEFAULT '';`,
			`CREATE TABLE IF NOT EXISTS watched_items_new (
				media_id TEXT NOT NULL,
				watched_at INTEGER NOT NULL,
				user_id TEXT NOT NULL DEFAULT '',
				PRIMARY KEY (media_id, user_id),
				FOREIGN KEY (media_id) REFERENCES media_items(id) ON DELETE CASCADE
			);`,
			`INSERT INTO watched_items_new (media_id, watched_at, user_id)
			 SELECT media_id, watched_at, '' FROM watched_items;`,
			`DROP TABLE watched_items;`,
			`ALTER TABLE watched_items_new RENAME TO watched_items;`,
			`CREATE TABLE IF NOT EXISTS favorites_new (
				media_id TEXT NOT NULL,
				added_at INTEGER NOT NULL,
				user_id TEXT NOT NULL DEFAULT '',
				PRIMARY KEY (media_id, user_id),
				FOREIGN KEY (media_id) REFERENCES media_items(id) ON DELETE CASCADE
			);`,
			`INSERT INTO favorites_new (media_id, added_at, user_id)
			 SELECT media_id, added_at, '' FROM favorites;`,
			`DROP TABLE favorites;`,
			`ALTER TABLE favorites_new RENAME TO favorites;`,
			`CREATE INDEX IF NOT EXISTS idx_playback_state_user_id ON playback_state(user_id, last_played_at DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_watched_items_user_id ON watched_items(user_id, watched_at DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_watched_items_watched_at ON watched_items(watched_at DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_favorites_user_id ON favorites(user_id, added_at DESC);`,
			`CREATE INDEX IF NOT EXISTS idx_favorites_added_at ON favorites(added_at DESC);`,
		},
	},
	{
		version: 8,
		statements: []string{
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
			`INSERT INTO transcoding_profiles (id, name, video_codec, audio_codec, resolution, max_bitrate, container, created_at)
			VALUES 
				('profile_original', 'original', 'copy', 'copy', NULL, NULL, 'mp4', strftime('%s', 'now')),
				('profile_mobile', 'mobile', 'libx264', 'aac', '720x480', 1000000, 'mp4', strftime('%s', 'now')),
				('profile_mobile_hevc', 'mobile-hevc', 'libx265', 'aac', '720x480', 800000, 'mp4', strftime('%s', 'now')),
				('profile_720p', '720p', 'libx264', 'aac', '1280x720', 2500000, 'mp4', strftime('%s', 'now')),
				('profile_720p_hevc', '720p-hevc', 'libx265', 'aac', '1280x720', 2000000, 'mp4', strftime('%s', 'now')),
				('profile_1080p', '1080p', 'libx264', 'aac', '1920x1080', 5000000, 'mp4', strftime('%s', 'now')),
				('profile_1080p_hevc', '1080p-hevc', 'libx265', 'aac', '1920x1080', 4000000, 'mp4', strftime('%s', 'now')),
				('profile_1080p_hdr', '1080p-hdr', 'libx265', 'aac', '1920x1080', 6000000, 'mp4', strftime('%s', 'now')),
				('profile_4k', '4k', 'libx265', 'aac', '3840x2160', 15000000, 'mp4', strftime('%s', 'now')),
				('profile_4k_hdr', '4k-hdr', 'libx265', 'aac', '3840x2160', 20000000, 'mp4', strftime('%s', 'now')),
				('profile_4k_hdr10plus', '4k-hdr10plus', 'libx265', 'aac', '3840x2160', 25000000, 'mp4', strftime('%s', 'now')),
				('profile_4k_dolby_vision', '4k-dolby-vision', 'libx265', 'eac3', '3840x2160', 30000000, 'mp4', strftime('%s', 'now')),
				('profile_1080p_dolby_atmos', '1080p-dolby-atmos', 'libx265', 'eac3', '1920x1080', 8000000, 'mp4', strftime('%s', 'now')),
				('profile_4k_dolby_atmos', '4k-dolby-atmos', 'libx265', 'eac3', '3840x2160', 25000000, 'mp4', strftime('%s', 'now')),
				('profile_1080p_h264_aac', '1080p-h264-aac', 'libx264', 'aac', '1920x1080', 5000000, 'mp4', strftime('%s', 'now')),
				('profile_720p_h264_aac', '720p-h264-aac', 'libx264', 'aac', '1280x720', 2500000, 'mp4', strftime('%s', 'now'))
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
	{
		version: 10,
		statements: []string{
			`CREATE TABLE IF NOT EXISTS auth_users (
				id TEXT PRIMARY KEY,
				username TEXT NOT NULL UNIQUE,
				password_hash TEXT NOT NULL,
				is_admin INTEGER NOT NULL DEFAULT 0,
				created_at INTEGER NOT NULL,
				last_login INTEGER
			);`,
			`CREATE INDEX IF NOT EXISTS idx_auth_users_username ON auth_users(username);`,
			`CREATE TABLE IF NOT EXISTS auth_sessions (
				token TEXT PRIMARY KEY,
				user_id TEXT NOT NULL,
				username TEXT NOT NULL,
				is_admin INTEGER NOT NULL DEFAULT 0,
				created_at INTEGER NOT NULL,
				expires_at INTEGER NOT NULL,
				FOREIGN KEY (user_id) REFERENCES auth_users(id) ON DELETE CASCADE
			);`,
			`CREATE INDEX IF NOT EXISTS idx_auth_sessions_user_id ON auth_sessions(user_id);`,
			`CREATE INDEX IF NOT EXISTS idx_auth_sessions_expires_at ON auth_sessions(expires_at);`,
		},
	},
	{
		version: 11,
		statements: []string{
			// Optimierte Indizes für bessere Login-Performance
			`CREATE INDEX IF NOT EXISTS idx_auth_sessions_token_expires ON auth_sessions(token, expires_at);`,
			`CREATE INDEX IF NOT EXISTS idx_auth_users_username_lower ON auth_users(LOWER(username));`,
			// NFO-Erweiterungen für vollständige Metadaten
			`ALTER TABLE nfo ADD COLUMN mpaa TEXT;`,
			`ALTER TABLE nfo ADD COLUMN premiered TEXT;`,
			`ALTER TABLE nfo ADD COLUMN release_date TEXT;`,
			`ALTER TABLE nfo ADD COLUMN countries TEXT;`,
			`ALTER TABLE nfo ADD COLUMN trailers TEXT;`,
			`ALTER TABLE nfo ADD COLUMN date_added TEXT;`,
			`ALTER TABLE nfo ADD COLUMN tvdb_id TEXT;`,
			// Neue Tabellen für erweiterte NFO-Daten
			`CREATE TABLE IF NOT EXISTS nfo_actors (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				media_id TEXT NOT NULL,
				name TEXT NOT NULL,
				role TEXT,
				type TEXT,
				tmdb_id TEXT,
				tvdb_id TEXT,
				imdb_id TEXT,
				sort_order INTEGER NOT NULL DEFAULT 0,
				FOREIGN KEY (media_id) REFERENCES media_items(id) ON DELETE CASCADE
			);`,
			`CREATE INDEX IF NOT EXISTS idx_nfo_actors_media_id ON nfo_actors(media_id, sort_order);`,
			`CREATE INDEX IF NOT EXISTS idx_nfo_actors_name ON nfo_actors(name);`,
			`CREATE TABLE IF NOT EXISTS nfo_unique_ids (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				media_id TEXT NOT NULL,
				type TEXT NOT NULL,
				value TEXT NOT NULL,
				FOREIGN KEY (media_id) REFERENCES media_items(id) ON DELETE CASCADE,
				UNIQUE(media_id, type)
			);`,
			`CREATE INDEX IF NOT EXISTS idx_nfo_unique_ids_media_id ON nfo_unique_ids(media_id);`,
			`CREATE INDEX IF NOT EXISTS idx_nfo_unique_ids_type_value ON nfo_unique_ids(type, value);`,
			`CREATE TABLE IF NOT EXISTS nfo_stream_video (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				media_id TEXT NOT NULL,
				codec TEXT,
				bitrate TEXT,
				width TEXT,
				height TEXT,
				aspect TEXT,
				aspect_ratio TEXT,
				frame_rate TEXT,
				scan_type TEXT,
				duration TEXT,
				duration_seconds TEXT,
				stream_index INTEGER NOT NULL DEFAULT 0,
				FOREIGN KEY (media_id) REFERENCES media_items(id) ON DELETE CASCADE
			);`,
			`CREATE INDEX IF NOT EXISTS idx_nfo_stream_video_media_id ON nfo_stream_video(media_id);`,
			`CREATE TABLE IF NOT EXISTS nfo_stream_audio (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				media_id TEXT NOT NULL,
				codec TEXT,
				bitrate TEXT,
				language TEXT,
				channels TEXT,
				sampling_rate TEXT,
				stream_index INTEGER NOT NULL DEFAULT 0,
				FOREIGN KEY (media_id) REFERENCES media_items(id) ON DELETE CASCADE
			);`,
			`CREATE INDEX IF NOT EXISTS idx_nfo_stream_audio_media_id ON nfo_stream_audio(media_id);`,
			`CREATE TABLE IF NOT EXISTS nfo_stream_subtitle (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				media_id TEXT NOT NULL,
				codec TEXT,
				language TEXT,
				stream_index INTEGER NOT NULL DEFAULT 0,
				FOREIGN KEY (media_id) REFERENCES media_items(id) ON DELETE CASCADE
			);`,
			`CREATE INDEX IF NOT EXISTS idx_nfo_stream_subtitle_media_id ON nfo_stream_subtitle(media_id);`,
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
