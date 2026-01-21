package storage

import (
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/treefix50/primetime/internal/server"
)

// ============================================================================
// Verbesserung 1: Multi-User-Support
// ============================================================================

func (s *Store) CreateMediaUser(id, name string, createdAt time.Time) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}
	_, err := s.db.Exec(`
		INSERT INTO users (id, name, created_at, last_active)
		VALUES (?, ?, ?, ?)
	`, id, name, createdAt.Unix(), createdAt.Unix())
	return err
}

func (s *Store) GetMediaUser(id string) (*server.User, bool, error) {
	if s == nil || s.db == nil {
		return nil, false, fmt.Errorf("storage: missing database connection")
	}

	var user server.User
	var createdAt, lastActive int64
	err := s.db.QueryRow(`
		SELECT id, name, created_at, last_active
		FROM users
		WHERE id = ?
	`, id).Scan(&user.ID, &user.Name, &createdAt, &lastActive)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	user.CreatedAt = time.Unix(createdAt, 0)
	if lastActive > 0 {
		user.LastActive = time.Unix(lastActive, 0)
	}
	return &user, true, nil
}

func (s *Store) GetMediaUserByName(name string) (*server.User, bool, error) {
	if s == nil || s.db == nil {
		return nil, false, fmt.Errorf("storage: missing database connection")
	}

	var user server.User
	var createdAt, lastActive int64
	err := s.db.QueryRow(`
		SELECT id, name, created_at, last_active
		FROM users
		WHERE name = ?
	`, name).Scan(&user.ID, &user.Name, &createdAt, &lastActive)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	user.CreatedAt = time.Unix(createdAt, 0)
	if lastActive > 0 {
		user.LastActive = time.Unix(lastActive, 0)
	}
	return &user, true, nil
}

func (s *Store) GetAllUsers() ([]server.User, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("storage: missing database connection")
	}

	rows, err := s.db.Query(`
		SELECT id, name, created_at, last_active
		FROM users
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []server.User
	for rows.Next() {
		var user server.User
		var createdAt, lastActive int64
		if err := rows.Scan(&user.ID, &user.Name, &createdAt, &lastActive); err != nil {
			return nil, err
		}
		user.CreatedAt = time.Unix(createdAt, 0)
		if lastActive > 0 {
			user.LastActive = time.Unix(lastActive, 0)
		}
		users = append(users, user)
	}

	return users, rows.Err()
}

func (s *Store) UpdateUserLastActive(id string, lastActive time.Time) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}
	_, err := s.db.Exec(`
		UPDATE users
		SET last_active = ?
		WHERE id = ?
	`, lastActive.Unix(), id)
	return err
}

func (s *Store) DeleteMediaUser(id string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}
	_, err := s.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	return err
}

func (s *Store) SetUserPreference(userID, key, value string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}
	_, err := s.db.Exec(`
		INSERT INTO user_preferences (user_id, key, value)
		VALUES (?, ?, ?)
		ON CONFLICT(user_id, key) DO UPDATE SET value = excluded.value
	`, userID, key, value)
	return err
}

func (s *Store) GetUserPreference(userID, key string) (string, bool, error) {
	if s == nil || s.db == nil {
		return "", false, fmt.Errorf("storage: missing database connection")
	}

	var value sql.NullString
	err := s.db.QueryRow(`
		SELECT value
		FROM user_preferences
		WHERE user_id = ? AND key = ?
	`, userID, key).Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", false, nil
		}
		return "", false, err
	}

	return value.String, value.Valid, nil
}

func (s *Store) GetAllUserPreferences(userID string) (map[string]string, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("storage: missing database connection")
	}

	rows, err := s.db.Query(`
		SELECT key, value
		FROM user_preferences
		WHERE user_id = ?
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	prefs := make(map[string]string)
	for rows.Next() {
		var key string
		var value sql.NullString
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		if value.Valid {
			prefs[key] = value.String
		}
	}

	return prefs, rows.Err()
}

// ============================================================================
// Verbesserung 2: Transkodierungs-Profile
// ============================================================================

