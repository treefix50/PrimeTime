package storage

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/treefix50/primetime/internal/server"
)

// SaveNFOExtended saves complete NFO data including actors, unique IDs, and stream details
func (s *Store) SaveNFOExtended(mediaID string, nfo *server.NFO) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("storage: missing database connection")
	}

	if nfo == nil {
		return s.DeleteNFOExtended(mediaID)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("storage: begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Save main NFO data
	genres := strings.Join(nfo.Genres, ",")
	directors := strings.Join(nfo.Directors, ",")
	studios := strings.Join(nfo.Studios, ",")
	countries := strings.Join(nfo.Countries, ",")
	trailers := strings.Join(nfo.Trailers, ",")

	year := parseInt(nfo.Year)
	rating := parseFloat(nfo.Rating)
	season := parseInt(nfo.Season)
	episode := parseInt(nfo.Episode)
	runtime := parseInt(nfo.Runtime)

	_, err = tx.Exec(`
		INSERT INTO nfo (
			media_id, type, title, original_title, plot, year, rating,
			genres, season, episode, show_title, raw_root,
			directors, studios, runtime, imdb_id, tmdb_id, tvdb_id,
			mpaa, premiered, release_date, countries, trailers, date_added
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
			directors=excluded.directors,
			studios=excluded.studios,
			runtime=excluded.runtime,
			imdb_id=excluded.imdb_id,
			tmdb_id=excluded.tmdb_id,
			tvdb_id=excluded.tvdb_id,
			mpaa=excluded.mpaa,
			premiered=excluded.premiered,
			release_date=excluded.release_date,
			countries=excluded.countries,
			trailers=excluded.trailers,
			date_added=excluded.date_added
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
		nullString(directors),
		nullString(studios),
		runtime,
		nullString(nfo.IMDbID),
		nullString(nfo.TMDbID),
		nullString(nfo.TVDbID),
		nullString(nfo.MPAA),
		nullString(nfo.Premiered),
		nullString(nfo.ReleaseDate),
		nullString(countries),
		nullString(trailers),
		nullString(nfo.DateAdded),
	)
	if err != nil {
		return fmt.Errorf("storage: save nfo: %w", err)
	}

	// Delete existing related data
	if _, err = tx.Exec(`DELETE FROM nfo_actors WHERE media_id = ?`, mediaID); err != nil {
		return fmt.Errorf("storage: delete actors: %w", err)
	}
	if _, err = tx.Exec(`DELETE FROM nfo_unique_ids WHERE media_id = ?`, mediaID); err != nil {
		return fmt.Errorf("storage: delete unique ids: %w", err)
	}
	if _, err = tx.Exec(`DELETE FROM nfo_stream_video WHERE media_id = ?`, mediaID); err != nil {
		return fmt.Errorf("storage: delete video streams: %w", err)
	}
	if _, err = tx.Exec(`DELETE FROM nfo_stream_audio WHERE media_id = ?`, mediaID); err != nil {
		return fmt.Errorf("storage: delete audio streams: %w", err)
	}
	if _, err = tx.Exec(`DELETE FROM nfo_stream_subtitle WHERE media_id = ?`, mediaID); err != nil {
		return fmt.Errorf("storage: delete subtitle streams: %w", err)
	}

	// Save actors
	if len(nfo.Actors) > 0 {
		actorStmt, err := tx.Prepare(`
			INSERT INTO nfo_actors (media_id, name, role, type, tmdb_id, tvdb_id, imdb_id, sort_order)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`)
		if err != nil {
			return fmt.Errorf("storage: prepare actor statement: %w", err)
		}
		defer actorStmt.Close()

		for i, actor := range nfo.Actors {
			if strings.TrimSpace(actor.Name) == "" {
				continue
			}
			_, err = actorStmt.Exec(
				mediaID,
				actor.Name,
				nullString(actor.Role),
				nullString(actor.Type),
				nullString(actor.TMDbID),
				nullString(actor.TVDbID),
				nullString(actor.IMDbID),
				i,
			)
			if err != nil {
				return fmt.Errorf("storage: save actor: %w", err)
			}
		}
	}

	// Save unique IDs
	if len(nfo.UniqueIDs) > 0 {
		uidStmt, err := tx.Prepare(`
			INSERT INTO nfo_unique_ids (media_id, type, value)
			VALUES (?, ?, ?)
		`)
		if err != nil {
			return fmt.Errorf("storage: prepare unique id statement: %w", err)
		}
		defer uidStmt.Close()

		for _, uid := range nfo.UniqueIDs {
			if strings.TrimSpace(uid.Type) == "" || strings.TrimSpace(uid.Value) == "" {
				continue
			}
			_, err = uidStmt.Exec(mediaID, uid.Type, uid.Value)
			if err != nil {
				return fmt.Errorf("storage: save unique id: %w", err)
			}
		}
	}

	// Save stream details
	if nfo.StreamDetails != nil {
		// Video streams
		if len(nfo.StreamDetails.Video) > 0 {
			videoStmt, err := tx.Prepare(`
				INSERT INTO nfo_stream_video (
					media_id, codec, bitrate, width, height, aspect, aspect_ratio,
					frame_rate, scan_type, duration, duration_seconds, stream_index
				)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`)
			if err != nil {
				return fmt.Errorf("storage: prepare video stream statement: %w", err)
			}
			defer videoStmt.Close()

			for i, v := range nfo.StreamDetails.Video {
				_, err = videoStmt.Exec(
					mediaID,
					nullString(v.Codec),
					nullString(v.Bitrate),
					nullString(v.Width),
					nullString(v.Height),
					nullString(v.Aspect),
					nullString(v.AspectRatio),
					nullString(v.FrameRate),
					nullString(v.ScanType),
					nullString(v.Duration),
					nullString(v.DurationInSecs),
					i,
				)
				if err != nil {
					return fmt.Errorf("storage: save video stream: %w", err)
				}
			}
		}

		// Audio streams
		if len(nfo.StreamDetails.Audio) > 0 {
			audioStmt, err := tx.Prepare(`
				INSERT INTO nfo_stream_audio (
					media_id, codec, bitrate, language, channels, sampling_rate, stream_index
				)
				VALUES (?, ?, ?, ?, ?, ?, ?)
			`)
			if err != nil {
				return fmt.Errorf("storage: prepare audio stream statement: %w", err)
			}
			defer audioStmt.Close()

			for i, a := range nfo.StreamDetails.Audio {
				_, err = audioStmt.Exec(
					mediaID,
					nullString(a.Codec),
					nullString(a.Bitrate),
					nullString(a.Language),
					nullString(a.Channels),
					nullString(a.SamplingRate),
					i,
				)
				if err != nil {
					return fmt.Errorf("storage: save audio stream: %w", err)
				}
			}
		}

		// Subtitle streams
		if len(nfo.StreamDetails.Subtitle) > 0 {
			subtitleStmt, err := tx.Prepare(`
				INSERT INTO nfo_stream_subtitle (media_id, codec, language, stream_index)
				VALUES (?, ?, ?, ?)
			`)
			if err != nil {
				return fmt.Errorf("storage: prepare subtitle stream statement: %w", err)
			}
			defer subtitleStmt.Close()

			for i, sub := range nfo.StreamDetails.Subtitle {
				_, err = subtitleStmt.Exec(
					mediaID,
					nullString(sub.Codec),
					nullString(sub.Language),
					i,
				)
				if err != nil {
					return fmt.Errorf("storage: save subtitle stream: %w", err)
				}
			}
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("storage: commit transaction: %w", err)
	}

	return nil
}

// GetNFOExtended retrieves complete NFO data including actors, unique IDs, and stream details
func (s *Store) GetNFOExtended(mediaID string) (*server.NFO, bool, error) {
	if s == nil || s.db == nil {
		return nil, false, fmt.Errorf("storage: missing database connection")
	}

	// Get main NFO data
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
		directors   sql.NullString
		studios     sql.NullString
		runtime     sql.NullInt64
		imdbID      sql.NullString
		tmdbID      sql.NullString
		tvdbID      sql.NullString
		mpaa        sql.NullString
		premiered   sql.NullString
		releaseDate sql.NullString
		countries   sql.NullString
		trailers    sql.NullString
		dateAdded   sql.NullString
	)

	err := s.db.QueryRow(`
		SELECT type, title, original_title, plot, year, rating, genres,
			season, episode, show_title, raw_root,
			directors, studios, runtime, imdb_id, tmdb_id, tvdb_id,
			mpaa, premiered, release_date, countries, trailers, date_added
		FROM nfo
		WHERE media_id = ?
	`, mediaID).Scan(
		&nfoType, &title, &original, &plot, &year, &rating, &genres,
		&season, &episode, &showTitle, &rawRootName,
		&directors, &studios, &runtime, &imdbID, &tmdbID, &tvdbID,
		&mpaa, &premiered, &releaseDate, &countries, &trailers, &dateAdded,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	// Populate basic fields
	nfo.Type = nfoType.String
	nfo.Title = title.String
	nfo.Original = original.String
	nfo.Plot = plot.String
	nfo.ShowTitle = showTitle.String
	nfo.RawRootName = rawRootName.String
	nfo.IMDbID = imdbID.String
	nfo.TMDbID = tmdbID.String
	nfo.TVDbID = tvdbID.String
	nfo.MPAA = mpaa.String
	nfo.Premiered = premiered.String
	nfo.ReleaseDate = releaseDate.String
	nfo.DateAdded = dateAdded.String

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
		nfo.Genres = trimGenres(strings.Split(genres.String, ","))
	}
	if directors.Valid {
		nfo.Directors = trimGenres(strings.Split(directors.String, ","))
	}
	if studios.Valid {
		nfo.Studios = trimGenres(strings.Split(studios.String, ","))
	}
	if countries.Valid {
		nfo.Countries = trimGenres(strings.Split(countries.String, ","))
	}
	if trailers.Valid {
		nfo.Trailers = trimGenres(strings.Split(trailers.String, ","))
	}

	// Get actors
	actorRows, err := s.db.Query(`
		SELECT name, role, type, tmdb_id, tvdb_id, imdb_id
		FROM nfo_actors
		WHERE media_id = ?
		ORDER BY sort_order
	`, mediaID)
	if err != nil {
		return nil, false, err
	}
	defer actorRows.Close()

	for actorRows.Next() {
		var actor server.Actor
		var role, actorType, tmdbID, tvdbID, imdbID sql.NullString
		if err := actorRows.Scan(&actor.Name, &role, &actorType, &tmdbID, &tvdbID, &imdbID); err != nil {
			return nil, false, err
		}
		actor.Role = role.String
		actor.Type = actorType.String
		actor.TMDbID = tmdbID.String
		actor.TVDbID = tvdbID.String
		actor.IMDbID = imdbID.String
		nfo.Actors = append(nfo.Actors, actor)
	}

	// Get unique IDs
	uidRows, err := s.db.Query(`
		SELECT type, value
		FROM nfo_unique_ids
		WHERE media_id = ?
	`, mediaID)
	if err != nil {
		return nil, false, err
	}
	defer uidRows.Close()

	for uidRows.Next() {
		var uid server.UniqueID
		if err := uidRows.Scan(&uid.Type, &uid.Value); err != nil {
			return nil, false, err
		}
		nfo.UniqueIDs = append(nfo.UniqueIDs, uid)
	}

	// Get stream details
	nfo.StreamDetails = &server.StreamDetails{}

	// Video streams
	videoRows, err := s.db.Query(`
		SELECT codec, bitrate, width, height, aspect, aspect_ratio,
			frame_rate, scan_type, duration, duration_seconds
		FROM nfo_stream_video
		WHERE media_id = ?
		ORDER BY stream_index
	`, mediaID)
	if err != nil {
		return nil, false, err
	}
	defer videoRows.Close()

	for videoRows.Next() {
		var v server.VideoStream
		var codec, bitrate, width, height, aspect, aspectRatio, frameRate, scanType, duration, durationSecs sql.NullString
		if err := videoRows.Scan(&codec, &bitrate, &width, &height, &aspect, &aspectRatio, &frameRate, &scanType, &duration, &durationSecs); err != nil {
			return nil, false, err
		}
		v.Codec = codec.String
		v.Bitrate = bitrate.String
		v.Width = width.String
		v.Height = height.String
		v.Aspect = aspect.String
		v.AspectRatio = aspectRatio.String
		v.FrameRate = frameRate.String
		v.ScanType = scanType.String
		v.Duration = duration.String
		v.DurationInSecs = durationSecs.String
		nfo.StreamDetails.Video = append(nfo.StreamDetails.Video, v)
	}

	// Audio streams
	audioRows, err := s.db.Query(`
		SELECT codec, bitrate, language, channels, sampling_rate
		FROM nfo_stream_audio
		WHERE media_id = ?
		ORDER BY stream_index
	`, mediaID)
	if err != nil {
		return nil, false, err
	}
	defer audioRows.Close()

	for audioRows.Next() {
		var a server.AudioStream
		var codec, bitrate, language, channels, samplingRate sql.NullString
		if err := audioRows.Scan(&codec, &bitrate, &language, &channels, &samplingRate); err != nil {
			return nil, false, err
		}
		a.Codec = codec.String
		a.Bitrate = bitrate.String
		a.Language = language.String
		a.Channels = channels.String
		a.SamplingRate = samplingRate.String
		nfo.StreamDetails.Audio = append(nfo.StreamDetails.Audio, a)
	}

	// Subtitle streams
	subtitleRows, err := s.db.Query(`
		SELECT codec, language
		FROM nfo_stream_subtitle
		WHERE media_id = ?
		ORDER BY stream_index
	`, mediaID)
	if err != nil {
		return nil, false, err
	}
	defer subtitleRows.Close()

	for subtitleRows.Next() {
		var sub server.SubtitleStream
		var codec, language sql.NullString
		if err := subtitleRows.Scan(&codec, &language); err != nil {
			return nil, false, err
		}
		sub.Codec = codec.String
		sub.Language = language.String
		nfo.StreamDetails.Subtitle = append(nfo.StreamDetails.Subtitle, sub)
	}

	return &nfo, true, nil
}

// DeleteNFOExtended deletes all NFO data including related tables
func (s *Store) DeleteNFOExtended(mediaID string) error {
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

	// Delete from all related tables (CASCADE should handle this, but being explicit)
	tables := []string{
		"nfo_actors",
		"nfo_unique_ids",
		"nfo_stream_video",
		"nfo_stream_audio",
		"nfo_stream_subtitle",
		"nfo",
	}

	for _, table := range tables {
		if _, err = tx.Exec(fmt.Sprintf(`DELETE FROM %s WHERE media_id = ?`, table), mediaID); err != nil {
			return err
		}
	}

	return tx.Commit()
}
