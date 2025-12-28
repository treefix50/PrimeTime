package storage

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db       *sql.DB
	readOnly bool
}

type Options struct {
	BusyTimeout time.Duration
	Synchronous string
	CacheSize   int
	ReadOnly    bool
}

func sqliteDSN(path string, readOnly bool) (string, error) {
	if !readOnly {
		return path, nil
	}
	if path == ":memory:" {
		return "", fmt.Errorf("storage: read-only mode requires a file-backed database")
	}
	dsn := path
	if !strings.HasPrefix(dsn, "file:") {
		dsn = "file:" + dsn
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set("mode", "ro")
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func Open(path string, options Options) (*Store, error) {
	dsn, err := sqliteDSN(path, options.ReadOnly)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		_ = db.Close()
		return nil, err
	}

	if !options.ReadOnly {
		if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
			_ = db.Close()
			return nil, err
		}
		synchronous := options.Synchronous
		if synchronous == "" {
			synchronous = "NORMAL"
		}
		if _, err := db.Exec(fmt.Sprintf("PRAGMA synchronous=%s", synchronous)); err != nil {
			_ = db.Close()
			return nil, err
		}
	}

	busyTimeoutMs := int(options.BusyTimeout / time.Millisecond)
	if _, err := db.Exec(fmt.Sprintf("PRAGMA busy_timeout=%d", busyTimeoutMs)); err != nil {
		_ = db.Close()
		return nil, err
	}

	if _, err := db.Exec("PRAGMA temp_store=MEMORY"); err != nil {
		_ = db.Close()
		return nil, err
	}

	if _, err := db.Exec(fmt.Sprintf("PRAGMA cache_size=%d", options.CacheSize)); err != nil {
		_ = db.Close()
		return nil, err
	}

	if !options.ReadOnly {
		if _, err := db.Exec("PRAGMA journal_size_limit=67108864"); err != nil {
			_ = db.Close()
			return nil, err
		}
	}

	store := &Store{db: db, readOnly: options.ReadOnly}
	if !options.ReadOnly {
		if err := store.MigrateSchema(); err != nil {
			_ = db.Close()
			return nil, err
		}
	}

	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) ReadOnly() bool {
	if s == nil {
		return false
	}
	return s.readOnly
}

func (s *Store) IntegrityCheck() ([]string, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("storage: missing database connection")
	}
	rows, err := s.db.Query("PRAGMA integrity_check")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var result string
		if err := rows.Scan(&result); err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

func (s *Store) Vacuum(target string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}
	if target == "" {
		_, err := s.db.Exec("VACUUM")
		return err
	}
	_, err := s.db.Exec("VACUUM INTO ?", target)
	return err
}

func (s *Store) Analyze() error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}
	_, err := s.db.Exec("ANALYZE")
	return err
}