func (s *Store) CreateTranscodingProfile(profile server.TranscodingProfile) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}
	_, err := s.db.Exec(`
		INSERT INTO transcoding_profiles (id, name, video_codec, audio_codec, resolution, max_bitrate, container, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, profile.ID, profile.Name, profile.VideoCodec, profile.AudioCodec,
		nullString(profile.Resolution), nullInt64(profile.MaxBitrate), profile.Container, profile.CreatedAt.Unix())
	return err
}

func (s *Store) GetTranscodingProfile(id string) (*server.TranscodingProfile, bool, error) {
	if s == nil || s.db == nil {
		return nil, false, fmt.Errorf("storage: missing database connection")
	}

	var profile server.TranscodingProfile
	var resolution sql.NullString
	var maxBitrate sql.NullInt64
	var createdAt int64

	err := s.db.QueryRow(`
		SELECT id, name, video_codec, audio_codec, resolution, max_bitrate, container, created_at
		FROM transcoding_profiles
		WHERE id = ?
	`, id).Scan(&profile.ID, &profile.Name, &profile.VideoCodec, &profile.AudioCodec,
		&resolution, &maxBitrate, &profile.Container, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	if resolution.Valid {
		profile.Resolution = resolution.String
	}
	if maxBitrate.Valid {
		profile.MaxBitrate = maxBitrate.Int64
	}
	profile.CreatedAt = time.Unix(createdAt, 0)
	return &profile, true, nil
}

func (s *Store) GetTranscodingProfileByName(name string) (*server.TranscodingProfile, bool, error) {
	if s == nil || s.db == nil {
		return nil, false, fmt.Errorf("storage: missing database connection")
	}

	var profile server.TranscodingProfile
	var resolution sql.NullString
	var maxBitrate sql.NullInt64
	var createdAt int64

	err := s.db.QueryRow(`
		SELECT id, name, video_codec, audio_codec, resolution, max_bitrate, container, created_at
		FROM transcoding_profiles
		WHERE name = ?
	`, name).Scan(&profile.ID, &profile.Name, &profile.VideoCodec, &profile.AudioCodec,
		&resolution, &maxBitrate, &profile.Container, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	if resolution.Valid {
		profile.Resolution = resolution.String
	}
	if maxBitrate.Valid {
		profile.MaxBitrate = maxBitrate.Int64
	}
	profile.CreatedAt = time.Unix(createdAt, 0)
	return &profile, true, nil
}

func (s *Store) GetAllTranscodingProfiles() ([]server.TranscodingProfile, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("storage: missing database connection")
	}

	rows, err := s.db.Query(`
		SELECT id, name, video_codec, audio_codec, resolution, max_bitrate, container, created_at
		FROM transcoding_profiles
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var profiles []server.TranscodingProfile
	for rows.Next() {
		var profile server.TranscodingProfile
		var resolution sql.NullString
		var maxBitrate sql.NullInt64
		var createdAt int64

		if err := rows.Scan(&profile.ID, &profile.Name, &profile.VideoCodec, &profile.AudioCodec,
			&resolution, &maxBitrate, &profile.Container, &createdAt); err != nil {
			return nil, err
		}

		if resolution.Valid {
			profile.Resolution = resolution.String
		}
		if maxBitrate.Valid {
			profile.MaxBitrate = maxBitrate.Int64
		}
		profile.CreatedAt = time.Unix(createdAt, 0)
		profiles = append(profiles, profile)
	}

	return profiles, rows.Err()
}

func (s *Store) DeleteTranscodingProfile(id string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}
	_, err := s.db.Exec(`DELETE FROM transcoding_profiles WHERE id = ?`, id)
	return err
}

func (s *Store) SaveTranscodingCache(cache server.TranscodingCache) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}
	_, err := s.db.Exec(`
		INSERT INTO transcoding_cache (id, media_id, profile_id, cache_path, created_at, last_accessed, size_bytes)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			last_accessed = excluded.last_accessed,
			size_bytes = excluded.size_bytes
	`, cache.ID, cache.MediaID, cache.ProfileID, cache.CachePath,
		cache.CreatedAt.Unix(), cache.LastAccessed.Unix(), cache.SizeBytes)
	return err
}

func (s *Store) GetTranscodingCache(mediaID, profileID string) (*server.TranscodingCache, bool, error) {
	if s == nil || s.db == nil {
		return nil, false, fmt.Errorf("storage: missing database connection")
	}

	var cache server.TranscodingCache
	var createdAt, lastAccessed int64

	err := s.db.QueryRow(`
		SELECT id, media_id, profile_id, cache_path, created_at, last_accessed, size_bytes
		FROM transcoding_cache
		WHERE media_id = ? AND profile_id = ?
	`, mediaID, profileID).Scan(&cache.ID, &cache.MediaID, &cache.ProfileID, &cache.CachePath,
		&createdAt, &lastAccessed, &cache.SizeBytes)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	cache.CreatedAt = time.Unix(createdAt, 0)
	cache.LastAccessed = time.Unix(lastAccessed, 0)
	return &cache, true, nil
}

