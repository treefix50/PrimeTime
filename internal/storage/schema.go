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
CREATE INDEX IF NOT EXISTS idx_media_items_modified ON media_items(modified);`

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
