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

func (s *Store) EnsureSchema() error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}

	statements := []string{
		schemaMediaItems,
		schemaMediaItemsIndexes,
		schemaNFO,
		schemaNFOIndexes,
		schemaPlaybackState,
		schemaLibraryRoots,
		schemaScanRuns,
	}

	for _, statement := range statements {
		if _, err := s.db.Exec(statement); err != nil {
			return err
		}
	}

	return nil
}