func (s *Store) DeleteTranscodingCache(id string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}
	_, err := s.db.Exec(`DELETE FROM transcoding_cache WHERE id = ?`, id)
	return err
}

func (s *Store) CleanOldTranscodingCache(olderThan time.Time) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}
	_, err := s.db.Exec(`
		DELETE FROM transcoding_cache
		WHERE last_accessed < ?
	`, olderThan.Unix())
	return err
}

// ============================================================================
// Verbesserung 3: Serien-Verwaltung (TV Shows)
// ============================================================================

func (s *Store) CreateTVShow(show server.TVShow) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}

	genres := strings.Join(show.Genres, ",")
	_, err := s.db.Exec(`
		INSERT INTO tv_shows (id, title, original_title, plot, poster_path, year, genres, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, show.ID, show.Title, nullString(show.OriginalTitle), nullString(show.Plot),
		nullString(show.PosterPath), nullInt(show.Year), nullString(genres),
		show.CreatedAt.Unix(), show.UpdatedAt.Unix())
	return err
}

func (s *Store) GetTVShow(id string) (*server.TVShow, bool, error) {
	if s == nil || s.db == nil {
		return nil, false, fmt.Errorf("storage: missing database connection")
	}

	var show server.TVShow
	var originalTitle, plot, posterPath, genres sql.NullString
	var year sql.NullInt64
	var createdAt, updatedAt int64

	err := s.db.QueryRow(`
		SELECT id, title, original_title, plot, poster_path, year, genres, created_at, updated_at
		FROM tv_shows
		WHERE id = ?
	`, id).Scan(&show.ID, &show.Title, &originalTitle, &plot, &posterPath, &year, &genres, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	if originalTitle.Valid {
		show.OriginalTitle = originalTitle.String
	}
	if plot.Valid {
		show.Plot = plot.String
	}
	if posterPath.Valid {
		show.PosterPath = posterPath.String
	}
	if year.Valid {
		show.Year = int(year.Int64)
	}
	if genres.Valid && genres.String != "" {
		show.Genres = strings.Split(genres.String, ",")
	}
	show.CreatedAt = time.Unix(createdAt, 0)
	show.UpdatedAt = time.Unix(updatedAt, 0)

	// Count seasons and episodes
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM seasons WHERE show_id = ?`, id).Scan(&show.SeasonCount)
	_ = s.db.QueryRow(`
		SELECT COUNT(*)
		FROM episodes e
		INNER JOIN seasons s ON e.season_id = s.id
		WHERE s.show_id = ?
	`, id).Scan(&show.EpisodeCount)

	return &show, true, nil
}

func (s *Store) GetAllTVShows(limit, offset int) ([]server.TVShow, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("storage: missing database connection")
	}

	limitValue := limit
	if limitValue == 0 {
		limitValue = -1
	}

	rows, err := s.db.Query(`
		SELECT id, title, original_title, plot, poster_path, year, genres, created_at, updated_at
		FROM tv_shows
		ORDER BY title
		LIMIT ? OFFSET ?
	`, limitValue, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var shows []server.TVShow
	for rows.Next() {
		var show server.TVShow
		var originalTitle, plot, posterPath, genres sql.NullString
		var year sql.NullInt64
		var createdAt, updatedAt int64

		if err := rows.Scan(&show.ID, &show.Title, &originalTitle, &plot, &posterPath, &year, &genres, &createdAt, &updatedAt); err != nil {
			return nil, err
		}

		if originalTitle.Valid {
			show.OriginalTitle = originalTitle.String
		}
		if plot.Valid {
			show.Plot = plot.String
		}
		if posterPath.Valid {
			show.PosterPath = posterPath.String
		}
		if year.Valid {
			show.Year = int(year.Int64)
		}
		if genres.Valid && genres.String != "" {
			show.Genres = strings.Split(genres.String, ",")
		}
		show.CreatedAt = time.Unix(createdAt, 0)
		show.UpdatedAt = time.Unix(updatedAt, 0)

		// Count seasons and episodes
		_ = s.db.QueryRow(`SELECT COUNT(*) FROM seasons WHERE show_id = ?`, show.ID).Scan(&show.SeasonCount)
		_ = s.db.QueryRow(`
			SELECT COUNT(*)
			FROM episodes e
			INNER JOIN seasons s ON e.season_id = s.id
			WHERE s.show_id = ?
		`, show.ID).Scan(&show.EpisodeCount)

		shows = append(shows, show)
	}

	return shows, rows.Err()
}

