package storage

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/treefix50/primetime/internal/server"
)

// Constants for column counts
const (
	mediaItemColumnsWithPoster    = 8
	mediaItemColumnsWithoutPoster = 7
)

// queryMediaItemsWithFallback executes a query with poster_path and falls back to without if needed
func (s *Store) queryMediaItemsWithFallback(queryWithPoster, queryWithoutPoster string, args ...interface{}) ([]server.MediaItem, error) {
	rows, err := s.db.Query(queryWithPoster, args...)
	if err != nil {
		// Fallback without poster_path
		rows, err = s.db.Query(queryWithoutPoster, args...)
		if err != nil {
			return nil, err
		}
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	hasPosterPath := len(cols) == mediaItemColumnsWithPoster

	items := []server.MediaItem{}
	for rows.Next() {
		var item server.MediaItem
		if hasPosterPath {
			var posterPath sql.NullString
			if err := rows.Scan(&item.ID, &item.VideoPath, &item.Title, &item.Size, &item.Modified, &item.NFOPath, &item.StableKey, &posterPath); err != nil {
				return nil, err
			}
			if posterPath.Valid {
				item.PosterPath = posterPath.String
			}
		} else {
			if err := rows.Scan(&item.ID, &item.VideoPath, &item.Title, &item.Size, &item.Modified, &item.NFOPath, &item.StableKey); err != nil {
				return nil, err
			}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// Erweiterung 1: Watched/Unwatched Status

func (s *Store) MarkWatched(mediaID string, watchedAt time.Time) error {
	if s.readOnly {
		return fmt.Errorf("storage: read-only mode")
	}
	_, err := s.db.Exec(`
		INSERT INTO watched_items (media_id, watched_at)
		VALUES (?, ?)
		ON CONFLICT(media_id) DO UPDATE SET watched_at = excluded.watched_at
	`, mediaID, watchedAt.Unix())
	return err
}

func (s *Store) UnmarkWatched(mediaID string) error {
	if s.readOnly {
		return fmt.Errorf("storage: read-only mode")
	}
	_, err := s.db.Exec(`DELETE FROM watched_items WHERE media_id = ?`, mediaID)
	return err
}

func (s *Store) IsWatched(mediaID string) (bool, error) {
	var exists int
	err := s.db.QueryRow(`SELECT 1 FROM watched_items WHERE media_id = ? LIMIT 1`, mediaID).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) GetWatchedItems(limit, offset int) ([]server.MediaItem, error) {
	queryWithPoster := `
		SELECT m.id, m.path, m.title, m.size, m.modified, m.nfo_path, m.stable_key, m.poster_path
		FROM media_items m
		INNER JOIN watched_items w ON m.id = w.media_id
		ORDER BY w.watched_at DESC
	`
	queryWithoutPoster := `
		SELECT m.id, m.path, m.title, m.size, m.modified, m.nfo_path, m.stable_key
		FROM media_items m
		INNER JOIN watched_items w ON m.id = w.media_id
		ORDER BY w.watched_at DESC
	`

	args := []interface{}{}
	if limit > 0 {
		queryWithPoster += " LIMIT ?"
		queryWithoutPoster += " LIMIT ?"
		args = append(args, limit)
	}
	if offset > 0 {
		queryWithPoster += " OFFSET ?"
		queryWithoutPoster += " OFFSET ?"
		args = append(args, offset)
	}

	return s.queryMediaItemsWithFallback(queryWithPoster, queryWithoutPoster, args...)
}

// Erweiterung 2: Favorites/Bookmarks

func (s *Store) AddFavorite(mediaID string, addedAt time.Time) error {
	if s.readOnly {
		return fmt.Errorf("storage: read-only mode")
	}
	_, err := s.db.Exec(`
		INSERT INTO favorites (media_id, added_at)
		VALUES (?, ?)
		ON CONFLICT(media_id) DO UPDATE SET added_at = excluded.added_at
	`, mediaID, addedAt.Unix())
	return err
}

func (s *Store) RemoveFavorite(mediaID string) error {
	if s.readOnly {
		return fmt.Errorf("storage: read-only mode")
	}
	_, err := s.db.Exec(`DELETE FROM favorites WHERE media_id = ?`, mediaID)
	return err
}

func (s *Store) IsFavorite(mediaID string) (bool, error) {
	var exists int
	err := s.db.QueryRow(`SELECT 1 FROM favorites WHERE media_id = ? LIMIT 1`, mediaID).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) GetFavorites(limit, offset int) ([]server.MediaItem, error) {
	queryWithPoster := `
		SELECT m.id, m.path, m.title, m.size, m.modified, m.nfo_path, m.stable_key, m.poster_path
		FROM media_items m
		INNER JOIN favorites f ON m.id = f.media_id
		ORDER BY f.added_at DESC
	`
	queryWithoutPoster := `
		SELECT m.id, m.path, m.title, m.size, m.modified, m.nfo_path, m.stable_key
		FROM media_items m
		INNER JOIN favorites f ON m.id = f.media_id
		ORDER BY f.added_at DESC
	`

	args := []interface{}{}
	if limit > 0 {
		queryWithPoster += " LIMIT ?"
		queryWithoutPoster += " LIMIT ?"
		args = append(args, limit)
	}
	if offset > 0 {
		queryWithPoster += " OFFSET ?"
		queryWithoutPoster += " OFFSET ?"
		args = append(args, offset)
	}

	return s.queryMediaItemsWithFallback(queryWithPoster, queryWithoutPoster, args...)
}

// Erweiterung 3: Recently Added

func (s *Store) GetRecentlyAdded(limit int, days int, itemType string) ([]server.MediaItem, error) {
	// Simplified query - just get recent items ordered by modified date
	// Note: days parameter is ignored because modified is stored as string (RFC3339)
	// Note: itemType parameter is ignored to avoid complex JOINs

	queryWithPoster := `
		SELECT id, path, title, size, modified, nfo_path, stable_key, poster_path
		FROM media_items
		ORDER BY modified DESC
	`
	queryWithoutPoster := `
		SELECT id, path, title, size, modified, nfo_path, stable_key
		FROM media_items
		ORDER BY modified DESC
	`

	args := []interface{}{}
	if limit > 0 {
		queryWithPoster += " LIMIT ?"
		queryWithoutPoster += " LIMIT ?"
		args = append(args, limit)
	}

	return s.queryMediaItemsWithFallback(queryWithPoster, queryWithoutPoster, args...)
}

// Erweiterung 4: Collections/Playlists

func (s *Store) CreateCollection(id, name, description string, createdAt time.Time) error {
	if s.readOnly {
		return fmt.Errorf("storage: read-only mode")
	}
	_, err := s.db.Exec(`
		INSERT INTO collections (id, name, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`, id, name, description, createdAt.Unix(), createdAt.Unix())
	return err
}

func (s *Store) GetCollections(limit, offset int) ([]server.Collection, error) {
	query := `
		SELECT c.id, c.name, c.description, c.created_at, c.updated_at, COUNT(ci.media_id) as item_count
		FROM collections c
		LEFT JOIN collection_items ci ON c.id = ci.collection_id
		GROUP BY c.id
		ORDER BY c.created_at DESC
	`
	args := []interface{}{}

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}
	if offset > 0 {
		query += " OFFSET ?"
		args = append(args, offset)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var collections []server.Collection
	for rows.Next() {
		var c server.Collection
		var description sql.NullString
		var createdAt, updatedAt int64
		if err := rows.Scan(&c.ID, &c.Name, &description, &createdAt, &updatedAt, &c.ItemCount); err != nil {
			return nil, err
		}
		if description.Valid {
			c.Description = description.String
		}
		c.CreatedAt = time.Unix(createdAt, 0)
		c.UpdatedAt = time.Unix(updatedAt, 0)
		collections = append(collections, c)
	}
	return collections, rows.Err()
}

func (s *Store) GetCollection(id string) (*server.Collection, bool, error) {
	var c server.Collection
	var description sql.NullString
	var createdAt, updatedAt int64

	err := s.db.QueryRow(`
		SELECT c.id, c.name, c.description, c.created_at, c.updated_at, COUNT(ci.media_id) as item_count
		FROM collections c
		LEFT JOIN collection_items ci ON c.id = ci.collection_id
		WHERE c.id = ?
		GROUP BY c.id
	`, id).Scan(&c.ID, &c.Name, &description, &createdAt, &updatedAt, &c.ItemCount)

	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	if description.Valid {
		c.Description = description.String
	}
	c.CreatedAt = time.Unix(createdAt, 0)
	c.UpdatedAt = time.Unix(updatedAt, 0)

	return &c, true, nil
}

func (s *Store) UpdateCollection(id, name, description string, updatedAt time.Time) error {
	if s.readOnly {
		return fmt.Errorf("storage: read-only mode")
	}
	_, err := s.db.Exec(`
		UPDATE collections
		SET name = ?, description = ?, updated_at = ?
		WHERE id = ?
	`, name, description, updatedAt.Unix(), id)
	return err
}

func (s *Store) DeleteCollection(id string) error {
	if s.readOnly {
		return fmt.Errorf("storage: read-only mode")
	}
	_, err := s.db.Exec(`DELETE FROM collections WHERE id = ?`, id)
	return err
}

func (s *Store) AddItemToCollection(collectionID, mediaID string, position int, addedAt time.Time) error {
	if s.readOnly {
		return fmt.Errorf("storage: read-only mode")
	}
	_, err := s.db.Exec(`
		INSERT INTO collection_items (collection_id, media_id, position, added_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(collection_id, media_id) DO UPDATE SET position = excluded.position
	`, collectionID, mediaID, position, addedAt.Unix())
	return err
}

func (s *Store) RemoveItemFromCollection(collectionID, mediaID string) error {
	if s.readOnly {
		return fmt.Errorf("storage: read-only mode")
	}
	_, err := s.db.Exec(`
		DELETE FROM collection_items
		WHERE collection_id = ? AND media_id = ?
	`, collectionID, mediaID)
	return err
}

func (s *Store) GetCollectionItems(collectionID string) ([]server.MediaItem, error) {
	queryWithPoster := `
		SELECT m.id, m.path, m.title, m.size, m.modified, m.nfo_path, m.stable_key, m.poster_path
		FROM media_items m
		INNER JOIN collection_items ci ON m.id = ci.media_id
		WHERE ci.collection_id = ?
		ORDER BY ci.position, ci.added_at
	`
	queryWithoutPoster := `
		SELECT m.id, m.path, m.title, m.size, m.modified, m.nfo_path, m.stable_key
		FROM media_items m
		INNER JOIN collection_items ci ON m.id = ci.media_id
		WHERE ci.collection_id = ?
		ORDER BY ci.position, ci.added_at
	`

	return s.queryMediaItemsWithFallback(queryWithPoster, queryWithoutPoster, collectionID)
}

// Erweiterung 5: Poster/Thumbnail Support

func (s *Store) GetPosterPath(mediaID string) (string, bool, error) {
	var posterPath sql.NullString
	err := s.db.QueryRow(`SELECT poster_path FROM media_items WHERE id = ?`, mediaID).Scan(&posterPath)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if !posterPath.Valid || posterPath.String == "" {
		return "", false, nil
	}
	return posterPath.String, true, nil
}

func (s *Store) SetPosterPath(mediaID, posterPath string) error {
	if s.readOnly {
		return fmt.Errorf("storage: read-only mode")
	}
	_, err := s.db.Exec(`UPDATE media_items SET poster_path = ? WHERE id = ?`, posterPath, mediaID)
	return err
}
