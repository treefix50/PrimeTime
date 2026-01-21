package storage

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/treefix50/primetime/internal/server"
)

// GetItemsByRoots returns media items from specific library roots
func (s *Store) GetItemsByRoots(rootIDs []string, limit, offset int, sortBy, query string) ([]server.MediaItem, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("storage: missing database connection")
	}

	if len(rootIDs) == 0 {
		return []server.MediaItem{}, nil
	}

	// Build placeholders for IN clause
	placeholders := make([]string, len(rootIDs))
	args := make([]interface{}, 0, len(rootIDs)+2)
	for i := range rootIDs {
		placeholders[i] = "?"
		args = append(args, rootIDs[i])
	}

	// Base query - get items that belong to specified roots
	// We need to join with library_roots to filter by root
	queryWithPoster := `
		SELECT DISTINCT m.id, m.path, m.title, m.size, m.modified, m.nfo_path, m.stable_key, m.poster_path
		FROM media_items m
		INNER JOIN library_roots lr ON (
			m.path LIKE lr.path || '%' OR 
			m.path LIKE lr.path || '/%' OR
			m.path LIKE lr.path || '\%'
		)
		WHERE lr.id IN (` + strings.Join(placeholders, ",") + `)
	`
	queryWithoutPoster := `
		SELECT DISTINCT m.id, m.path, m.title, m.size, m.modified, m.nfo_path, m.stable_key
		FROM media_items m
		INNER JOIN library_roots lr ON (
			m.path LIKE lr.path || '%' OR 
			m.path LIKE lr.path || '/%' OR
			m.path LIKE lr.path || '\%'
		)
		WHERE lr.id IN (` + strings.Join(placeholders, ",") + `)
	`

	// Add query filter if provided
	if query != "" {
		queryWithPoster += " AND m.title LIKE ?"
		queryWithoutPoster += " AND m.title LIKE ?"
		args = append(args, "%"+query+"%")
	}

	// Add sorting
	switch sortBy {
	case "modified":
		queryWithPoster += " ORDER BY m.modified DESC"
		queryWithoutPoster += " ORDER BY m.modified DESC"
	case "size":
		queryWithPoster += " ORDER BY m.size DESC"
		queryWithoutPoster += " ORDER BY m.size DESC"
	default:
		queryWithPoster += " ORDER BY m.title"
		queryWithoutPoster += " ORDER BY m.title"
	}

	// Add pagination
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

	// Try with poster_path first, fallback without
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
	hasPosterPath := len(cols) == 8

	var items []server.MediaItem
	for rows.Next() {
		var item server.MediaItem
		var nfoPath, stableKey, posterPath sql.NullString

		if hasPosterPath {
			if err := rows.Scan(&item.ID, &item.VideoPath, &item.Title, &item.Size, &item.Modified, &nfoPath, &stableKey, &posterPath); err != nil {
				return nil, err
			}
			if posterPath.Valid {
				item.PosterPath = posterPath.String
			}
		} else {
			if err := rows.Scan(&item.ID, &item.VideoPath, &item.Title, &item.Size, &item.Modified, &nfoPath, &stableKey); err != nil {
				return nil, err
			}
		}

		if nfoPath.Valid {
			item.NFOPath = nfoPath.String
		}
		if stableKey.Valid {
			item.StableKey = stableKey.String
		}

		items = append(items, item)
	}

	return items, rows.Err()
}