func (s *Store) UpdateTVShow(show server.TVShow) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}

	genres := strings.Join(show.Genres, ",")
	_, err := s.db.Exec(`
		UPDATE tv_shows
		SET title = ?, original_title = ?, plot = ?, poster_path = ?, year = ?, genres = ?, updated_at = ?
		WHERE id = ?
	`, show.Title, nullString(show.OriginalTitle), nullString(show.Plot),
		nullString(show.PosterPath), nullInt(show.Year), nullString(genres),
		show.UpdatedAt.Unix(), show.ID)
	return err
}

func (s *Store) DeleteTVShow(id string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}
	_, err := s.db.Exec(`DELETE FROM tv_shows WHERE id = ?`, id)
	return err
}

func (s *Store) CreateSeason(season server.Season) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}

	_, err := s.db.Exec(`
		INSERT INTO seasons (id, show_id, season_number, title, plot, poster_path, episode_count, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, season.ID, season.ShowID, season.SeasonNumber, nullString(season.Title),
		nullString(season.Plot), nullString(season.PosterPath), season.EpisodeCount, season.CreatedAt.Unix())
	return err
}

func (s *Store) GetSeason(id string) (*server.Season, bool, error) {
	if s == nil || s.db == nil {
		return nil, false, fmt.Errorf("storage: missing database connection")
	}

	var season server.Season
	var title, plot, posterPath sql.NullString
	var createdAt int64

	err := s.db.QueryRow(`
		SELECT id, show_id, season_number, title, plot, poster_path, episode_count, created_at
		FROM seasons
		WHERE id = ?
	`, id).Scan(&season.ID, &season.ShowID, &season.SeasonNumber, &title, &plot, &posterPath, &season.EpisodeCount, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	if title.Valid {
		season.Title = title.String
	}
	if plot.Valid {
		season.Plot = plot.String
	}
	if posterPath.Valid {
		season.PosterPath = posterPath.String
	}
	season.CreatedAt = time.Unix(createdAt, 0)
	return &season, true, nil
}

func (s *Store) GetSeasonsByShow(showID string) ([]server.Season, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("storage: missing database connection")
	}

	rows, err := s.db.Query(`
		SELECT id, show_id, season_number, title, plot, poster_path, episode_count, created_at
		FROM seasons
		WHERE show_id = ?
		ORDER BY season_number
	`, showID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var seasons []server.Season
	for rows.Next() {
		var season server.Season
		var title, plot, posterPath sql.NullString
		var createdAt int64

		if err := rows.Scan(&season.ID, &season.ShowID, &season.SeasonNumber, &title, &plot, &posterPath, &season.EpisodeCount, &createdAt); err != nil {
			return nil, err
		}

		if title.Valid {
			season.Title = title.String
		}
		if plot.Valid {
			season.Plot = plot.String
		}
		if posterPath.Valid {
			season.PosterPath = posterPath.String
		}
		season.CreatedAt = time.Unix(createdAt, 0)
		seasons = append(seasons, season)
	}

	return seasons, rows.Err()
}

func (s *Store) UpdateSeason(season server.Season) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}

	_, err := s.db.Exec(`
		UPDATE seasons
		SET title = ?, plot = ?, poster_path = ?, episode_count = ?
		WHERE id = ?
	`, nullString(season.Title), nullString(season.Plot), nullString(season.PosterPath), season.EpisodeCount, season.ID)
	return err
}

func (s *Store) DeleteSeason(id string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}
	_, err := s.db.Exec(`DELETE FROM seasons WHERE id = ?`, id)
	return err
}

func (s *Store) CreateEpisode(episode server.Episode) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}

	_, err := s.db.Exec(`
		INSERT INTO episodes (id, season_id, episode_number, media_id, title, plot, air_date, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, episode.ID, episode.SeasonID, episode.EpisodeNumber, episode.MediaID,
		nullString(episode.Title), nullString(episode.Plot), nullString(episode.AirDate), episode.CreatedAt.Unix())
	return err
}

