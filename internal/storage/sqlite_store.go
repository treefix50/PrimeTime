package storage

import (
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/treefix50/primetime/internal/server"
)

func (s *Store) AddRoot(path, rootType string) (server.LibraryRoot, error) {
	if s == nil || s.db == nil {
		return server.LibraryRoot{}, fmt.Errorf("storage: missing database connection")
	}

	normalizedPath := strings.TrimSpace(path)
	normalizedType := strings.TrimSpace(rootType)
	if normalizedPath == "" {
		return server.LibraryRoot{}, fmt.Errorf("storage: root path is required")
	}
	if normalizedType == "" {
		normalizedType = "library"
	}

	id := stableRootID(normalizedPath, normalizedType)
	createdAt := time.Now().Unix()
	_, err := s.db.Exec(`
		INSERT INTO library_roots (id, path, type, created_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO NOTHING
	`, id, normalizedPath, normalizedType, createdAt)
	if err != nil {
		return server.LibraryRoot{}, err
	}

	var stored server.LibraryRoot
	var createdAtStored int64
	if err := s.db.QueryRow(`
		SELECT id, path, type, created_at
		FROM library_roots
		WHERE id = ?
	`, id).Scan(&stored.ID, &stored.Path, &stored.Type, &createdAtStored); err != nil {
		return server.LibraryRoot{}, err
	}
	stored.CreatedAt = time.Unix(createdAtStored, 0)
	return stored, nil
}

