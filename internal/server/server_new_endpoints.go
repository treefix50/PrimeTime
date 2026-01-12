package server

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ============================================================================
// Verbesserung 1: Multi-User-Support Endpoints
// ============================================================================

func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	if s.handleOptions(w, r, "GET, POST, OPTIONS") {
		return
	}
	if s.lib.store == nil {
		s.writeError(w, "not available without database", http.StatusNotImplemented)
		return
	}

	switch r.Method {
	case http.MethodGet:
		users, err := s.lib.store.GetAllUsers()
		if err != nil {
			s.writeError(w, errInternal, http.StatusInternalServerError)
			return
		}
		writeJSON(w, r, users)

	case http.MethodPost:
		if s.readOnly {
			s.writeError(w, "read-only mode", http.StatusForbidden)
			return
		}

		var payload struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			s.writeError(w, "bad request", http.StatusBadRequest)
			return
		}

		if strings.TrimSpace(payload.Name) == "" {
			s.writeError(w, "name is required", http.StatusBadRequest)
			return
		}

		// Check if user already exists
		existing, ok, err := s.lib.store.GetUserByName(payload.Name)
		if err != nil {
			s.writeError(w, errInternal, http.StatusInternalServerError)
			return
		}
		if ok {
			writeJSON(w, r, existing)
			return
		}

		id := generateUserID(payload.Name)
		if err := s.lib.store.CreateUser(id, payload.Name, time.Now()); err != nil {
			s.writeError(w, errInternal, http.StatusInternalServerError)
			return
		}

		user, ok, err := s.lib.store.GetUser(id)
		if err != nil {
			s.writeError(w, errInternal, http.StatusInternalServerError)
			return
		}
		if !ok {
			s.writeError(w, errNotFound, http.StatusNotFound)
			return
		}
		writeJSON(w, r, user)

	default:
		s.methodNotAllowed(w)
	}
}