func (s *Store) GetEpisode(id string) (*server.Episode, bool, error) {
	if s == nil || s.db == nil {
		return nil, false, fmt.Errorf("storage: missing database connection")
	}

	var episode server.Episode
	var title, plot, airDate sql.NullString
	var createdAt int64

	err := s.db.QueryRow(`
		SELECT id, season_id, episode_number, media_id, title, plot, air_date, created_at
		FROM episodes
		WHERE id = ?
	`, id).Scan(&episode.ID, &episode.SeasonID, &episode.EpisodeNumber, &episode.MediaID, &title, &plot, &airDate, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	if title.Valid {
		episode.Title = title.String
	}
	if plot.Valid {
		episode.Plot = plot.String
	}
	if airDate.Valid {
		episode.AirDate = airDate.String
	}
	episode.CreatedAt = time.Unix(createdAt, 0)
	return &episode, true, nil
}

func (s *Store) AutoGroupEpisodes() error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}

	// Get all media items with NFO that have show_title
	rows, err := s.db.Query(`
		SELECT m.id, n.show_title, n.season, n.episode, n.title, n.plot
		FROM media_items m
		INNER JOIN nfo n ON m.id = n.media_id
		WHERE n.show_title IS NOT NULL AND n.show_title != ''
			AND n.season IS NOT NULL AND n.episode IS NOT NULL
		ORDER BY n.show_title, n.season, n.episode
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type episodeInfo struct {
		mediaID   string
		showTitle string
		season    int
		episode   int
		title     string
		plot      string
	}

	var episodes []episodeInfo
	for rows.Next() {
		var info episodeInfo
		var season, episode sql.NullInt64
		var title, plot sql.NullString

		if err := rows.Scan(&info.mediaID, &info.showTitle, &season, &episode, &title, &plot); err != nil {
			return err
		}

		if season.Valid {
			info.season = int(season.Int64)
		}
		if episode.Valid {
			info.episode = int(episode.Int64)
		}
		if title.Valid {
			info.title = title.String
		}
		if plot.Valid {
			info.plot = plot.String
		}

		episodes = append(episodes, info)
	}

	if err := rows.Err(); err != nil {
		return err
	}

	// Group episodes by show
	showMap := make(map[string][]episodeInfo)
	for _, ep := range episodes {
		showMap[ep.showTitle] = append(showMap[ep.showTitle], ep)
	}

	// Create shows, seasons, and episodes
	now := time.Now()
	for showTitle, eps := range showMap {
		// Create or get show
		showID := generateShowID(showTitle)

		// Check if show exists
		var exists bool
		err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM tv_shows WHERE id = ?)`, showID).Scan(&exists)
		if err != nil {
			return err
		}

		if !exists {
			show := server.TVShow{
				ID:        showID,
				Title:     showTitle,
				CreatedAt: now,
				UpdatedAt: now,
			}
			if err := s.CreateTVShow(show); err != nil {
				return err
			}
		}

		// Group by season
		seasonMap := make(map[int][]episodeInfo)
		for _, ep := range eps {
			seasonMap[ep.season] = append(seasonMap[ep.season], ep)
		}

		// Create seasons and episodes
		for seasonNum, seasonEps := range seasonMap {
			seasonID := generateSeasonID(showID, seasonNum)

			// Check if season exists
			var seasonExists bool
			err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM seasons WHERE id = ?)`, seasonID).Scan(&seasonExists)
			if err != nil {
				return err
			}

			if !seasonExists {
				season := server.Season{
					ID:           seasonID,
					ShowID:       showID,
					SeasonNumber: seasonNum,
					EpisodeCount: len(seasonEps),
					CreatedAt:    now,
				}
				if err := s.CreateSeason(season); err != nil {
					return err
				}
			} else {
				// Update episode count
				_, err := s.db.Exec(`UPDATE seasons SET episode_count = ? WHERE id = ?`, len(seasonEps), seasonID)
				if err != nil {
					return err
				}
			}

			// Create episodes
			for _, ep := range seasonEps {
				episodeID := generateEpisodeID(seasonID, ep.episode)

				// Check if episode exists
				var epExists bool
				err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM episodes WHERE id = ?)`, episodeID).Scan(&epExists)
				if err != nil {
					return err
				}

				if !epExists {
					episode := server.Episode{
						ID:            episodeID,
						SeasonID:      seasonID,
						EpisodeNumber: ep.episode,
						MediaID:       ep.mediaID,
						Title:         ep.title,
						Plot:          ep.plot,
						CreatedAt:     now,
					}
					if err := s.CreateEpisode(episode); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

// Helper functions

func nullInt64(value int64) sql.NullInt64 {
	if value == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: value, Valid: true}
}

func nullInt(value int) sql.NullInt64 {
	if value == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(value), Valid: true}
}

