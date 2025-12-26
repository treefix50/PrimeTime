package storage

import "fmt"

const schemaMediaItems = `
CREATE TABLE IF NOT EXISTS media_items (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	path TEXT NOT NULL,
	title TEXT,
	size INTEGER,
	modified INTEGER,
	nfo_path TEXT
);`

const schemaNFO = `
CREATE TABLE IF NOT EXISTS nfo (
	media_id INTEGER NOT NULL,
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
	FOREIGN KEY (media_id) REFERENCES media_items(id)
);`

func (s *Store) EnsureSchema() error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}

	statements := []string{
		schemaMediaItems,
		schemaNFO,
	}

	for _, statement := range statements {
		if _, err := s.db.Exec(statement); err != nil {
			return err
		}
	}

	return nil
}