func (s *Store) ListRoots() ([]server.LibraryRoot, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("storage: missing database connection")
	}

	rows, err := s.db.Query(`
		SELECT id, path, type, created_at
		FROM library_roots
		ORDER BY created_at
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roots []server.LibraryRoot
	for rows.Next() {
		var root server.LibraryRoot
		var createdAt int64
		if err := rows.Scan(&root.ID, &root.Path, &root.Type, &createdAt); err != nil {
			return nil, err
		}
		root.CreatedAt = time.Unix(createdAt, 0)
		roots = append(roots, root)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return roots, nil
}

func (s *Store) RemoveRoot(id string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}
	if strings.TrimSpace(id) == "" {
		return nil
	}
	_, err := s.db.Exec(`DELETE FROM library_roots WHERE id = ?`, id)
	return err
}

func (s *Store) StartScanRun(rootID string, startedAt time.Time) (server.ScanRun, error) {
	if s == nil || s.db == nil {
		return server.ScanRun{}, fmt.Errorf("storage: missing database connection")
	}
	if strings.TrimSpace(rootID) == "" {
		return server.ScanRun{}, fmt.Errorf("storage: root ID is required")
	}
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	id := scanRunID(rootID, startedAt)
	_, err := s.db.Exec(`
		INSERT INTO scan_runs (id, root_id, started_at, status)
		VALUES (?, ?, ?, ?)
	`, id, rootID, startedAt.Unix(), "running")
	if err != nil {
		return server.ScanRun{}, err
	}
	return server.ScanRun{
		ID:        id,
		RootID:    rootID,
		StartedAt: startedAt,
		Status:    "running",
	}, nil
}

func (s *Store) FinishScanRun(id string, finishedAt time.Time) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}
	if strings.TrimSpace(id) == "" {
		return nil
	}
	if finishedAt.IsZero() {
		finishedAt = time.Now()
	}
	_, err := s.db.Exec(`
		UPDATE scan_runs
		SET finished_at = ?, status = ?, error = NULL
		WHERE id = ?
	`, finishedAt.Unix(), "success", id)
	return err
}

func (s *Store) FailScanRun(id string, finishedAt time.Time, errMsg string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}
	if strings.TrimSpace(id) == "" {
		return nil
	}
	if finishedAt.IsZero() {
		finishedAt = time.Now()
	}
	_, err := s.db.Exec(`
		UPDATE scan_runs
		SET finished_at = ?, status = ?, error = ?
		WHERE id = ?
	`, finishedAt.Unix(), "failed", nullString(errMsg), id)
	return err
}

func (s *Store) SaveItems(items []server.MediaItem) (err error) {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}
	if len(items) == 0 {
		return nil
	}

	const batchSize = 750
	for start := 0; start < len(items); start += batchSize {
		end := start + batchSize
		if end > len(items) {
			end = len(items)
		}

		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		rollback := func() {
			_ = tx.Rollback()
		}

		stmt, err := tx.Prepare(`
		INSERT INTO media_items (id, path, title, size, modified, nfo_path, stable_key, poster_path)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			path=excluded.path,
			title=excluded.title,
			size=excluded.size,
			modified=excluded.modified,
			nfo_path=excluded.nfo_path,
			stable_key=excluded.stable_key,
			poster_path=excluded.poster_path
	`)
		if err != nil {
			rollback()
			return err
		}

		for _, item := range items[start:end] {
			_, err = stmt.Exec(
				item.ID,
				item.VideoPath,
				item.Title,
				item.Size,
				item.Modified.Unix(),
				nullString(item.NFOPath),
				nullString(item.StableKey),
				nullString(item.PosterPath),
			)
			if err != nil {
				stmt.Close()
				rollback()
				return err
			}
		}

		if err := stmt.Close(); err != nil {
			rollback()
			return err
		}

		if err = tx.Commit(); err != nil {
			rollback()
			return err
		}
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

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	rollback := func() {
		_ = tx.Rollback()
	}

	if _, err := tx.Exec(query, args...); err != nil {
		rollback()
		return err
	}

	playbackQuery := fmt.Sprintf(
		"DELETE FROM playback_state WHERE media_id IN (%s)",
		strings.Join(placeholders, ","),
	)
	if _, err := tx.Exec(playbackQuery, args...); err != nil {
		rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		rollback()
		return err
	}
	return nil
}

func (s *Store) GetAll() ([]server.MediaItem, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("storage: missing database connection")
	}

	rows, err := s.db.Query(`
		SELECT id, path, title, size, modified, nfo_path, stable_key, poster_path
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

func (s *Store) GetAllLimited(limit, offset int, sortBy, query string) ([]server.MediaItem, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("storage: missing database connection")
	}
	if limit < 0 || offset < 0 {
		return nil, fmt.Errorf("storage: limit/offset must be non-negative")
	}

	orderBy := "title COLLATE NOCASE"
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "modified":
		orderBy = "modified DESC, title COLLATE NOCASE"
	case "size":
		orderBy = "size DESC, title COLLATE NOCASE"
	}

	where := ""
	args := []any{}
	normalizedQuery := strings.TrimSpace(query)
	if normalizedQuery != "" {
		where = "WHERE lower(COALESCE(title, '')) LIKE ?"
		args = append(args, "%"+strings.ToLower(normalizedQuery)+"%")
	}

	limitValue := limit
	if limitValue == 0 {
		limitValue = -1
	}
	args = append(args, limitValue, offset)

	querySQL := fmt.Sprintf(`
		SELECT id, path, title, size, modified, nfo_path, stable_key, poster_path
		FROM media_items
		%s
		ORDER BY %s
		LIMIT ? OFFSET ?
	`, where, orderBy)

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

func (s *Store) GetByID(id string) (server.MediaItem, bool, error) {
	if s == nil || s.db == nil {
		return server.MediaItem{}, false, fmt.Errorf("storage: missing database connection")
	}

	var (
		item       server.MediaItem
		title      sql.NullString
		modified   int64
		nfoPath    sql.NullString
		stableKey  sql.NullString
		posterPath sql.NullString
	)

	err := s.db.QueryRow(`
		SELECT id, path, title, size, modified, nfo_path, stable_key, poster_path
		FROM media_items
		WHERE id = ?
	`, id).Scan(&item.ID, &item.VideoPath, &title, &item.Size, &modified, &nfoPath, &stableKey, &posterPath)
	if err != nil {
		if err == sql.ErrNoRows {
			return server.MediaItem{}, false, nil
		}
		return server.MediaItem{}, false, err
	}

	if title.Valid {
		item.Title = title.String
	}
	item.Modified = time.Unix(modified, 0)
	if nfoPath.Valid {
		item.NFOPath = nfoPath.String
	}
	if stableKey.Valid {
		item.StableKey = stableKey.String
	}
	if posterPath.Valid {
		item.PosterPath = posterPath.String
	}

	return item, true, nil
}

