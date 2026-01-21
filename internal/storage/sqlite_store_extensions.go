package storage

import (
	"database/sql"
	"fmt"
	"strings"
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

// Watched/Unwatched Status
func (s *Store) MarkWatched(mediaID string, watchedAt time.Time) error {
	if s.readOnly {
		return fmt.Errorf("storage: read-only mode")
	}
	_, err := s.db.Exec(`INSERT INTO watched_items (media_id, watched_at, user_id) VALUES (?, ?, '') ON CONFLICT(media_id, user_id) DO UPDATE SET watched_at = excluded.watched_at`, mediaID, watchedAt.Unix())
	return err
}

func (s *Store) UnmarkWatched(mediaID string) error {
	if s.readOnly {
		return fmt.Errorf("storage: read-only mode")
	}
	_, err := s.db.Exec(`DELETE FROM watched_items WHERE media_id = ? AND user_id = ''`, mediaID)
	return err
}

func (s *Store) IsWatched(mediaID string) (bool, error) {
	var exists int
	err := s.db.QueryRow(`SELECT 1 FROM watched_items WHERE media_id = ? AND user_id = '' LIMIT 1`, mediaID).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func (s *Store) GetWatchedItems(limit, offset int) ([]server.MediaItem, error) {
	queryWithPoster := `SELECT m.id, m.path, m.title, m.size, m.modified, m.nfo_path, m.stable_key, m.poster_path FROM media_items m INNER JOIN watched_items w ON m.id = w.media_id WHERE w.user_id = '' ORDER BY w.watched_at DESC`
	queryWithoutPoster := `SELECT m.id, m.path, m.title, m.size, m.modified, m.nfo_path, m.stable_key FROM media_items m INNER JOIN watched_items w ON m.id = w.media_id WHERE w.user_id = '' ORDER BY w.watched_at DESC`
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

// Favorites/Bookmarks
func (s *Store) AddFavorite(mediaID string, addedAt time.Time) error {
	if s.readOnly {
		return fmt.Errorf("storage: read-only mode")
	}
	_, err := s.db.Exec(`INSERT INTO favorites (media_id, added_at, user_id) VALUES (?, ?, '') ON CONFLICT(media_id, user_id) DO UPDATE SET added_at = excluded.added_at`, mediaID, addedAt.Unix())
	return err
}

func (s *Store) RemoveFavorite(mediaID string) error {
	if s.readOnly {
		return fmt.Errorf("storage: read-only mode")
	}
	_, err := s.db.Exec(`DELETE FROM favorites WHERE media_id = ? AND user_id = ''`, mediaID)
	return err
}

func (s *Store) IsFavorite(mediaID string) (bool, error) {
	var exists int
	err := s.db.QueryRow(`SELECT 1 FROM favorites WHERE media_id = ? AND user_id = '' LIMIT 1`, mediaID).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func (s *Store) GetFavorites(limit, offset int) ([]server.MediaItem, error) {
	queryWithPoster := `SELECT m.id, m.path, m.title, m.size, m.modified, m.nfo_path, m.stable_key, m.poster_path FROM media_items m INNER JOIN favorites f ON m.id = f.media_id WHERE f.user_id = '' ORDER BY f.added_at DESC`
	queryWithoutPoster := `SELECT m.id, m.path, m.title, m.size, m.modified, m.nfo_path, m.stable_key FROM media_items m INNER JOIN favorites f ON m.id = f.media_id WHERE f.user_id = '' ORDER BY f.added_at DESC`
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

// Recently Added
func (s *Store) GetRecentlyAdded(limit int, days int, itemType string) ([]server.MediaItem, error) {
	queryWithPoster := `SELECT id, path, title, size, modified, nfo_path, stable_key, poster_path FROM media_items ORDER BY modified DESC`
	queryWithoutPoster := `SELECT id, path, title, size, modified, nfo_path, stable_key FROM media_items ORDER BY modified DESC`
	args := []interface{}{}
	if limit > 0 {
		queryWithPoster += " LIMIT ?"
		queryWithoutPoster += " LIMIT ?"
		args = append(args, limit)
	}
	return s.queryMediaItemsWithFallback(queryWithPoster, queryWithoutPoster, args...)
}

// Collections/Playlists
func (s *Store) CreateCollection(id, name, description string, createdAt time.Time) error {
	if s.readOnly {
		return fmt.Errorf("storage: read-only mode")
	}
	_, err := s.db.Exec(`INSERT INTO collections (id, name, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, id, name, description, createdAt.Unix(), createdAt.Unix())
	return err
}

func (s *Store) GetCollections(limit, offset int) ([]server.Collection, error) {
	query := `SELECT c.id, c.name, c.description, c.created_at, c.updated_at, COUNT(ci.media_id) as item_count FROM collections c LEFT JOIN collection_items ci ON c.id = ci.collection_id GROUP BY c.id ORDER BY c.created_at DESC`
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
	err := s.db.QueryRow(`SELECT c.id, c.name, c.description, c.created_at, c.updated_at, COUNT(ci.media_id) as item_count FROM collections c LEFT JOIN collection_items ci ON c.id = ci.collection_id WHERE c.id = ? GROUP BY c.id`, id).Scan(&c.ID, &c.Name, &description, &createdAt, &updatedAt, &c.ItemCount)
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
	_, err := s.db.Exec(`UPDATE collections SET name = ?, description = ?, updated_at = ? WHERE id = ?`, name, description, updatedAt.Unix(), id)
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
	_, err := s.db.Exec(`INSERT INTO collection_items (collection_id, media_id, position, added_at) VALUES (?, ?, ?, ?) ON CONFLICT(collection_id, media_id) DO UPDATE SET position = excluded.position`, collectionID, mediaID, position, addedAt.Unix())
	return err
}

func (s *Store) RemoveItemFromCollection(collectionID, mediaID string) error {
	if s.readOnly {
		return fmt.Errorf("storage: read-only mode")
	}
	_, err := s.db.Exec(`DELETE FROM collection_items WHERE collection_id = ? AND media_id = ?`, collectionID, mediaID)
	return err
}

func (s *Store) GetCollectionItems(collectionID string) ([]server.MediaItem, error) {
	queryWithPoster := `SELECT m.id, m.path, m.title, m.size, m.modified, m.nfo_path, m.stable_key, m.poster_path FROM media_items m INNER JOIN collection_items ci ON m.id = ci.media_id WHERE ci.collection_id = ? ORDER BY ci.position, ci.added_at`
	queryWithoutPoster := `SELECT m.id, m.path, m.title, m.size, m.modified, m.nfo_path, m.stable_key FROM media_items m INNER JOIN collection_items ci ON m.id = ci.media_id WHERE ci.collection_id = ? ORDER BY ci.position, ci.added_at`
	return s.queryMediaItemsWithFallback(queryWithPoster, queryWithoutPoster, collectionID)
}

// Poster/Thumbnail Support
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

// NFO-based filtering and TV Show grouping
func (s *Store) GetItemsByNFOType(nfoType string, limit, offset int, sortBy, query string) ([]server.MediaItem, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("storage: missing database connection")
	}
	orderBy := "m.title COLLATE NOCASE"
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "modified":
		orderBy = "m.modified DESC, m.title COLLATE NOCASE"
	case "size":
		orderBy = "m.size DESC, m.title COLLATE NOCASE"
	case "year":
		orderBy = "CAST(COALESCE(n.year, 0) AS INTEGER) DESC, m.title COLLATE NOCASE"
	}
	where := []string{"n.type = ?"}
	args := []interface{}{nfoType}
	if query != "" {
		where = append(where, "lower(COALESCE(m.title, '')) LIKE ?")
		args = append(args, "%"+strings.ToLower(query)+"%")
	}
	limitValue := limit
	if limitValue == 0 {
		limitValue = -1
	}
	args = append(args, limitValue, offset)
	querySQL := fmt.Sprintf(`SELECT m.id, m.path, m.title, m.size, m.modified, m.nfo_path, m.stable_key, m.poster_path FROM media_items m INNER JOIN nfo n ON m.id = n.media_id WHERE %s ORDER BY %s LIMIT ? OFFSET ?`, strings.Join(where, " AND "), orderBy)
	rows, err := s.db.Query(querySQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []server.MediaItem
	for rows.Next() {
		var id, path string
		var title, nfoPath, stable, posterPath sql.NullString
		var size, modified int64
		if err := rows.Scan(&id, &path, &title, &size, &modified, &nfoPath, &stable, &posterPath); err != nil {
			return nil, err
		}
		items = append(items, server.MediaItem{ID: id, VideoPath: path, Title: title.String, Size: size, Modified: time.Unix(modified, 0), NFOPath: nfoPath.String, StableKey: stable.String, PosterPath: posterPath.String})
	}
	return items, rows.Err()
}

func (s *Store) GetTVShowsGrouped(limit, offset int, sortBy, query string) ([]server.TVShowGroup, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("storage: missing database connection")
	}
	orderBy := "show_title COLLATE NOCASE"
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "modified":
		orderBy = "MAX(m.modified) DESC, show_title COLLATE NOCASE"
	case "year":
		orderBy = "CAST(COALESCE(MAX(n.year), 0) AS INTEGER) DESC, show_title COLLATE NOCASE"
	}
	where := []string{"n.type = 'episode'", "n.show_title IS NOT NULL", "n.show_title != ''"}
	args := []interface{}{}
	if query != "" {
		where = append(where, "lower(n.show_title) LIKE ?")
		args = append(args, "%"+strings.ToLower(query)+"%")
	}
	limitValue := limit
	if limitValue == 0 {
		limitValue = -1
	}
	args = append(args, limitValue, offset)
	querySQL := fmt.Sprintf(`SELECT n.show_title, COUNT(DISTINCT m.id), COUNT(DISTINCT n.season), MIN(CAST(n.season AS INTEGER)), MAX(m.modified), MAX(n.year), GROUP_CONCAT(DISTINCT m.id) FROM media_items m INNER JOIN nfo n ON m.id = n.media_id WHERE %s GROUP BY n.show_title ORDER BY %s LIMIT ? OFFSET ?`, strings.Join(where, " AND "), orderBy)
	rows, err := s.db.Query(querySQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var shows []server.TVShowGroup
	for rows.Next() {
		var showTitle, episodeIDsStr string
		var episodeCount, seasonCount, firstSeason int
		var lastModified int64
		var year sql.NullString
		if err := rows.Scan(&showTitle, &episodeCount, &seasonCount, &firstSeason, &lastModified, &year, &episodeIDsStr); err != nil {
			return nil, err
		}
		episodeIDs := strings.Split(episodeIDsStr, ",")
		firstEpisodeID := ""
		if len(episodeIDs) > 0 {
			firstEpisodeID = episodeIDs[0]
		}
		shows = append(shows, server.TVShowGroup{ShowTitle: showTitle, EpisodeCount: episodeCount, SeasonCount: seasonCount, FirstSeason: firstSeason, LastModified: time.Unix(lastModified, 0), Year: year.String, FirstEpisodeID: firstEpisodeID})
	}
	return shows, rows.Err()
}

func (s *Store) GetSeasonsByShowTitle(showTitle string) ([]server.TVSeasonGroup, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("storage: missing database connection")
	}
	querySQL := `SELECT CAST(n.season AS INTEGER), COUNT(DISTINCT m.id), MAX(m.modified), GROUP_CONCAT(m.id) FROM media_items m INNER JOIN nfo n ON m.id = n.media_id WHERE n.type = 'episode' AND n.show_title = ? AND n.season IS NOT NULL AND n.season != '' GROUP BY n.season ORDER BY CAST(n.season AS INTEGER)`
	rows, err := s.db.Query(querySQL, showTitle)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var seasons []server.TVSeasonGroup
	for rows.Next() {
		var seasonNumber, episodeCount int
		var lastModified int64
		var episodeIDsStr string
		if err := rows.Scan(&seasonNumber, &episodeCount, &lastModified, &episodeIDsStr); err != nil {
			return nil, err
		}
		episodeIDs := strings.Split(episodeIDsStr, ",")
		seasons = append(seasons, server.TVSeasonGroup{ShowTitle: showTitle, SeasonNumber: seasonNumber, EpisodeCount: episodeCount, LastModified: time.Unix(lastModified, 0), EpisodeIDs: episodeIDs})
	}
	return seasons, rows.Err()
}

func (s *Store) GetEpisodesByShowAndSeason(showTitle string, seasonNumber int) ([]server.MediaItem, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("storage: missing database connection")
	}
	querySQL := `SELECT m.id, m.path, m.title, m.size, m.modified, m.nfo_path, m.stable_key, m.poster_path FROM media_items m INNER JOIN nfo n ON m.id = n.media_id WHERE n.type = 'episode' AND n.show_title = ? AND CAST(n.season AS INTEGER) = ? ORDER BY CAST(n.episode AS INTEGER)`
	rows, err := s.db.Query(querySQL, showTitle, seasonNumber)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []server.MediaItem
	for rows.Next() {
		var id, path string
		var title, nfoPath, stable, posterPath sql.NullString
		var size, modified int64
		if err := rows.Scan(&id, &path, &title, &size, &modified, &nfoPath, &stable, &posterPath); err != nil {
			return nil, err
		}
		items = append(items, server.MediaItem{ID: id, VideoPath: path, Title: title.String, Size: size, Modified: time.Unix(modified, 0), NFOPath: nfoPath.String, StableKey: stable.String, PosterPath: posterPath.String})
	}
	return items, rows.Err()
}
