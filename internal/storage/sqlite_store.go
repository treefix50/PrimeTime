package storage

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/treefix50/primetime/internal/server"
)

func (s *Store) SaveItems(items []server.MediaItem) (err error) {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	stmt, err := tx.Prepare(`
		INSERT INTO media_items (id, path, title, size, modified, nfo_path)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			path=excluded.path,
			title=excluded.title,
			size=excluded.size,
			modified=excluded.modified,
			nfo_path=excluded.nfo_path
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, item := range items {
		_, err = stmt.Exec(
			item.ID,
			item.VideoPath,
			item.Title,
			item.Size,
			item.Modified.Unix(),
			nullString(item.NFOPath),
		)
		if err != nil {
			return err
		}
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (s *Store) DeleteItems(ids []string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}
	if len(ids) == 0 {
		return nil
	}

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(
		"DELETE FROM media_items WHERE id IN (%s)",
		strings.Join(placeholders, ","),
	)

	_, err := s.db.Exec(query, args...)
	return err
}

func (s *Store) GetAll() ([]server.MediaItem, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("storage: missing database connection")
	}

	rows, err := s.db.Query(`
		SELECT id, path, title, size, modified, nfo_path
		FROM media_items
		ORDER BY title
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []server.MediaItem
	for rows.Next() {
		var (
			id       string
			path     string
			title    sql.NullString
			size     int64
			modified int64
			nfoPath  sql.NullString
		)
		if err := rows.Scan(&id, &path, &title, &size, &modified, &nfoPath); err != nil {
			return nil, err
		}
		items = append(items, server.MediaItem{
			ID:        id,
			VideoPath: path,
			Title:     title.String,
			Size:      size,
			Modified:  time.Unix(modified, 0),
			NFOPath:   nfoPath.String,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (s *Store) GetByID(id string) (server.MediaItem, bool, error) {
	if s == nil || s.db == nil {
		return server.MediaItem{}, false, fmt.Errorf("storage: missing database connection")
	}

	var (
		item     server.MediaItem
		title    sql.NullString
		modified int64
		nfoPath  sql.NullString
	)

	err := s.db.QueryRow(`
		SELECT id, path, title, size, modified, nfo_path
		FROM media_items
		WHERE id = ?
	`, id).Scan(&item.ID, &item.VideoPath, &title, &item.Size, &modified, &nfoPath)
	if err != nil {
		if err == sql.ErrNoRows {
			return server.MediaItem{}, false, nil
		}
		return server.MediaItem{}, false, err
	}

	item.Title = title.String
	item.Modified = time.Unix(modified, 0)
	item.NFOPath = nfoPath.String

	return item, true, nil
}

func (s *Store) SaveNFO(mediaID string, nfo *server.NFO) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}

	if nfo == nil {
		return s.DeleteNFO(mediaID)
	}

	genres := strings.Join(nfo.Genres, ",")
	year := parseInt(nfo.Year)
	rating := parseFloat(nfo.Rating)
	season := parseInt(nfo.Season)
	episode := parseInt(nfo.Episode)

	_, err := s.db.Exec(`
		INSERT INTO nfo (
			media_id, type, title, original_title, plot, year, rating,
			genres, season, episode, show_title, raw_root
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(media_id) DO UPDATE SET
			type=excluded.type,
			title=excluded.title,
			original_title=excluded.original_title,
			plot=excluded.plot,
			year=excluded.year,
			rating=excluded.rating,
			genres=excluded.genres,
			season=excluded.season,
			episode=excluded.episode,
			show_title=excluded.show_title,
			raw_root=excluded.raw_root
	`,
		mediaID,
		nullString(nfo.Type),
		nullString(nfo.Title),
		nullString(nfo.Original),
		nullString(nfo.Plot),
		year,
		rating,
		nullString(genres),
		season,
		episode,
		nullString(nfo.ShowTitle),
		nullString(nfo.RawRootName),
	)
	return err
}

func (s *Store) DeleteNFO(mediaID string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}

	_, err := s.db.Exec(`DELETE FROM nfo WHERE media_id = ?`, mediaID)
	return err
}

func (s *Store) GetNFO(mediaID string) (*server.NFO, bool, error) {
	if s == nil || s.db == nil {
		return nil, false, fmt.Errorf("storage: missing database connection")
	}

	var (
		nfo         server.NFO
		nfoType     sql.NullString
		title       sql.NullString
		original    sql.NullString
		plot        sql.NullString
		year        sql.NullInt64
		rating      sql.NullFloat64
		genres      sql.NullString
		season      sql.NullInt64
		episode     sql.NullInt64
		showTitle   sql.NullString
		rawRootName sql.NullString
	)

	err := s.db.QueryRow(`
		SELECT type, title, original_title, plot, year, rating, genres,
			season, episode, show_title, raw_root
		FROM nfo
		WHERE media_id = ?
	`, mediaID).Scan(
		&nfoType,
		&title,
		&original,
		&plot,
		&year,
		&rating,
		&genres,
		&season,
		&episode,
		&showTitle,
		&rawRootName,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	nfo.Type = nfoType.String
	nfo.Title = title.String
	nfo.Original = original.String
	nfo.Plot = plot.String
	nfo.ShowTitle = showTitle.String
	nfo.RawRootName = rawRootName.String

	if year.Valid {
		nfo.Year = strconv.FormatInt(year.Int64, 10)
	}
	if rating.Valid {
		nfo.Rating = strconv.FormatFloat(rating.Float64, 'f', -1, 64)
	}
	if season.Valid {
		nfo.Season = strconv.FormatInt(season.Int64, 10)
	}
	if episode.Valid {
		nfo.Episode = strconv.FormatInt(episode.Int64, 10)
	}
	if genres.Valid {
		parts := strings.Split(genres.String, ",")
		nfo.Genres = trimGenres(parts)
	}

	return &nfo, true, nil
}

func nullString(value string) sql.NullString {
	if strings.TrimSpace(value) == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

func parseInt(value string) sql.NullInt64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return sql.NullInt64{}
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: parsed, Valid: true}
}

func parseFloat(value string) sql.NullFloat64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return sql.NullFloat64{}
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: parsed, Valid: true}
}

func trimGenres(genres []string) []string {
	out := make([]string, 0, len(genres))
	for _, genre := range genres {
		genre = strings.TrimSpace(genre)
		if genre != "" {
			out = append(out, genre)
		}
	}
	return out
}
