package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type Server struct {
	addr string
	lib  *Library
	http *http.Server
}

func New(root, addr string) (*Server, error) {
	lib, err := NewLibrary(root)
	if err != nil {
		return nil, err
	}
	// initial scan
	if err := lib.Scan(); err != nil {
		return nil, err
	}

	mux := http.NewServeMux()

	s := &Server{
		addr: addr,
		lib:  lib,
	}
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/library", s.handleLibrary)
	mux.HandleFunc("/items/", s.handleItems)

	s.http = &http.Server{
		Addr:              addr,
		Handler:           logMiddleware(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return s, nil
}

func (s *Server) Start() error { return s.http.ListenAndServe() }

func (s *Server) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return s.http.Shutdown(ctx)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) handleLibrary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, s.lib.All())
}

// Routes under /items/{id}[/{action}...]
func (s *Server) handleItems(w http.ResponseWriter, r *http.Request) {
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

	item, ok := s.lib.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}

	switch action {
	case "":
		// /items/{id}
		writeJSON(w, item)

	case "stream":
		// /items/{id}/stream
		ServeVideoFile(w, r, item.VideoPath)

	case "nfo":
		// /items/{id}/nfo  OR  /items/{id}/nfo/raw
		if len(parts) == 2 {
			nfo, err := ParseNFOFile(item.NFOPath)
			if err != nil {
				http.Error(w, "no nfo", http.StatusNotFound)
				return
			}
			writeJSON(w, nfo)
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

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}
