package storage

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/treefix50/primetime/internal/server"
)

// Verbesserung 3: Erweiterte Suchfunktionalität
func (s *Store) GetAllLimitedWithFilters(limit, offset int, sortBy, query, genre, year, itemType, rating string) ([]server.MediaItem, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("storage: missing database connection")
	}
	if limit < 0 || offset < 0 {
		return nil, fmt.Errorf("storage: limit/offset must be non-negative")
	}

	orderBy := "m.title COLLATE NOCASE"
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "modified":
		orderBy = "m.modified DESC, m.title COLLATE NOCASE"
	case "size":
		orderBy = "m.size DESC, m.title COLLATE NOCASE"
	}

	whereClauses := []string{}
	args := []any{}

	// Text search in title, plot, and original_title
	normalizedQuery := strings.TrimSpace(query)
	if normalizedQuery != "" {
		whereClauses = append(whereClauses, "(lower(COALESCE(m.title, '')) LIKE ? OR lower(COALESCE(n.plot, '')) LIKE ? OR lower(COALESCE(n.original_title, '')) LIKE ?)")
		searchPattern := "%" + strings.ToLower(normalizedQuery) + "%"
		args = append(args, searchPattern, searchPattern, searchPattern)
	}

	// Genre filter
	normalizedGenre := strings.TrimSpace(genre)
	if normalizedGenre != "" {
		whereClauses = append(whereClauses, "lower(COALESCE(n.genres, '')) LIKE ?")
		args = append(args, "%"+strings.ToLower(normalizedGenre)+"%")
	}

	// Year filter
	normalizedYear := strings.TrimSpace(year)
	if normalizedYear != "" {
		if yearInt, err := strconv.Atoi(normalizedYear); err == nil {
			whereClauses = append(whereClauses, "n.year = ?")
			args = append(args, yearInt)
		}
	}

	// Type filter
	normalizedType := strings.TrimSpace(itemType)
	if normalizedType != "" {
		whereClauses = append(whereClauses, "lower(COALESCE(n.type, '')) = ?")
		args = append(args, strings.ToLower(normalizedType))
	}

	// Rating filter (minimum rating)
	normalizedRating := strings.TrimSpace(rating)
	if normalizedRating != "" {
		if ratingFloat, err := strconv.ParseFloat(normalizedRating, 64); err == nil {
			whereClauses = append(whereClauses, "n.rating >= ?")
			args = append(args, ratingFloat)
		}
	}

	whereClause := ""
	if len(whereClauses) > 0 {
		whereClause = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	limitValue := limit
	if limitValue == 0 {
		limitValue = -1
	}
	args = append(args, limitValue, offset)

	querySQL := fmt.Sprintf(`
		SELECT m.id, m.path, m.title, m.size, m.modified, m.nfo_path, m.stable_key, m.poster_path
		FROM media_items m
		LEFT JOIN nfo n ON m.id = n.media_id
		%s
		ORDER BY %s
		LIMIT ? OFFSET ?
	`, whereClause, orderBy)

	rows, err := s.db.Query(querySQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []server.MediaItem
	for rows.Next() {
		var (
			id         string
			path       string
			title      sql.NullString
			size       int64
			modified   int64
			nfoPath    sql.NullString
			stable     sql.NullString
			posterPath sql.NullString
		)
		if err := rows.Scan(&id, &path, &title, &size, &modified, &nfoPath, &stable, &posterPath); err != nil {
			return nil, err
		}
		items = append(items, server.MediaItem{
			ID:         id,
			VideoPath:  path,
			Title:      title.String,
			Size:       size,
			Modified:   time.Unix(modified, 0),
			NFOPath:    nfoPath.String,
			StableKey:  stable.String,
			PosterPath: posterPath.String,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}
