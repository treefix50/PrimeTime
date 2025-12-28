package storage

import (
	"database/sql"
	"testing"
	"time"

	"github.com/treefix50/primetime/internal/server"
)

func newTestStore(t *testing.T, ensureSchema bool) *Store {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	store := &Store{db: db}
	if ensureSchema {
		if err := store.EnsureSchema(); err != nil {
			t.Fatalf("ensure schema: %v", err)
		}
	}

	return store
}

func TestEnsureSchema(t *testing.T) {
	store := newTestStore(t, false)

	if err := store.MigrateSchema(); err != nil {
		t.Fatalf("MigrateSchema() error = %v", err)
	}

	rows, err := store.db.Query(`
		SELECT name
		FROM sqlite_master
		WHERE type = 'table'
	`)
	if err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}
	defer rows.Close()

	found := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan sqlite_master: %v", err)
		}
		found[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("sqlite_master rows: %v", err)
	}

	for _, table := range []string{"schema_migrations", "media_items", "nfo", "playback_state", "library_roots", "scan_runs"} {
		if !found[table] {
			t.Fatalf("expected table %q to exist", table)
		}
	}

	var version int
	if err := store.db.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&version); err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	if version != 1 {
		t.Fatalf("unexpected schema version: got %d want 1", version)
	}
}

func TestSaveItems(t *testing.T) {
	store := newTestStore(t, true)

	modified := time.Unix(1700000000, 0)
	items := []server.MediaItem{
		{
			ID:        "item-1",
			Title:     "First",
			VideoPath: "/tmp/first.mkv",
			NFOPath:   "/tmp/first.nfo",
			Size:      100,
			Modified:  modified,
		},
		{
			ID:        "item-2",
			Title:     "Second",
			VideoPath: "/tmp/second.mkv",
			NFOPath:   "",
			Size:      200,
			Modified:  modified.Add(10 * time.Second),
		},
	}

	if err := store.SaveItems(items); err != nil {
		t.Fatalf("SaveItems() error = %v", err)
	}

	var (
		id      string
		path    string
		title   sql.NullString
		size    int64
		modTime int64
		nfoPath sql.NullString
	)
	err := store.db.QueryRow(`
		SELECT id, path, title, size, modified, nfo_path
		FROM media_items
		WHERE id = ?
	`, "item-2").Scan(&id, &path, &title, &size, &modTime, &nfoPath)
	if err != nil {
		t.Fatalf("query saved item: %v", err)
	}

	if id != "item-2" || path != "/tmp/second.mkv" || title.String != "Second" || size != 200 {
		t.Fatalf("unexpected stored values: id=%q path=%q title=%q size=%d", id, path, title.String, size)
	}
	if modTime != items[1].Modified.Unix() {
		t.Fatalf("unexpected modified value: got %d want %d", modTime, items[1].Modified.Unix())
	}
	if nfoPath.Valid {
		t.Fatalf("expected nfo_path to be NULL, got %q", nfoPath.String)
	}

	items[0].Title = "Updated"
	items[0].Size = 555
	if err := store.SaveItems(items[:1]); err != nil {
		t.Fatalf("SaveItems() update error = %v", err)
	}

	var updatedTitle sql.NullString
	var updatedSize int64
	err = store.db.QueryRow(`
		SELECT title, size
		FROM media_items
		WHERE id = ?
	`, "item-1").Scan(&updatedTitle, &updatedSize)
	if err != nil {
		t.Fatalf("query updated item: %v", err)
	}
	if updatedTitle.String != "Updated" || updatedSize != 555 {
		t.Fatalf("unexpected updated values: title=%q size=%d", updatedTitle.String, updatedSize)
	}
}

func TestGetAll(t *testing.T) {
	store := newTestStore(t, true)

	items := []server.MediaItem{
		{
			ID:        "beta",
			Title:     "Beta",
			VideoPath: "/tmp/beta.mkv",
			NFOPath:   "",
			Size:      10,
			Modified:  time.Unix(1700000100, 0),
		},
		{
			ID:        "alpha",
			Title:     "Alpha",
			VideoPath: "/tmp/alpha.mkv",
			NFOPath:   "/tmp/alpha.nfo",
			Size:      20,
			Modified:  time.Unix(1700000200, 0),
		},
	}

	if err := store.SaveItems(items); err != nil {
		t.Fatalf("SaveItems() error = %v", err)
	}

	got, err := store.GetAll()
	if err != nil {
		t.Fatalf("GetAll() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("GetAll() len = %d, want 2", len(got))
	}

	if got[0].Title != "Alpha" || got[1].Title != "Beta" {
		t.Fatalf("GetAll() order = %q, %q; want Alpha, Beta", got[0].Title, got[1].Title)
	}

	expect := map[string]server.MediaItem{}
	for _, item := range items {
		expect[item.ID] = item
	}

	for _, item := range got {
		want, ok := expect[item.ID]
		if !ok {
			t.Fatalf("unexpected item id: %s", item.ID)
		}
		if item.Title != want.Title || item.VideoPath != want.VideoPath || item.NFOPath != want.NFOPath || item.Size != want.Size {
			t.Fatalf("unexpected item fields for %s", item.ID)
		}
		if item.Modified.Unix() != want.Modified.Unix() {
			t.Fatalf("unexpected modified for %s: got %d want %d", item.ID, item.Modified.Unix(), want.Modified.Unix())
		}
	}
}

func TestDeleteItems(t *testing.T) {
	store := newTestStore(t, true)

	items := []server.MediaItem{
		{
			ID:        "item-1",
			Title:     "First",
			VideoPath: "/tmp/first.mkv",
			Size:      100,
			Modified:  time.Unix(1700000000, 0),
		},
		{
			ID:        "item-2",
			Title:     "Second",
			VideoPath: "/tmp/second.mkv",
			Size:      200,
			Modified:  time.Unix(1700000100, 0),
		},
	}

	if err := store.SaveItems(items); err != nil {
		t.Fatalf("SaveItems() error = %v", err)
	}

	if err := store.DeleteItems([]string{"item-1"}); err != nil {
		t.Fatalf("DeleteItems() error = %v", err)
	}

	_, ok, err := store.GetByID("item-1")
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if ok {
		t.Fatalf("expected item-1 to be deleted")
	}

	_, ok, err = store.GetByID("item-2")
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if !ok {
		t.Fatalf("expected item-2 to remain")
	}
}