func generateShowID(title string) string {
	sum := sha1.Sum([]byte("show:" + strings.ToLower(title)))
	return "show_" + hex.EncodeToString(sum[:8])
}

func generateSeasonID(showID string, seasonNumber int) string {
	sum := sha1.Sum([]byte(fmt.Sprintf("%s:season:%d", showID, seasonNumber)))
	return "season_" + hex.EncodeToString(sum[:8])
}

func generateEpisodeID(seasonID string, episodeNumber int) string {
	sum := sha1.Sum([]byte(fmt.Sprintf("%s:episode:%d", seasonID, episodeNumber)))
	return "episode_" + hex.EncodeToString(sum[:8])
}

func (s *Store) GetEpisodesBySeason(seasonID string) ([]server.Episode, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("storage: missing database connection")
	}

	rows, err := s.db.Query(`
		SELECT id, season_id, episode_number, media_id, title, plot, air_date, created_at
		FROM episodes
		WHERE season_id = ?
		ORDER BY episode_number
	`, seasonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var episodes []server.Episode
	for rows.Next() {
		var episode server.Episode
		var title, plot, airDate sql.NullString
		var createdAt int64

		if err := rows.Scan(&episode.ID, &episode.SeasonID, &episode.EpisodeNumber, &episode.MediaID, &title, &plot, &airDate, &createdAt); err != nil {
			return nil, err
		}

		if title.Valid {
			episode.Title = title.String
		}
		if plot.Valid {
			episode.Plot = plot.String
		}
		if airDate.Valid {
			episode.AirDate = airDate.String
		}
		episode.CreatedAt = time.Unix(createdAt, 0)
		episodes = append(episodes, episode)
	}

	return episodes, rows.Err()
}

func (s *Store) GetEpisodeByMediaID(mediaID string) (*server.Episode, bool, error) {
	if s == nil || s.db == nil {
		return nil, false, fmt.Errorf("storage: missing database connection")
	}

	var episode server.Episode
	var title, plot, airDate sql.NullString
	var createdAt int64

	err := s.db.QueryRow(`
		SELECT id, season_id, episode_number, media_id, title, plot, air_date, created_at
		FROM episodes
		WHERE media_id = ?
	`, mediaID).Scan(&episode.ID, &episode.SeasonID, &episode.EpisodeNumber, &episode.MediaID, &title, &plot, &airDate, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	if title.Valid {
		episode.Title = title.String
	}
	if plot.Valid {
		episode.Plot = plot.String
	}
	if airDate.Valid {
		episode.AirDate = airDate.String
	}
	episode.CreatedAt = time.Unix(createdAt, 0)
	return &episode, true, nil
}

func (s *Store) UpdateEpisode(episode server.Episode) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}

	_, err := s.db.Exec(`
		UPDATE episodes
		SET title = ?, plot = ?, air_date = ?
		WHERE id = ?
	`, nullString(episode.Title), nullString(episode.Plot), nullString(episode.AirDate), episode.ID)
	return err
}

func (s *Store) DeleteEpisode(id string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}
	_, err := s.db.Exec(`DELETE FROM episodes WHERE id = ?`, id)
	return err
}

func (s *Store) GetNextUnwatchedEpisode(showID, userID string) (*server.Episode, bool, error) {
	if s == nil || s.db == nil {
		return nil, false, fmt.Errorf("storage: missing database connection")
	}

	// Find the next unwatched episode for this show and user
	var episode server.Episode
	var title, plot, airDate sql.NullString
	var createdAt int64

	query := `
		SELECT e.id, e.season_id, e.episode_number, e.media_id, e.title, e.plot, e.air_date, e.created_at
		FROM episodes e
		INNER JOIN seasons s ON e.season_id = s.id
		LEFT JOIN watched_items w ON e.media_id = w.media_id AND w.user_id = ?
		WHERE s.show_id = ? AND w.media_id IS NULL
		ORDER BY s.season_number, e.episode_number
		LIMIT 1
	`

	err := s.db.QueryRow(query, userID, showID).Scan(
		&episode.ID, &episode.SeasonID, &episode.EpisodeNumber, &episode.MediaID, &title, &plot, &airDate, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	if title.Valid {
		episode.Title = title.String
	}
	if plot.Valid {
		episode.Plot = plot.String
	}
	if airDate.Valid {
		episode.AirDate = airDate.String
	}
	episode.CreatedAt = time.Unix(createdAt, 0)
	return &episode, true, nil
}
