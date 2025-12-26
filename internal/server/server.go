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
	items := s.lib.All()
	writeJSON(w, items)
}

// Routes under /items/{id}[/{action}]
func (s *Server) handleItems(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/items/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	id := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	item, ok := s.lib.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}

	switch action {
	case "":
		writeJSON(w, item)

	case "stream":
		// Range-Requests: http.ServeContent kann das (wenn Seekable Reader)
		ServeVideoFile(w, r, item.VideoPath)

	case "nfo":
		nfo, err := ParseNFOFile(item.NFOPath)
		if err != nil {
			http.Error(w, "no nfo or failed to parse", http.StatusNotFound)
			return
		}
		writeJSON(w, nfo)

	case "nfo", "nfo/":
		// unreachable (action parsing), but harmless
		http.NotFound(w, r)

	case "nforaw", "nfo_raw":
		http.NotFound(w, r)

	default:
		// support /items/{id}/nfo/raw
		if action == "nfo" && len(parts) > 2 && parts[2] == "raw" {
			if item.NFOPath == "" {
				http.Error(w, "no nfo", http.StatusNotFound)
				return
			}
			ServeTextFile(w, r, item.NFOPath, "text/xml; charset=utf-8")
			return
		}
		if action == "nfo" && len(parts) == 2 && parts[1] == "raw" {
			// if someone uses /items/{id}/raw by mistake
			http.NotFound(w, r)
			return
		}
		if action == "nfo" && len(parts) > 1 && parts[1] == "raw" {
			http.NotFound(w, r)
			return
		}
		http.NotFound(w, r)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}