func (s *Server) handleUserDetail(w http.ResponseWriter, r *http.Request) {
	if s.handleOptions(w, r, "GET, DELETE, OPTIONS") {
		return
	}
	if s.lib.store == nil {
		s.writeError(w, "not available without database", http.StatusNotImplemented)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/users/")
	parts := strings.Split(path, "/")
	if len(parts) < 1 || parts[0] == "" {
		s.writeError(w, errNotFound, http.StatusNotFound)
		return
	}

	userID := parts[0]

	switch r.Method {
	case http.MethodGet:
		user, ok, err := s.lib.store.GetUser(userID)
		if err != nil {
			s.writeError(w, errInternal, http.StatusInternalServerError)
			return
		}
		if !ok {
			s.writeError(w, errNotFound, http.StatusNotFound)
			return
		}

		// Update last active
		if !s.readOnly {
			_ = s.lib.store.UpdateUserLastActive(userID, time.Now())
		}

		writeJSON(w, r, user)

	case http.MethodDelete:
		if s.readOnly {
			s.writeError(w, "read-only mode", http.StatusForbidden)
			return
		}

		if err := s.lib.store.DeleteUser(userID); err != nil {
			s.writeError(w, errInternal, http.StatusInternalServerError)
			return
		}
		writeJSON(w, r, map[string]string{"status": "ok"})

	default:
		s.methodNotAllowed(w)
	}
}

// ============================================================================
// Verbesserung 2: Transkodierungs-Profile Endpoints
// ============================================================================

func (s *Server) handleTranscodingProfiles(w http.ResponseWriter, r *http.Request) {
	if s.handleOptions(w, r, "GET, POST, OPTIONS") {
		return
	}
	if s.lib.store == nil {
		s.writeError(w, "not available without database", http.StatusNotImplemented)
		return
	}

	switch r.Method {
	case http.MethodGet:
		profiles, err := s.lib.store.GetAllTranscodingProfiles()
		if err != nil {
			s.writeError(w, errInternal, http.StatusInternalServerError)
			return
		}
		writeJSON(w, r, profiles)

	case http.MethodPost:
		if s.readOnly {
			s.writeError(w, "read-only mode", http.StatusForbidden)
			return
		}

		var payload struct {
			Name       string `json:"name"`
			VideoCodec string `json:"videoCodec"`
			AudioCodec string `json:"audioCodec"`
			Resolution string `json:"resolution"`
			MaxBitrate int64  `json:"maxBitrate"`
			Container  string `json:"container"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			s.writeError(w, "bad request", http.StatusBadRequest)
			return
		}

		if strings.TrimSpace(payload.Name) == "" {
			s.writeError(w, "name is required", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(payload.VideoCodec) == "" {
			s.writeError(w, "videoCodec is required", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(payload.AudioCodec) == "" {
			s.writeError(w, "audioCodec is required", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(payload.Container) == "" {
			payload.Container = "mp4"
		}

		id := generateProfileID(payload.Name)
		profile := TranscodingProfile{
			ID:         id,
			Name:       payload.Name,
			VideoCodec: payload.VideoCodec,
			AudioCodec: payload.AudioCodec,
			Resolution: payload.Resolution,
			MaxBitrate: payload.MaxBitrate,
			Container:  payload.Container,
			CreatedAt:  time.Now(),
		}

		if err := s.lib.store.CreateTranscodingProfile(profile); err != nil {
			s.writeError(w, errInternal, http.StatusInternalServerError)
			return
		}

		created, ok, err := s.lib.store.GetTranscodingProfile(id)
		if err != nil {
			s.writeError(w, errInternal, http.StatusInternalServerError)
			return
		}
		if !ok {
			s.writeError(w, errNotFound, http.StatusNotFound)
			return
		}
		writeJSON(w, r, created)

	default:
		s.methodNotAllowed(w)
	}
}

func (s *Server) handleTranscodingProfileDetail(w http.ResponseWriter, r *http.Request) {
	if s.handleOptions(w, r, "GET, DELETE, OPTIONS") {
		return
	}
	if s.lib.store == nil {
		s.writeError(w, "not available without database", http.StatusNotImplemented)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/transcoding/profiles/")
	parts := strings.Split(path, "/")
	if len(parts) < 1 || parts[0] == "" {
		s.writeError(w, errNotFound, http.StatusNotFound)
		return
	}

	profileID := parts[0]

	switch r.Method {
	case http.MethodGet:
		profile, ok, err := s.lib.store.GetTranscodingProfile(profileID)
		if err != nil {
			s.writeError(w, errInternal, http.StatusInternalServerError)
			return
		}
		if !ok {
			s.writeError(w, errNotFound, http.StatusNotFound)
			return
		}
		writeJSON(w, r, profile)

	case http.MethodDelete:
		if s.readOnly {
			s.writeError(w, "read-only mode", http.StatusForbidden)
			return
		}

		if err := s.lib.store.DeleteTranscodingProfile(profileID); err != nil {
			s.writeError(w, errInternal, http.StatusInternalServerError)
			return
		}
		writeJSON(w, r, map[string]string{"status": "ok"})

	default:
		s.methodNotAllowed(w)
	}
}

// ============================================================================
// Verbesserung 3: TV Shows/Serien-Verwaltung Endpoints
// ============================================================================

func (s *Server) handleTVShows(w http.ResponseWriter, r *http.Request) {
	if s.handleOptions(w, r, "GET, POST, OPTIONS") {
		return
	}
	if s.lib.store == nil {
		s.writeError(w, "not available without database", http.StatusNotImplemented)
		return
	}

	switch r.Method {
	case http.MethodGet:
		limit, offset, ok := parseLimitOffset(r)
		if !ok {
			s.writeError(w, "bad request", http.StatusBadRequest)
			return
		}

		shows, err := s.lib.store.GetAllTVShows(limit, offset)
		if err != nil {
			s.writeError(w, errInternal, http.StatusInternalServerError)
			return
		}
		writeJSON(w, r, shows)

	case http.MethodPost:
		if s.readOnly {
			s.writeError(w, "read-only mode", http.StatusForbidden)
			return
		}

		// Trigger auto-grouping of episodes
		if err := s.lib.store.AutoGroupEpisodes(); err != nil {
			s.writeError(w, errInternal, http.StatusInternalServerError)
			return
		}
		writeJSON(w, r, map[string]string{"status": "ok", "message": "episodes grouped successfully"})

	default:
		s.methodNotAllowed(w)
	}
}

func (s *Server) handleTVShowDetail(w http.ResponseWriter, r *http.Request) {
	if s.handleOptions(w, r, "GET, DELETE, OPTIONS") {
		return
	}
	if s.lib.store == nil {
		s.writeError(w, "not available without database", http.StatusNotImplemented)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/shows/")
	parts := strings.Split(path, "/")
	if len(parts) < 1 || parts[0] == "" {
		s.writeError(w, errNotFound, http.StatusNotFound)
		return
	}

	showID := parts[0]
	action := ""
	if len(parts) >= 2 {
		action = parts[1]
	}

	if action == "seasons" {
		// /shows/{id}/seasons
		if r.Method != http.MethodGet {
			s.methodNotAllowed(w)
			return
		}

		seasons, err := s.lib.store.GetSeasonsByShow(showID)
		if err != nil {
			s.writeError(w, errInternal, http.StatusInternalServerError)
			return
		}
		writeJSON(w, r, seasons)
		return
	}

	if action == "next-episode" {
		// /shows/{id}/next-episode?userId=xyz
		if r.Method != http.MethodGet {
			s.methodNotAllowed(w)
			return
		}

		userID := strings.TrimSpace(r.URL.Query().Get("userId"))
		if userID == "" {
			userID = "" // Default user
		}

		episode, ok, err := s.lib.store.GetNextUnwatchedEpisode(showID, userID)
		if err != nil {
			s.writeError(w, errInternal, http.StatusInternalServerError)
			return
		}
		if !ok {
			s.writeError(w, errNotFound, http.StatusNotFound)
			return
		}
		writeJSON(w, r, episode)
		return
	}

	// /shows/{id}
	switch r.Method {
	case http.MethodGet:
		show, ok, err := s.lib.store.GetTVShow(showID)
		if err != nil {
			s.writeError(w, errInternal, http.StatusInternalServerError)
			return
		}
		if !ok {
			s.writeError(w, errNotFound, http.StatusNotFound)
			return
		}
		writeJSON(w, r, show)

	case http.MethodDelete:
		if s.readOnly {
			s.writeError(w, "read-only mode", http.StatusForbidden)
			return
		}

		if err := s.lib.store.DeleteTVShow(showID); err != nil {
			s.writeError(w, errInternal, http.StatusInternalServerError)
			return
		}
		writeJSON(w, r, map[string]string{"status": "ok"})

	default:
		s.methodNotAllowed(w)
	}
}

func (s *Server) handleSeasonDetail(w http.ResponseWriter, r *http.Request) {
	if s.handleOptions(w, r, "GET, OPTIONS") {
		return
	}
	if s.lib.store == nil {
		s.writeError(w, "not available without database", http.StatusNotImplemented)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/shows/")
	parts := strings.Split(path, "/")
	if len(parts) < 4 || parts[0] == "" || parts[1] != "seasons" || parts[2] == "" {
		s.writeError(w, errNotFound, http.StatusNotFound)
		return
	}

	showID := parts[0]
	seasonNumStr := parts[2]
	action := ""
	if len(parts) >= 4 {
		action = parts[3]
	}

	seasonNum, err := strconv.Atoi(seasonNumStr)
	if err != nil {
		s.writeError(w, "bad request", http.StatusBadRequest)
		return
	}

	// Get all seasons for this show to find the right one
	seasons, err := s.lib.store.GetSeasonsByShow(showID)
	if err != nil {
		s.writeError(w, errInternal, http.StatusInternalServerError)
		return
	}

	var season *Season
	for _, s := range seasons {
		if s.SeasonNumber == seasonNum {
			season = &s
			break
		}
	}

	if season == nil {
		s.writeError(w, errNotFound, http.StatusNotFound)
		return
	}

	if action == "episodes" {
		// /shows/{id}/seasons/{season}/episodes
		if r.Method != http.MethodGet {
			s.methodNotAllowed(w)
			return
		}

		episodes, err := s.lib.store.GetEpisodesBySeason(season.ID)
		if err != nil {
			s.writeError(w, errInternal, http.StatusInternalServerError)
			return
		}
		writeJSON(w, r, episodes)
		return
	}

	// /shows/{id}/seasons/{season}
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w)
		return
	}
	writeJSON(w, r, season)
}

// Helper functions

func generateUserID(name string) string {
	sum := sha1.Sum([]byte("user:" + strings.ToLower(name)))
	return "user_" + hex.EncodeToString(sum[:8])
}

func generateProfileID(name string) string {
	sum := sha1.Sum([]byte("profile:" + strings.ToLower(name)))
	return "profile_" + hex.EncodeToString(sum[:8])
}

// ============================================================================
// Verbesserung 2: Transkodierung - Stream-Endpoints
// ============================================================================

func (s *Server) handleTranscodedStream(w http.ResponseWriter, r *http.Request, item MediaItem, profileName string) {
	if s.lib.store == nil {
		s.writeError(w, "transcoding not available without database", http.StatusNotImplemented)
		return
	}

	// Get profile by name
	profile, ok, err := s.lib.store.GetTranscodingProfileByName(profileName)
	if err != nil {
		s.writeError(w, errInternal, http.StatusInternalServerError)
		return
	}
	if !ok {
		s.writeError(w, "profile not found", http.StatusNotFound)
		return
	}

	// Check if already cached
	cached, ok, err := s.lib.store.GetTranscodingCache(item.ID, profile.ID)
	if err == nil && ok {
		// Serve from cache
		s.transcodingMgr.ServeTranscodedFile(w, r, cached.CachePath)
		return
	}

	// Start transcoding job
	job, err := s.transcodingMgr.StartTranscoding(item.ID, profile.ID, item, *profile)
	if err != nil {
		s.writeError(w, errInternal, http.StatusInternalServerError)
		return
	}

	// If job is completed, serve the file
	if job.Status == "completed" && job.OutputPath != "" {
		s.transcodingMgr.ServeTranscodedFile(w, r, job.OutputPath)
		return
	}

	// Job is still running, return status
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]any{
		"status":   job.Status,
		"progress": job.Progress,
		"jobId":    job.ID,
		"message":  "transcoding in progress, please retry in a few seconds",
	})
}

func (s *Server) handleHLSStream(w http.ResponseWriter, r *http.Request, item MediaItem, profileName string) {
	if s.lib.store == nil {
		s.writeError(w, "HLS not available without database", http.StatusNotImplemented)
		return
	}

	// Get profile by name
	profile, ok, err := s.lib.store.GetTranscodingProfileByName(profileName)
	if err != nil {
		s.writeError(w, errInternal, http.StatusInternalServerError)
		return
	}
	if !ok {
		s.writeError(w, "profile not found", http.StatusNotFound)
		return
	}

	// Start HLS transcoding job
	job, err := s.transcodingMgr.StartHLSTranscoding(item.ID, profile.ID, item, *profile)
	if err != nil {
		s.writeError(w, errInternal, http.StatusInternalServerError)
		return
	}

	// If job is completed, serve the playlist
	if job.Status == "completed" && job.OutputPath != "" {
		s.transcodingMgr.ServeHLSPlaylist(w, r, job.OutputPath)
		return
	}

	// Job is still running, return status
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]any{
		"status":   job.Status,
		"progress": job.Progress,
		"jobId":    job.ID,
		"message":  "HLS transcoding in progress, please retry in a few seconds",
	})
}

func (s *Server) handleTranscodingJobs(w http.ResponseWriter, r *http.Request) {
	if s.handleOptions(w, r, "GET, OPTIONS") {
		return
	}
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w)
		return
	}

	// Return all active jobs (simplified - in production, store jobs in DB)
	writeJSON(w, r, map[string]string{
		"message": "transcoding jobs endpoint - implementation pending",
	})
}