func (s *Store) GetIDByPath(path string) (string, bool, error) {
	if s == nil || s.db == nil {
		return "", false, fmt.Errorf("storage: missing database connection")
	}

	var id string
	err := s.db.QueryRow(`
		SELECT id
		FROM media_items
		WHERE path = ?
	`, path).Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", false, nil
		}
		return "", false, err
	}
	return id, true, nil
}

func (s *Store) SaveNFO(mediaID string, nfo *server.NFO) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}

	if nfo == nil {
		return s.DeleteNFO(mediaID)
	}

	genres := strings.Join(nfo.Genres, ",")

	// Convert Actor structs to comma-separated names
	actorNames := make([]string, 0, len(nfo.Actors))
	for _, actor := range nfo.Actors {
		if actor.Name != "" {
			actorNames = append(actorNames, actor.Name)
		}
	}
	actors := strings.Join(actorNames, ",")

	directors := strings.Join(nfo.Directors, ",")
	studios := strings.Join(nfo.Studios, ",")
	year := parseInt(nfo.Year)
	rating := parseFloat(nfo.Rating)
	season := parseInt(nfo.Season)
	episode := parseInt(nfo.Episode)
	runtime := parseInt(nfo.Runtime)

	_, err := s.db.Exec(`
		INSERT INTO nfo (
			media_id, type, title, original_title, plot, year, rating,
			genres, season, episode, show_title, raw_root,
			actors, directors, studios, runtime, imdb_id, tmdb_id
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
			raw_root=excluded.raw_root,
			actors=excluded.actors,
			directors=excluded.directors,
			studios=excluded.studios,
			runtime=excluded.runtime,
			imdb_id=excluded.imdb_id,
			tmdb_id=excluded.tmdb_id
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
		nullString(actors),
		nullString(directors),
		nullString(studios),
		runtime,
		nullString(nfo.IMDbID),
		nullString(nfo.TMDbID),
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
		actors      sql.NullString
		directors   sql.NullString
		studios     sql.NullString
		runtime     sql.NullInt64
		imdbID      sql.NullString
		tmdbID      sql.NullString
	)

	err := s.db.QueryRow(`
		SELECT type, title, original_title, plot, year, rating, genres,
			season, episode, show_title, raw_root,
			actors, directors, studios, runtime, imdb_id, tmdb_id
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
		&actors,
		&directors,
		&studios,
		&runtime,
		&imdbID,
		&tmdbID,
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
	nfo.IMDbID = imdbID.String
	nfo.TMDbID = tmdbID.String

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
	if runtime.Valid {
		nfo.Runtime = strconv.FormatInt(runtime.Int64, 10)
	}
	if genres.Valid {
		parts := strings.Split(genres.String, ",")
		nfo.Genres = trimGenres(parts)
	}
	if actors.Valid {
		parts := strings.Split(actors.String, ",")
		actorNames := trimGenres(parts)
		// Convert actor names back to Actor structs
		nfo.Actors = make([]server.Actor, 0, len(actorNames))
		for _, name := range actorNames {
			nfo.Actors = append(nfo.Actors, server.Actor{Name: name})
		}
	}
	if directors.Valid {
		parts := strings.Split(directors.String, ",")
		nfo.Directors = trimGenres(parts)
	}
	if studios.Valid {
		parts := strings.Split(studios.String, ",")
		nfo.Studios = trimGenres(parts)
	}

	return &nfo, true, nil
}

