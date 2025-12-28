package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

type Server struct {
	addr         string
	lib          *Library
	http         *http.Server
	cors         bool
	scanInterval time.Duration
	scanTicker   *time.Ticker
	scanStop     chan struct{}
}

func New(root, addr string, store MediaStore, scanInterval time.Duration, cors bool) (*Server, error) {
	lib, err := NewLibrary(root, store)
	if err != nil {
		return nil, err
	}
	// initial scan
	if err := lib.Scan(); err != nil {
		return nil, err
	}

	mux := http.NewServeMux()

	s := &Server{
		addr:         addr,
		lib:          lib,
		cors:         cors,
		scanInterval: scanInterval,
	}

	if s.scanInterval > 0 {
		s.scanTicker = time.NewTicker(s.scanInterval)
		s.scanStop = make(chan struct{})
		go s.runScanTicker()
	}

	mux.HandleFunc("/health", s.handleHealth)
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
	for {
		select {
		case <-s.scanTicker.C:
			if err := s.lib.Scan(); err != nil {
				log.Printf("periodic scan failed: %v", err)
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
	close(s.scanStop)
	s.scanStop = nil
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", textContentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
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
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if query != "" {
			items = filterItems(items, query)
		}
		writeJSON(w, r, items)
	case http.MethodPost:
		if err := s.lib.Scan(); err != nil {
			http.Error(w, "scan failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, r, map[string]string{"status": "ok"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// Routes under /items/{id}[/{action}...]
func (s *Server) handleItems(w http.ResponseWriter, r *http.Request) {
	if s.handleOptions(w, r, "GET, OPTIONS") {
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/items/")
	parts := strings.Split(path, "/")
	if len(parts) < 1 || parts[0] == "" {
		http.NotFound(w, r)
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
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
	}
	if !ok {
		http.NotFound(w, r)
		return
	}

	switch action {
	case "":
		// /items/{id}
		writeJSON(w, r, item)

	case "stream":
		// /items/{id}/stream
		ServeVideoFile(w, r, item.VideoPath)

	case "nfo":
		// /items/{id}/nfo  OR  /items/{id}/nfo/raw
		if len(parts) == 2 {
			if s.lib.store != nil {
				nfo, ok, err := s.lib.store.GetNFO(item.ID)
				if err != nil {
					http.Error(w, "internal server error", http.StatusInternalServerError)
					return
				}
				if ok && nfo != nil {
					writeJSON(w, r, nfo)
					return
				}
			}

			nfo, err := ParseNFOFile(item.NFOPath)
			if err != nil {
				http.Error(w, "no nfo", http.StatusNotFound)
				return
			}
			writeJSON(w, r, nfo)
			return
		}

		if len(parts) == 3 && parts[2] == "raw" {
			if item.NFOPath == "" {
				http.Error(w, "no nfo", http.StatusNotFound)
				return
			}
			ServeTextFile(w, r, item.NFOPath, "text/xml; charset=utf-8")
			return
		}

		http.NotFound(w, r)

	default:
		http.NotFound(w, r)
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
		http.Error(w, "internal server error", http.StatusInternalServerError)
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
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
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
