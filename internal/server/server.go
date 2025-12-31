package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	errInternal         = "internal server error"
	errNotFound         = "not found"
	errMethodNotAllowed = "method not allowed"
)

func methodNotAllowed(w http.ResponseWriter) {
	http.Error(w, errMethodNotAllowed, http.StatusMethodNotAllowed)
}

type Server struct {
	addr         string
	lib          *Library
	http         *http.Server
	cors         bool
	readOnly     bool
	version      VersionInfo
	scanInterval time.Duration
	scanTicker   *time.Ticker
	scanStop     chan struct{}
	scanStopOnce sync.Once
	scanWg       sync.WaitGroup
}

type VersionInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"buildDate"`
}

func New(root, addr string, store MediaStore, scanInterval time.Duration, cors bool, version VersionInfo) (*Server, error) {
	lib, err := NewLibrary(root, store)
	if err != nil {
		return nil, err
	}
	readOnly := storeReadOnly(store)
	if !readOnly {
		// initial scan
		if err := lib.Scan(); err != nil {
			log.Printf("scan failed (initial): %v", err)
			return nil, err
		}
	}

	mux := http.NewServeMux()

	s := &Server{
		addr:         addr,
		lib:          lib,
		cors:         cors,
		readOnly:     readOnly,
		version:      version,
		scanInterval: scanInterval,
	}

	if s.scanInterval > 0 && !s.readOnly {
		s.scanTicker = time.NewTicker(s.scanInterval)
		s.scanStop = make(chan struct{})
		s.scanWg.Add(1)
		go s.runScanTicker()
	}

	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/stats", s.handleStats)
	mux.HandleFunc("/version", s.handleVersion)
	mux.HandleFunc("/library", s.handleLibrary)
	mux.HandleFunc("/items/", s.handleItems)

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
	w.Header().Set("Content-Type", textContentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	if s.handleOptions(w, r, "GET, OPTIONS") {
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	writeJSON(w, r, s.version)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if s.handleOptions(w, r, "GET, OPTIONS") {
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	totalItems, lastScan, err := s.lib.Stats()
	if err != nil {
		http.Error(w, errInternal, http.StatusInternalServerError)
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
		if s.lib.store == nil {
			items := s.lib.All()
			if query != "" {
				items = filterItems(items, query)
			}
			writeJSON(w, r, items)
			return
		}

		items, err := s.lib.store.GetAll()
		if err != nil {
			http.Error(w, errInternal, http.StatusInternalServerError)
			return
		}
		if query != "" {
			items = filterItems(items, query)
		}
		writeJSON(w, r, items)
	case http.MethodPost:
		if s.readOnly {
			http.Error(w, "read-only mode", http.StatusForbidden)
			return
		}
		if err := s.lib.Scan(); err != nil {
			http.Error(w, errInternal, http.StatusInternalServerError)
			return
		}
		writeJSON(w, r, map[string]string{"status": "ok"})
	default:
		methodNotAllowed(w)
	}
}

// Routes under /items/{id}[/{action}...]
func (s *Server) handleItems(w http.ResponseWriter, r *http.Request) {
	if s.handleOptions(w, r, "GET, POST, OPTIONS") {
		return
	}

	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/items/")
	parts := strings.Split(path, "/")
	if len(parts) < 1 || parts[0] == "" {
		http.Error(w, errNotFound, http.StatusNotFound)
		return
	}

	id := parts[0]
	action := ""
	if len(parts) >= 2 {
		action = parts[1]
	}

	var item MediaItem
	var ok bool
	if s.lib.store == nil {
		item, ok = s.lib.Get(id)
	} else {
		var err error
		item, ok, err = s.lib.store.GetByID(id)
		if err != nil {
			http.Error(w, errInternal, http.StatusInternalServerError)
			return
		}
	}
	if !ok {
		http.Error(w, errNotFound, http.StatusNotFound)
		return
	}

	switch action {
	case "":
		// /items/{id}
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		writeJSON(w, r, item)

	case "stream":
		// /items/{id}/stream
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		ServeVideoFile(w, r, item.VideoPath)

	case "nfo":
		// /items/{id}/nfo  OR  /items/{id}/nfo/raw
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		if len(parts) == 2 {
			if s.lib.store != nil {
				nfo, ok, err := s.lib.store.GetNFO(item.ID)
				if err != nil {
					http.Error(w, errInternal, http.StatusInternalServerError)
					return
				}
				if ok && nfo != nil {
					writeJSON(w, r, nfo)
					return
				}
			}

			nfo, err := ParseNFOFile(item.NFOPath)
			if err != nil {
				http.Error(w, errNotFound, http.StatusNotFound)
				return
			}
			writeJSON(w, r, nfo)
			return
		}

		if len(parts) == 3 && parts[2] == "raw" {
			if item.NFOPath == "" {
				http.Error(w, errNotFound, http.StatusNotFound)
				return
			}
			ServeTextFile(w, r, item.NFOPath, "text/xml; charset=utf-8")
			return
		}

		http.Error(w, errNotFound, http.StatusNotFound)

	case "playback":
		if s.lib.store == nil {
			http.Error(w, errNotFound, http.StatusNotFound)
			return
		}

		switch r.Method {
		case http.MethodGet:
			clientID := strings.TrimSpace(r.URL.Query().Get("clientId"))
			state, ok, err := s.lib.store.GetPlaybackState(item.ID, clientID)
			if err != nil {
				http.Error(w, errInternal, http.StatusInternalServerError)
				return
			}
			if !ok || state == nil {
				http.Error(w, errNotFound, http.StatusNotFound)
				return
			}
			writeJSON(w, r, state)
		case http.MethodPost:
			if s.readOnly {
				http.Error(w, "read-only mode", http.StatusForbidden)
				return
			}
			var payload PlaybackEvent
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}

			event := strings.ToLower(strings.TrimSpace(payload.Event))
			if event == "" {
				event = "progress"
			}
			if event != "progress" && event != "stop" {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}

			clientID := strings.TrimSpace(payload.ClientID)
			position := payload.PositionSeconds
			duration := payload.DurationSeconds

			shouldDelete := position <= 0 || duration <= 0 || (event == "stop" && position >= duration)
			if shouldDelete {
				if err := s.lib.store.DeletePlaybackState(item.ID, clientID); err != nil {
					http.Error(w, errInternal, http.StatusInternalServerError)
					return
				}
				writeJSON(w, r, map[string]string{"status": "ok"})
				return
			}

			if err := s.lib.store.UpsertPlaybackState(item.ID, position, duration, clientID); err != nil {
				http.Error(w, errInternal, http.StatusInternalServerError)
				return
			}
			writeJSON(w, r, map[string]string{"status": "ok"})
		default:
			methodNotAllowed(w)
		}

	default:
		http.Error(w, errNotFound, http.StatusNotFound)
	}
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

func (s *Server) handleOptions(w http.ResponseWriter, r *http.Request, methods string) bool {
	if r.Method != http.MethodOptions {
		return false
	}
	if s.cors {
		s.writePreflightHeaders(w, methods)
		w.WriteHeader(http.StatusOK)
		return true
	}
	methodNotAllowed(w)
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