func (s *Store) UpsertPlaybackState(mediaID string, positionSeconds, durationSeconds int64, lastPlayedAt int64, percentComplete *float64, clientID string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}

	normalizedClientID := strings.TrimSpace(clientID)
	percentValue := sql.NullFloat64{}
	if percentComplete != nil {
		percentValue = sql.NullFloat64{Float64: *percentComplete, Valid: true}
	}
	_, err := s.db.Exec(`
		INSERT INTO playback_state (
			media_id, position_seconds, duration_seconds, updated_at, last_played_at, percent_complete, client_id
		)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(media_id, client_id) DO UPDATE SET
			position_seconds=excluded.position_seconds,
			duration_seconds=excluded.duration_seconds,
			updated_at=excluded.updated_at,
			last_played_at=excluded.last_played_at,
			percent_complete=excluded.percent_complete
	`,
		mediaID,
		positionSeconds,
		durationSeconds,
		time.Now().Unix(),
		lastPlayedAt,
		percentValue,
		normalizedClientID,
	)
	return err
}

func (s *Store) GetPlaybackState(mediaID, clientID string) (*server.PlaybackState, bool, error) {
	if s == nil || s.db == nil {
		return nil, false, fmt.Errorf("storage: missing database connection")
	}

	normalizedClientID := strings.TrimSpace(clientID)
	query := `
		SELECT media_id, position_seconds, duration_seconds, updated_at, last_played_at, percent_complete, client_id
		FROM playback_state
		WHERE media_id = ?
	`
	args := []any{mediaID}
	if normalizedClientID != "" {
		query += " AND client_id = ?"
		args = append(args, normalizedClientID)
	} else {
		query += " ORDER BY updated_at DESC LIMIT 1"
	}

	var state server.PlaybackState
	var percentComplete sql.NullFloat64
	err := s.db.QueryRow(query, args...).Scan(
		&state.MediaID,
		&state.PositionSeconds,
		&state.DurationSeconds,
		&state.UpdatedAt,
		&state.LastPlayedAt,
		&percentComplete,
		&state.ClientID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	if percentComplete.Valid {
		value := percentComplete.Float64
		state.PercentComplete = &value
	}

	return &state, true, nil
}

func (s *Store) DeletePlaybackState(mediaID, clientID string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}

	normalizedClientID := strings.TrimSpace(clientID)
	query := "DELETE FROM playback_state WHERE media_id = ?"
	args := []any{mediaID}
	if normalizedClientID != "" {
		query += " AND client_id = ?"
		args = append(args, normalizedClientID)
	}
	_, err := s.db.Exec(query, args...)
	return err
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

func stableRootID(path, rootType string) string {
	sum := sha1.Sum([]byte(path + ":" + rootType))
	return hex.EncodeToString(sum[:8])
}

func scanRunID(rootID string, startedAt time.Time) string {
	sum := sha1.Sum([]byte(fmt.Sprintf("%s:%d", rootID, startedAt.UnixNano())))
	return hex.EncodeToString(sum[:8])
}

// Verbesserung 2: Batch-Operations für Playback-State
func (s *Store) GetAllPlaybackStates(clientID string, onlyUnfinished bool) ([]server.PlaybackState, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("storage: missing database connection")
	}

	query := `
		SELECT media_id, position_seconds, duration_seconds, updated_at, last_played_at, percent_complete, client_id
		FROM playback_state
		WHERE 1=1
	`
	args := []any{}

	normalizedClientID := strings.TrimSpace(clientID)
	if normalizedClientID != "" {
		query += " AND client_id = ?"
		args = append(args, normalizedClientID)
	}

	if onlyUnfinished {
		query += " AND (percent_complete IS NULL OR percent_complete < 90.0)"
	}

	query += " ORDER BY last_played_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var states []server.PlaybackState
	for rows.Next() {
		var state server.PlaybackState
		var percentComplete sql.NullFloat64
		if err := rows.Scan(
			&state.MediaID,
			&state.PositionSeconds,
			&state.DurationSeconds,
			&state.UpdatedAt,
			&state.LastPlayedAt,
			&percentComplete,
			&state.ClientID,
		); err != nil {
			return nil, err
		}
		if percentComplete.Valid {
			value := percentComplete.Float64
			state.PercentComplete = &value
		}
		states = append(states, state)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return states, nil
}

// Verbesserung 4: Duplicate Detection
func (s *Store) GetDuplicates() ([]server.DuplicateGroup, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("storage: missing database connection")
	}

	query := `
		SELECT stable_key, GROUP_CONCAT(id, ',') as ids, COUNT(*) as count
		FROM media_items
		WHERE stable_key IS NOT NULL AND stable_key != ''
		GROUP BY stable_key
		HAVING count > 1
		ORDER BY count DESC, stable_key
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []server.DuplicateGroup
	for rows.Next() {
		var stableKey string
		var idsStr string
		var count int
		if err := rows.Scan(&stableKey, &idsStr, &count); err != nil {
			return nil, err
		}

		ids := strings.Split(idsStr, ",")
		items := make([]server.MediaItem, 0, len(ids))
		for _, id := range ids {
			item, ok, err := s.GetByID(id)
			if err != nil {
				return nil, err
			}
			if ok {
				items = append(items, item)
			}
		}

		if len(items) > 1 {
			groups = append(groups, server.DuplicateGroup{
				StableKey: stableKey,
				Items:     items,
				Count:     len(items),
			})
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return groups, nil
}

// Verbesserung 5: Erweiterte Statistiken
func (s *Store) GetDetailedStats() (*server.DetailedStats, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("storage: missing database connection")
	}

	stats := &server.DetailedStats{}

	// Total items
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM media_items`).Scan(&stats.TotalItems); err != nil {
		return nil, err
	}

	// Total size
	if err := s.db.QueryRow(`SELECT COALESCE(SUM(size), 0) FROM media_items`).Scan(&stats.TotalSizeBytes); err != nil {
		return nil, err
	}

	// Items by type
	rows, err := s.db.Query(`
		SELECT COALESCE(n.type, 'unknown') as type, COUNT(*) as count
		FROM media_items m
		LEFT JOIN nfo n ON m.id = n.media_id
		GROUP BY n.type
		ORDER BY count DESC
	`)
	if err != nil {
		return nil, err
	}
	stats.ItemsByType = make(map[string]int)
	for rows.Next() {
		var itemType string
		var count int
		if err := rows.Scan(&itemType, &count); err != nil {
			rows.Close()
			return nil, err
		}
		stats.ItemsByType[itemType] = count
	}
	rows.Close()

	// Items with/without NFO
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM media_items WHERE nfo_path IS NOT NULL AND nfo_path != ''`).Scan(&stats.ItemsWithNFO); err != nil {
		return nil, err
	}
	stats.ItemsWithoutNFO = stats.TotalItems - stats.ItemsWithNFO

	// Top 10 most watched
	rows, err = s.db.Query(`
		SELECT m.id, m.title, COUNT(DISTINCT p.client_id) as watch_count
		FROM media_items m
		INNER JOIN playback_state p ON m.id = p.media_id
		GROUP BY m.id, m.title
		ORDER BY watch_count DESC, m.title
		LIMIT 10
	`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var item server.TopItem
		if err := rows.Scan(&item.ID, &item.Title, &item.WatchCount); err != nil {
			rows.Close()
			return nil, err
		}
		stats.TopWatchedItems = append(stats.TopWatchedItems, item)
	}
	rows.Close()

	// Recent scan runs
	rows, err = s.db.Query(`
		SELECT id, root_id, started_at, finished_at, status, error
		FROM scan_runs
		ORDER BY started_at DESC
		LIMIT 10
	`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var run server.ScanRun
		var finishedAt sql.NullInt64
		var errorMsg sql.NullString
		var startedAtUnix int64
		if err := rows.Scan(&run.ID, &run.RootID, &startedAtUnix, &finishedAt, &run.Status, &errorMsg); err != nil {
			rows.Close()
			return nil, err
		}
		run.StartedAt = time.Unix(startedAtUnix, 0)
		if finishedAt.Valid {
			run.FinishedAt = time.Unix(finishedAt.Int64, 0)
		}
		run.Error = errorMsg.String
		stats.RecentScans = append(stats.RecentScans, run)
	}
	rows.Close()

	return stats, nil
}
