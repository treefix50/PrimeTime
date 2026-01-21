package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/treefix50/primetime/internal/auth"
)

const (
	errInternal         = "internal server error"
	errNotFound         = "not found"
	errMethodNotAllowed = "method not allowed"
	manualScanRateLimit = 30 * time.Second
	playbackProgressMin = 5 * time.Second
)

type Server struct {
	addr              string
	lib               *Library
	http              *http.Server
	cors              bool
	jsonErrors        bool
	readOnly          bool
	allowReadOnlyScan bool
	ffmpegReady       bool
	ffmpegPath        string
	startedAt         time.Time
	version           VersionInfo
	scanInterval      time.Duration
	scanTicker        *time.Ticker
	scanStop          chan struct{}
	scanStopOnce      sync.Once
	scanWg            sync.WaitGroup
	manualScanLimiter *RateLimiter
	playbackLimiter   *RateLimiter
	transcodingMgr    *TranscodingManager
	authManager       *auth.Manager
}

func (s *Server) methodNotAllowed(w http.ResponseWriter) {
	s.writeError(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
}

type VersionInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"buildDate"`
}

func New(root, addr string, store MediaStore, scanInterval time.Duration, noInitialScan bool, cors bool, jsonErrors bool, version VersionInfo, ffmpegReady bool, allowReadOnlyScan bool, extensions []string, ffmpegPath string) (*Server, error) {
	lib, err := NewLibrary(root, store, extensions)
	if err != nil {
		return nil, err
	}
	readOnly := storeReadOnly(store)
	allowScan := !readOnly || allowReadOnlyScan
	if allowScan && !noInitialScan {
		// initial scan
		if err := lib.Scan(); err != nil {
			log.Printf("scan failed (initial): %v", err)
			return nil, err
		}
	}

	mux := http.NewServeMux()

	// Initialize transcoding manager
	cacheDir := filepath.Join(root, "..", "cache", "transcoding")
	transcodingMgr := NewTranscodingManager(ffmpegPath, cacheDir, store)

	// Initialize auth manager
	var authMgr *auth.Manager
	if store != nil {
		// Type assert to auth.Store - the storage.Store implements all required methods
		if authStore, ok := store.(auth.Store); ok {
			authMgr = auth.NewManager(authStore, 24*time.Hour) // 24 hour sessions
		}
	}

	s := &Server{
		addr:              addr,
		lib:               lib,
		cors:              cors,
		jsonErrors:        jsonErrors,
		readOnly:          readOnly,
		allowReadOnlyScan: allowReadOnlyScan,
		ffmpegReady:       ffmpegReady,
		ffmpegPath:        ffmpegPath,
		startedAt:         time.Now(),
		version:           version,
		scanInterval:      scanInterval,
		manualScanLimiter: NewRateLimiter(manualScanRateLimit),
		playbackLimiter:   NewRateLimiter(playbackProgressMin),
		transcodingMgr:    transcodingMgr,
		authManager:       authMgr,
	}

	if s.scanInterval > 0 && (!s.readOnly || s.allowReadOnlyScan) {
		s.scanTicker = time.NewTicker(s.scanInterval)
		s.scanStop = make(chan struct{})
		s.scanWg.Add(1)
		go s.runScanTicker()
	}

	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/stats", s.handleStats)
	mux.HandleFunc("/version", s.handleVersion)
	mux.HandleFunc("/library", s.handleLibrary)
	mux.HandleFunc("/library/scan", s.handleLibraryScan)
	mux.HandleFunc("/library/duplicates", s.handleLibraryDuplicates)
	mux.HandleFunc("/library/recent", s.handleLibraryRecent)
	mux.HandleFunc("/library/roots", s.handleLibraryRoots)
	mux.HandleFunc("/library/roots/", s.handleLibraryRootScan)
	mux.HandleFunc("/library/type/", s.handleLibraryByType)
	mux.HandleFunc("/playback", s.handlePlayback)
	mux.HandleFunc("/favorites", s.handleFavorites)
	mux.HandleFunc("/watched", s.handleWatched)
	mux.HandleFunc("/collections", s.handleCollections)
	mux.HandleFunc("/collections/", s.handleCollectionDetail)
	mux.HandleFunc("/items/", s.handleItems)

	// Verbesserung 1: Multi-User-Support
	mux.HandleFunc("/users", s.handleUsers)
	mux.HandleFunc("/users/", s.handleUserDetail)

	// Verbesserung 2: Transkodierungs-Profile
	mux.HandleFunc("/transcoding/profiles", s.handleTranscodingProfiles)
	mux.HandleFunc("/transcoding/profiles/", s.handleTranscodingProfileDetail)

	// Verbesserung 3: TV Shows/Serien-Verwaltung
	mux.HandleFunc("/shows", s.handleTVShows)
	mux.HandleFunc("/shows/", s.handleTVShowDetail)

	// Authentication endpoints
	mux.HandleFunc("/auth/login", s.handleAuthLogin)
	mux.HandleFunc("/auth/logout", s.handleAuthLogout)
	mux.HandleFunc("/auth/session", s.handleAuthSession)
	mux.HandleFunc("/auth/users", s.handleAuthUsers)
	mux.HandleFunc("/auth/users/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/password") {
			s.handleAuthUserPassword(w, r)
		} else {
			s.handleAuthUserDelete(w, r)
		}
	})

	s.http = &http.Server{
		Addr:              addr,
		Handler:           logMiddleware(mux, s.cors),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return s, nil
}

func (s *Server) Start() error { return s.http.ListenAndServe() }

func (s *Server) Close() error {
	s.stopScanTicker()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return s.http.Shutdown(ctx)
}

func (s *Server) runScanTicker() {
	defer s.scanWg.Done()
	for {
		select {
		case <-s.scanTicker.C:
			if err := s.lib.Scan(); err != nil {
				log.Printf("scan failed (periodic): %v", err)
			}
		case <-s.scanStop:
			s.scanTicker.Stop()
			return
		}
	}
}

func (s *Server) stopScanTicker() {
	if s.scanStop == nil {
		return
	}
	s.scanStopOnce.Do(func() {
		close(s.scanStop)
	})
	s.scanWg.Wait()
	s.scanStop = nil
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if s.handleOptions(w, r, "GET, OPTIONS") {
		return
	}
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w)
		return
	}
	if strings.TrimSpace(r.URL.Query().Get("json")) != "" {
		writeJSON(w, r, map[string]any{
			"db": map[string]any{
				"connected": s.lib.store != nil,
				"readOnly":  s.readOnly,
			},
			"ffmpeg": map[string]any{
				"ready": s.ffmpegReady,
			},
			"uptime": int64(time.Since(s.startedAt).Seconds()),
		})
		return
	}
	w.Header().Set("Content-Type", textContentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	if s.handleOptions(w, r, "GET, OPTIONS") {
		return
	}
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w)
		return
	}
	writeJSON(w, r, s.version)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if s.handleOptions(w, r, "GET, OPTIONS") {
		return
	}
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w)
		return
	}

	// Verbesserung 5: Erweiterte Statistiken
	detailed := strings.TrimSpace(r.URL.Query().Get("detailed"))
	if detailed != "" && s.lib.store != nil {
		stats, err := s.lib.store.GetDetailedStats()
		if err != nil {
			s.writeError(w, errInternal, http.StatusInternalServerError)
			return
		}
		writeJSON(w, r, stats)
		return
	}

	totalItems, lastScan, err := s.lib.Stats()
	if err != nil {
		s.writeError(w, errInternal, http.StatusInternalServerError)
		return
	}
	writeJSON(w, r, map[string]any{
		"totalItems": totalItems,
		"lastScan":   lastScan,
	})
}

func (s *Server) handleLibrary(w http.ResponseWriter, r *http.Request) {
	if s.handleOptions(w, r, "GET, POST, OPTIONS") {
		return
	}

	switch r.Method {
	case http.MethodGet:
		query := strings.TrimSpace(r.URL.Query().Get("q"))
		sortBy := normalizeSortBy(r.URL.Query().Get("sort"))
		limit, offset, ok := parseLimitOffset(r)
		if !ok {
			s.writeError(w, "bad request", http.StatusBadRequest)
			return
		}

		// Verbesserung 3: Erweiterte Suchfunktionalität
		genre := strings.TrimSpace(r.URL.Query().Get("genre"))
		year := strings.TrimSpace(r.URL.Query().Get("year"))
		itemType := strings.TrimSpace(r.URL.Query().Get("type"))
		rating := strings.TrimSpace(r.URL.Query().Get("rating"))

		if s.lib.store == nil {
			items := s.lib.All()
			if query != "" {
				items = filterItems(items, query)
			}
			sortItems(items, sortBy)
			items = applyLimitOffset(items, limit, offset)
			writeJSON(w, r, items)
			return
		}

		items, err := s.lib.store.GetAllLimitedWithFilters(limit, offset, sortBy, query, genre, year, itemType, rating)
		if err != nil {
			s.writeError(w, errInternal, http.StatusInternalServerError)
			return
		}
		writeJSON(w, r, items)
	case http.MethodPost:
		if s.readOnly && !s.allowReadOnlyScan {
			s.writeError(w, "read-only mode", http.StatusForbidden)
			return
		}
		if ok, wait := s.allowManualScan(); !ok {
			s.writeError(w, manualScanError(wait), http.StatusTooManyRequests)
			return
		}
		if err := s.lib.Scan(); err != nil {
			s.writeError(w, errInternal, http.StatusInternalServerError)
			return
		}
		writeJSON(w, r, map[string]string{"status": "ok"})
	default:
		s.methodNotAllowed(w)
	}
}

func (s *Server) handleLibraryScan(w http.ResponseWriter, r *http.Request) {
	if s.handleOptions(w, r, "POST, OPTIONS") {
		return
	}
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	if s.readOnly && !s.allowReadOnlyScan {
		s.writeError(w, "read-only mode", http.StatusForbidden)
		return
	}
	if ok, wait := s.allowManualScan(); !ok {
		s.writeError(w, manualScanError(wait), http.StatusTooManyRequests)
		return
	}

	var payload struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		s.writeError(w, "bad request", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(payload.Path) == "" {
		s.writeError(w, "bad request", http.StatusBadRequest)
		return
	}

	if err := s.lib.ScanPath(payload.Path); err != nil {
		if errors.Is(err, ErrInvalidScanPath) || errors.Is(err, ErrScanPathNotFound) {
			s.writeError(w, "bad request", http.StatusBadRequest)
			return
		}
		s.writeError(w, errInternal, http.StatusInternalServerError)
		return
	}
	writeJSON(w, r, map[string]string{"status": "ok"})
}

func (s *Server) allowManualScan() (bool, time.Duration) {
	return s.manualScanLimiter.Allow("manual-scan")
}

func manualScanError(wait time.Duration) string {
	waitSeconds := int(wait.Seconds())
	if waitSeconds < 1 {
		waitSeconds = 1
	}
	return fmt.Sprintf("rescan zu früh, bitte in %ds erneut versuchen", waitSeconds)
}

func playbackRateLimitError(wait time.Duration) string {
	waitSeconds := int(wait.Seconds())
	if waitSeconds < 1 {
		waitSeconds = 1
	}
	return fmt.Sprintf("playback update zu früh, bitte in %ds erneut versuchen", waitSeconds)
}

// Verbesserung 4: Duplicate Detection
func (s *Server) handleLibraryDuplicates(w http.ResponseWriter, r *http.Request) {
	if s.handleOptions(w, r, "GET, OPTIONS") {
		return
	}
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w)
		return
	}
	if s.lib.store == nil {
		s.writeError(w, "not available without database", http.StatusNotImplemented)
		return
	}

	duplicates, err := s.lib.store.GetDuplicates()
	if err != nil {
		s.writeError(w, errInternal, http.StatusInternalServerError)
		return
	}
	writeJSON(w, r, duplicates)
}

// Verbesserung 2: Batch-Operations für Playback-State
func (s *Server) handlePlayback(w http.ResponseWriter, r *http.Request) {
	if s.handleOptions(w, r, "GET, OPTIONS") {
		return
	}
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w)
		return
	}
	if s.lib.store == nil {
		s.writeError(w, "not available without database", http.StatusNotImplemented)
		return
	}

	clientID := strings.TrimSpace(r.URL.Query().Get("clientId"))
	onlyUnfinished := strings.TrimSpace(r.URL.Query().Get("unfinished")) != ""

	states, err := s.lib.store.GetAllPlaybackStates(clientID, onlyUnfinished)
	if err != nil {
		s.writeError(w, errInternal, http.StatusInternalServerError)
		return
	}
	writeJSON(w, r, states)
}

// Routes under /items/{id}[/{action}...]
func (s *Server) handleItems(w http.ResponseWriter, r *http.Request) {
	if s.handleOptions(w, r, "GET, POST, DELETE, OPTIONS") {
		return
	}

	if r.Method != http.MethodGet && r.Method != http.MethodPost && r.Method != http.MethodDelete {
		s.methodNotAllowed(w)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/items/")
	parts := strings.Split(path, "/")
	if len(parts) < 1 || parts[0] == "" {
		s.writeError(w, errNotFound, http.StatusNotFound)
		return
	}

	id := parts[0]
	action := ""
	if len(parts) >= 2 {
		action = parts[1]
	}

	if action == "exists" {
		if r.Method != http.MethodGet {
			s.methodNotAllowed(w)
			return
		}
		var ok bool
		if s.lib.store == nil {
			_, ok = s.lib.Get(id)
		} else {
			var err error
			_, ok, err = s.lib.store.GetByID(id)
			if err != nil {
				s.writeError(w, errInternal, http.StatusInternalServerError)
				return
			}
		}
		if ok {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var item MediaItem
	var ok bool
	if s.lib.store == nil {
		item, ok = s.lib.Get(id)
	} else {
		var err error
		item, ok, err = s.lib.store.GetByID(id)
		if err != nil {
			s.writeError(w, errInternal, http.StatusInternalServerError)
			return
		}
	}
	if !ok {
		s.writeError(w, errNotFound, http.StatusNotFound)
		return
	}

	switch action {
	case "":
		// /items/{id}
		if r.Method != http.MethodGet {
			s.methodNotAllowed(w)
			return
		}
		writeJSON(w, r, item)

	case "stream":
		// /items/{id}/stream  OR  /items/{id}/stream.m3u8
		if r.Method != http.MethodGet {
			s.methodNotAllowed(w)
			return
		}

		// Check for HLS request
		if len(parts) >= 2 && strings.HasSuffix(parts[1], ".m3u8") {
			profileName := strings.TrimSpace(r.URL.Query().Get("profile"))
			if profileName == "" {
				profileName = "720p" // Default profile for HLS
			}
			s.handleHLSStream(w, r, item, profileName)
			return
		}

		// Check for transcoding profile parameter
		profileName := strings.TrimSpace(r.URL.Query().Get("profile"))
		if profileName != "" && profileName != "original" {
			s.handleTranscodedStream(w, r, item, profileName)
			return
		}

		// Serve original file
		ServeVideoFile(w, r, item.VideoPath)

	case "nfo":
		// /items/{id}/nfo  OR  /items/{id}/nfo/raw
		if r.Method != http.MethodGet {
			s.methodNotAllowed(w)
			return
		}
		if len(parts) == 2 {
			if s.lib.store != nil {
				// Use extended NFO retrieval for complete metadata
				nfo, ok, err := s.lib.store.GetNFOExtended(item.ID)
				if err != nil {
					s.writeError(w, errInternal, http.StatusInternalServerError)
					return
				}
				if ok && nfo != nil {
					writeJSON(w, r, nfo)
					return
				}
			}

			if item.NFOPath != "" {
				nfo, err := ParseNFOFile(item.NFOPath)
				if err == nil {
					writeJSON(w, r, nfo)
					return
				}
			}

			if fallback, ok := fallbackNFOFromFilename(item.VideoPath); ok {
				writeJSON(w, r, fallback)
				return
			}
			s.writeError(w, errNotFound, http.StatusNotFound)
			return
		}

		if len(parts) == 3 && parts[2] == "raw" {
			if item.NFOPath == "" {
				s.writeError(w, errNotFound, http.StatusNotFound)
				return
			}
			ServeTextFile(w, r, item.NFOPath, "text/xml; charset=utf-8")
			return
		}

		s.writeError(w, errNotFound, http.StatusNotFound)

	case "subtitles":
		// /items/{id}/subtitles
		if r.Method != http.MethodGet {
			s.methodNotAllowed(w)
			return
		}
		if len(parts) != 2 {
			s.writeError(w, errNotFound, http.StatusNotFound)
			return
		}
		subtitlePath, contentType := subtitlePathForVideo(item.VideoPath)
		if subtitlePath == "" {
			s.writeError(w, errNotFound, http.StatusNotFound)
			return
		}
		ServeTextFile(w, r, subtitlePath, contentType)

	case "playback":
		if s.lib.store == nil {
			s.writeError(w, errNotFound, http.StatusNotFound)
			return
		}

		switch r.Method {
		case http.MethodGet:
			clientID := strings.TrimSpace(r.URL.Query().Get("clientId"))
			state, ok, err := s.lib.store.GetPlaybackState(item.ID, clientID)
			if err != nil {
				s.writeError(w, errInternal, http.StatusInternalServerError)
				return
			}
			if !ok || state == nil {
				s.writeError(w, errNotFound, http.StatusNotFound)
				return
			}
			writeJSON(w, r, state)
		case http.MethodPost:
			if s.readOnly {
				s.writeError(w, "read-only mode", http.StatusForbidden)
				return
			}
			var payload PlaybackEvent
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				s.writeError(w, "bad request", http.StatusBadRequest)
				return
			}

			event := strings.ToLower(strings.TrimSpace(payload.Event))
			if event == "" {
				event = "progress"
			}
			if event != "progress" && event != "stop" {
				s.writeError(w, "bad request", http.StatusBadRequest)
				return
			}

			clientID := strings.TrimSpace(payload.ClientID)
			position := payload.PositionSeconds
			duration := payload.DurationSeconds
			lastPlayedAt := payload.LastPlayedAt
			if lastPlayedAt <= 0 {
				lastPlayedAt = time.Now().Unix()
			}

			shouldDelete := position <= 0 || duration <= 0 || (event == "stop" && position >= duration)
			if shouldDelete {
				if err := s.lib.store.DeletePlaybackState(item.ID, clientID); err != nil {
					s.writeError(w, errInternal, http.StatusInternalServerError)
					return
				}
				writeJSON(w, r, map[string]string{"status": "ok"})
				return
			}

			if event == "progress" {
				key := item.ID + "|" + clientID
				if ok, wait := s.playbackLimiter.Allow(key); !ok {
					s.writeError(w, playbackRateLimitError(wait), http.StatusTooManyRequests)
					return
				}
			}

			if err := s.lib.store.UpsertPlaybackState(item.ID, position, duration, lastPlayedAt, payload.PercentComplete, clientID); err != nil {
				s.writeError(w, errInternal, http.StatusInternalServerError)
				return
			}
			writeJSON(w, r, map[string]string{"status": "ok"})
		default:
			s.methodNotAllowed(w)
		}

	case "watched":
		// /items/{id}/watched
		if s.lib.store == nil {
			s.writeError(w, errNotFound, http.StatusNotFound)
			return
		}

		switch r.Method {
		case http.MethodGet:
			watched, err := s.lib.store.IsWatched(item.ID)
			if err != nil {
				s.writeError(w, errInternal, http.StatusInternalServerError)
				return
			}
			writeJSON(w, r, map[string]bool{"watched": watched})
		case http.MethodPost:
			if s.readOnly {
				s.writeError(w, "read-only mode", http.StatusForbidden)
				return
			}
			if err := s.lib.store.MarkWatched(item.ID, time.Now()); err != nil {
				s.writeError(w, errInternal, http.StatusInternalServerError)
				return
			}
			writeJSON(w, r, map[string]string{"status": "ok"})
		case http.MethodDelete:
			if s.readOnly {
				s.writeError(w, "read-only mode", http.StatusForbidden)
				return
			}
			if err := s.lib.store.UnmarkWatched(item.ID); err != nil {
				s.writeError(w, errInternal, http.StatusInternalServerError)
				return
			}
			writeJSON(w, r, map[string]string{"status": "ok"})
		default:
			s.methodNotAllowed(w)
		}

	case "favorite":
		// /items/{id}/favorite
		if s.lib.store == nil {
			s.writeError(w, errNotFound, http.StatusNotFound)
			return
		}

		switch r.Method {
		case http.MethodGet:
			favorite, err := s.lib.store.IsFavorite(item.ID)
			if err != nil {
				s.writeError(w, errInternal, http.StatusInternalServerError)
				return
			}
			writeJSON(w, r, map[string]bool{"favorite": favorite})
		case http.MethodPost:
			if s.readOnly {
				s.writeError(w, "read-only mode", http.StatusForbidden)
				return
			}
			if err := s.lib.store.AddFavorite(item.ID, time.Now()); err != nil {
				s.writeError(w, errInternal, http.StatusInternalServerError)
				return
			}
			writeJSON(w, r, map[string]string{"status": "ok"})
		case http.MethodDelete:
			if s.readOnly {
				s.writeError(w, "read-only mode", http.StatusForbidden)
				return
			}
			if err := s.lib.store.RemoveFavorite(item.ID); err != nil {
				s.writeError(w, errInternal, http.StatusInternalServerError)
				return
			}
			writeJSON(w, r, map[string]string{"status": "ok"})
		default:
			s.methodNotAllowed(w)
		}

	case "poster":
		// /items/{id}/poster  OR  /items/{id}/poster/exists
		if r.Method != http.MethodGet {
			s.methodNotAllowed(w)
			return
		}

		if len(parts) == 3 && parts[2] == "exists" {
			var exists bool
			if s.lib.store != nil {
				posterPath, ok, err := s.lib.store.GetPosterPath(item.ID)
				if err != nil {
					s.writeError(w, errInternal, http.StatusInternalServerError)
					return
				}
				exists = ok && posterPath != ""
			}
			if !exists {
				posterPath, exists := FindPosterForVideo(item.VideoPath)
				if exists && s.lib.store != nil && !s.readOnly {
					_ = s.lib.store.SetPosterPath(item.ID, posterPath)
				}
			}
			writeJSON(w, r, map[string]bool{"exists": exists})
			return
		}

		if len(parts) != 2 {
			s.writeError(w, errNotFound, http.StatusNotFound)
			return
		}

		var posterPath string
		if s.lib.store != nil {
			path, ok, err := s.lib.store.GetPosterPath(item.ID)
			if err != nil {
				s.writeError(w, errInternal, http.StatusInternalServerError)
				return
			}
			if ok && path != "" {
				posterPath = path
			}
		}

		if posterPath == "" {
			path, ok := FindPosterForVideo(item.VideoPath)
			if !ok {
				s.writeError(w, errNotFound, http.StatusNotFound)
				return
			}
			posterPath = path
			if s.lib.store != nil && !s.readOnly {
				_ = s.lib.store.SetPosterPath(item.ID, posterPath)
			}
		}

		contentType := GetPosterContentType(posterPath)
		w.Header().Set("Content-Type", contentType)
		http.ServeFile(w, r, posterPath)

	default:
		s.writeError(w, errNotFound, http.StatusNotFound)
	}
}

func subtitlePathForVideo(videoPath string) (string, string) {
	base := strings.TrimSuffix(videoPath, filepath.Ext(videoPath))
	if base == "" {
		return "", ""
	}
	candidates := []struct {
		ext         string
		contentType string
	}{
		{ext: ".vtt", contentType: "text/vtt; charset=utf-8"},
		{ext: ".srt", contentType: "application/x-subrip; charset=utf-8"},
	}
	for _, candidate := range candidates {
		path := base + candidate.ext
		if _, err := os.Stat(path); err == nil {
			return path, candidate.contentType
		}
	}
	return "", ""
}

func writeJSON(w http.ResponseWriter, r *http.Request, v any) {
	w.Header().Set("Content-Type", jsonContentType)
	var buf strings.Builder
	encoder := json.NewEncoder(&buf)
	if err := encoder.Encode(v); err != nil {
		log.Printf("json encode failed for %s %s: %v", r.Method, r.URL.Path, err)
		if sw, ok := w.(interface{ Written() bool }); ok && sw.Written() {
			return
		}
		http.Error(w, errInternal, http.StatusInternalServerError)
		return
	}
	if sw, ok := w.(interface{ Written() bool }); ok && !sw.Written() {
		w.WriteHeader(http.StatusOK)
	}
	_, _ = w.Write([]byte(buf.String()))
}

func (s *Server) writePreflightHeaders(w http.ResponseWriter, methods string) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", methods)
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}

func (s *Server) writeError(w http.ResponseWriter, message string, code int) {
	setCORSHeaders(w, s.cors)
	if s.jsonErrors {
		w.Header().Set("Content-Type", jsonContentType)
		w.WriteHeader(code)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": message}); err != nil {
			log.Printf("json encode failed for error response: %v", err)
		}
		return
	}
	http.Error(w, message, code)
}

func (s *Server) handleOptions(w http.ResponseWriter, r *http.Request, methods string) bool {
	if r.Method != http.MethodOptions {
		return false
	}
	if s.cors {
		s.writePreflightHeaders(w, methods)
		w.WriteHeader(http.StatusOK)
		return true
	}
	s.methodNotAllowed(w)
	return true
}

func filterItems(items []MediaItem, query string) []MediaItem {
	normalized := strings.ToLower(query)
	filtered := make([]MediaItem, 0, len(items))
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.Title), normalized) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func applyLimitOffset(items []MediaItem, limit, offset int) []MediaItem {
	if offset > 0 {
		if offset >= len(items) {
			return []MediaItem{}
		}
		items = items[offset:]
	}
	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}
	return items
}

func parseLimitOffset(r *http.Request) (int, int, bool) {
	limit := 0
	offset := 0
	limitRaw := strings.TrimSpace(r.URL.Query().Get("limit"))
	if limitRaw != "" {
		parsed, err := strconv.Atoi(limitRaw)
		if err != nil || parsed < 0 {
			return 0, 0, false
		}
		limit = parsed
	}
	offsetRaw := strings.TrimSpace(r.URL.Query().Get("offset"))
	if offsetRaw != "" {
		parsed, err := strconv.Atoi(offsetRaw)
		if err != nil || parsed < 0 {
			return 0, 0, false
		}
		offset = parsed
	}
	return limit, offset, true
}

// Erweiterung 3: Recently Added
func (s *Server) handleLibraryRecent(w http.ResponseWriter, r *http.Request) {
	if s.handleOptions(w, r, "GET, OPTIONS") {
		return
	}
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w)
		return
	}
	if s.lib.store == nil {
		s.writeError(w, "not available without database", http.StatusNotImplemented)
		return
	}

	limit := 20
	limitRaw := strings.TrimSpace(r.URL.Query().Get("limit"))
	if limitRaw != "" {
		if parsed, err := strconv.Atoi(limitRaw); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	days := 0
	daysRaw := strings.TrimSpace(r.URL.Query().Get("days"))
	if daysRaw != "" {
		if parsed, err := strconv.Atoi(daysRaw); err == nil && parsed > 0 {
			days = parsed
		}
	}

	itemType := strings.TrimSpace(r.URL.Query().Get("type"))

	items, err := s.lib.store.GetRecentlyAdded(limit, days, itemType)
	if err != nil {
		s.writeError(w, errInternal, http.StatusInternalServerError)
		return
	}
	writeJSON(w, r, items)
}

// Erweiterung 2: Favorites
func (s *Server) handleFavorites(w http.ResponseWriter, r *http.Request) {
	if s.handleOptions(w, r, "GET, OPTIONS") {
		return
	}
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w)
		return
	}
	if s.lib.store == nil {
		s.writeError(w, "not available without database", http.StatusNotImplemented)
		return
	}

	limit, offset, ok := parseLimitOffset(r)
	if !ok {
		s.writeError(w, "bad request", http.StatusBadRequest)
		return
	}

	items, err := s.lib.store.GetFavorites(limit, offset)
	if err != nil {
		s.writeError(w, errInternal, http.StatusInternalServerError)
		return
	}
	writeJSON(w, r, items)
}

// Erweiterung 1: Watched
func (s *Server) handleWatched(w http.ResponseWriter, r *http.Request) {
	if s.handleOptions(w, r, "GET, OPTIONS") {
		return
	}
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w)
		return
	}
	if s.lib.store == nil {
		s.writeError(w, "not available without database", http.StatusNotImplemented)
		return
	}

	limit, offset, ok := parseLimitOffset(r)
	if !ok {
		s.writeError(w, "bad request", http.StatusBadRequest)
		return
	}

	items, err := s.lib.store.GetWatchedItems(limit, offset)
	if err != nil {
		s.writeError(w, errInternal, http.StatusInternalServerError)
		return
	}
	writeJSON(w, r, items)
}

// Erweiterung 4: Collections
func (s *Server) handleCollections(w http.ResponseWriter, r *http.Request) {
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

		collections, err := s.lib.store.GetCollections(limit, offset)
		if err != nil {
			s.writeError(w, errInternal, http.StatusInternalServerError)
			return
		}
		writeJSON(w, r, collections)

	case http.MethodPost:
		if s.readOnly {
			s.writeError(w, "read-only mode", http.StatusForbidden)
			return
		}

		var payload struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			s.writeError(w, "bad request", http.StatusBadRequest)
			return
		}

		if strings.TrimSpace(payload.Name) == "" {
			s.writeError(w, "name is required", http.StatusBadRequest)
			return
		}

		id := newCollectionID()
		if err := s.lib.store.CreateCollection(id, payload.Name, payload.Description, time.Now()); err != nil {
			s.writeError(w, errInternal, http.StatusInternalServerError)
			return
		}

		collection, ok, err := s.lib.store.GetCollection(id)
		if err != nil {
			s.writeError(w, errInternal, http.StatusInternalServerError)
			return
		}
		if !ok {
			s.writeError(w, errNotFound, http.StatusNotFound)
			return
		}
		writeJSON(w, r, collection)

	default:
		s.methodNotAllowed(w)
	}
}

func (s *Server) handleCollectionDetail(w http.ResponseWriter, r *http.Request) {
	if s.handleOptions(w, r, "GET, PUT, DELETE, POST, OPTIONS") {
		return
	}
	if s.lib.store == nil {
		s.writeError(w, "not available without database", http.StatusNotImplemented)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/collections/")
	parts := strings.Split(path, "/")
	if len(parts) < 1 || parts[0] == "" {
		s.writeError(w, errNotFound, http.StatusNotFound)
		return
	}

	collectionID := parts[0]
	action := ""
	if len(parts) >= 2 {
		action = parts[1]
	}

	if action == "items" {
		// /collections/{id}/items
		switch r.Method {
		case http.MethodGet:
			items, err := s.lib.store.GetCollectionItems(collectionID)
			if err != nil {
				s.writeError(w, errInternal, http.StatusInternalServerError)
				return
			}
			writeJSON(w, r, items)

		case http.MethodPost:
			if s.readOnly {
				s.writeError(w, "read-only mode", http.StatusForbidden)
				return
			}

			var payload struct {
				MediaID  string `json:"mediaId"`
				Position int    `json:"position"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				s.writeError(w, "bad request", http.StatusBadRequest)
				return
			}

			if strings.TrimSpace(payload.MediaID) == "" {
				s.writeError(w, "mediaId is required", http.StatusBadRequest)
				return
			}

			if err := s.lib.store.AddItemToCollection(collectionID, payload.MediaID, payload.Position, time.Now()); err != nil {
				s.writeError(w, errInternal, http.StatusInternalServerError)
				return
			}
			writeJSON(w, r, map[string]string{"status": "ok"})

		case http.MethodDelete:
			if s.readOnly {
				s.writeError(w, "read-only mode", http.StatusForbidden)
				return
			}

			if len(parts) < 3 {
				s.writeError(w, "mediaId is required", http.StatusBadRequest)
				return
			}

			mediaID := parts[2]
			if err := s.lib.store.RemoveItemFromCollection(collectionID, mediaID); err != nil {
				s.writeError(w, errInternal, http.StatusInternalServerError)
				return
			}
			writeJSON(w, r, map[string]string{"status": "ok"})

		default:
			s.methodNotAllowed(w)
		}
		return
	}

	// /collections/{id}
	switch r.Method {
	case http.MethodGet:
		collection, ok, err := s.lib.store.GetCollection(collectionID)
		if err != nil {
			s.writeError(w, errInternal, http.StatusInternalServerError)
			return
		}
		if !ok {
			s.writeError(w, errNotFound, http.StatusNotFound)
			return
		}
		writeJSON(w, r, collection)

	case http.MethodPut:
		if s.readOnly {
			s.writeError(w, "read-only mode", http.StatusForbidden)
			return
		}

		var payload struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			s.writeError(w, "bad request", http.StatusBadRequest)
			return
		}

		if strings.TrimSpace(payload.Name) == "" {
			s.writeError(w, "name is required", http.StatusBadRequest)
			return
		}

		if err := s.lib.store.UpdateCollection(collectionID, payload.Name, payload.Description, time.Now()); err != nil {
			s.writeError(w, errInternal, http.StatusInternalServerError)
			return
		}

		collection, ok, err := s.lib.store.GetCollection(collectionID)
		if err != nil {
			s.writeError(w, errInternal, http.StatusInternalServerError)
			return
		}
		if !ok {
			s.writeError(w, errNotFound, http.StatusNotFound)
			return
		}
		writeJSON(w, r, collection)

	case http.MethodDelete:
		if s.readOnly {
			s.writeError(w, "read-only mode", http.StatusForbidden)
			return
		}

		if err := s.lib.store.DeleteCollection(collectionID); err != nil {
			s.writeError(w, errInternal, http.StatusInternalServerError)
			return
		}
		writeJSON(w, r, map[string]string{"status": "ok"})

	default:
		s.methodNotAllowed(w)
	}
}

func newCollectionID() string {
	return fmt.Sprintf("col_%d", time.Now().UnixNano())
}
