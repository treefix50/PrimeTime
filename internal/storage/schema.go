package storage

import "fmt"

const schemaMediaItems = `
CREATE TABLE IF NOT EXISTS media_items (
	id TEXT PRIMARY KEY,
	path TEXT NOT NULL,
	title TEXT,
	size INTEGER,
	modified INTEGER,
	nfo_path TEXT
);`

const schemaNFO = `
CREATE TABLE IF NOT EXISTS nfo (
	media_id TEXT NOT NULL PRIMARY KEY,
	type TEXT,
	title TEXT,
	original_title TEXT,
	plot TEXT,
	year INTEGER,
	rating REAL,
	genres TEXT,
	season INTEGER,
	episode INTEGER,
	show_title TEXT,
	raw_root TEXT,
	FOREIGN KEY (media_id) REFERENCES media_items(id) ON DELETE CASCADE
);`

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
	path TEXT NOT NULL,
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
		schemaNFO,
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
