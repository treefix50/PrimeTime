package server

import (
	"encoding/json"
	"net/http"
	"strings"
)

// handleLibraryRoots manages multiple media library roots
func (s *Server) handleLibraryRoots(w http.ResponseWriter, r *http.Request) {
	if s.handleOptions(w, r, "GET, POST, DELETE, OPTIONS") {
		return
	}
	if s.lib.store == nil {
		s.writeError(w, "not available without database", http.StatusNotImplemented)
		return
	}

	switch r.Method {
	case http.MethodGet:
		// List all library roots
		roots, err := s.lib.store.ListRoots()
		if err != nil {
			s.writeError(w, errInternal, http.StatusInternalServerError)
			return
		}
		writeJSON(w, r, roots)

	case http.MethodPost:
		if s.readOnly {
			s.writeError(w, "read-only mode", http.StatusForbidden)
			return
		}

		var payload struct {
			Path string `json:"path"`
			Type string `json:"type"` // "movies", "tv", "music", "photos", etc.
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			s.writeError(w, "bad request", http.StatusBadRequest)
			return
		}

		if strings.TrimSpace(payload.Path) == "" {
			s.writeError(w, "path is required", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(payload.Type) == "" {
			payload.Type = "library" // Default type
		}

		// Add the root
		root, err := s.lib.store.AddRoot(payload.Path, payload.Type)
		if err != nil {
			s.writeError(w, errInternal, http.StatusInternalServerError)
			return
		}

		// Trigger scan for the new root
		go func() {
			if err := s.lib.ScanPath(payload.Path); err != nil {
				// Log error but don't fail the request
				// The root was added successfully
			}
		}()

		writeJSON(w, r, root)

	case http.MethodDelete:
		if s.readOnly {
			s.writeError(w, "read-only mode", http.StatusForbidden)
			return
		}

		var payload struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			s.writeError(w, "bad request", http.StatusBadRequest)
			return
		}

		if strings.TrimSpace(payload.ID) == "" {
			s.writeError(w, "id is required", http.StatusBadRequest)
			return
		}

		if err := s.lib.store.RemoveRoot(payload.ID); err != nil {
			s.writeError(w, errInternal, http.StatusInternalServerError)
			return
		}

		writeJSON(w, r, map[string]string{"status": "ok"})

	default:
		s.methodNotAllowed(w)
	}
}

// handleLibraryRootScan triggers a scan for a specific root
func (s *Server) handleLibraryRootScan(w http.ResponseWriter, r *http.Request) {
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
	if s.lib.store == nil {
		s.writeError(w, "not available without database", http.StatusNotImplemented)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/library/roots/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] != "scan" {
		s.writeError(w, errNotFound, http.StatusNotFound)
		return
	}

	rootID := parts[0]

	// Get the root to find its path
	roots, err := s.lib.store.ListRoots()
	if err != nil {
		s.writeError(w, errInternal, http.StatusInternalServerError)
		return
	}

	var rootPath string
	for _, root := range roots {
		if root.ID == rootID {
			rootPath = root.Path
			break
		}
	}

	if rootPath == "" {
		s.writeError(w, "root not found", http.StatusNotFound)
		return
	}

	// Check rate limit
	if ok, wait := s.allowManualScan(); !ok {
		s.writeError(w, manualScanError(wait), http.StatusTooManyRequests)
		return
	}

	// Trigger scan
	if err := s.lib.ScanPath(rootPath); err != nil {
		s.writeError(w, errInternal, http.StatusInternalServerError)
		return
	}

	writeJSON(w, r, map[string]string{"status": "ok"})
}

// handleLibraryByType returns items filtered by NFO type (movie, tvshow) or grouped TV shows
func (s *Server) handleLibraryByType(w http.ResponseWriter, r *http.Request) {
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

	path := strings.TrimPrefix(r.URL.Path, "/library/type/")
	libraryType := strings.TrimSpace(path)
	if libraryType == "" {
		s.writeError(w, "library type is required", http.StatusBadRequest)
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	sortBy := normalizeSortBy(r.URL.Query().Get("sort"))
	limit, offset, ok := parseLimitOffset(r)
	if !ok {
		s.writeError(w, "bad request", http.StatusBadRequest)
		return
	}

	// Map client library types to NFO types
	var nfoType string
	switch strings.ToLower(libraryType) {
	case "movie", "movies", "filme":
		nfoType = "movie"
	case "tvshow", "tvshows", "tv", "series", "serien":
		// For TV shows, return grouped shows instead of individual episodes
		shows, err := s.lib.store.GetTVShowsGrouped(limit, offset, sortBy, query)
		if err != nil {
			s.writeError(w, errInternal, http.StatusInternalServerError)
			return
		}
		writeJSON(w, r, shows)
		return
	case "episode", "episodes":
		nfoType = "episode"
	default:
		// Try to use it as-is
		nfoType = libraryType
	}

	// Get items filtered by NFO type
	items, err := s.lib.store.GetItemsByNFOType(nfoType, limit, offset, sortBy, query)
	if err != nil {
		s.writeError(w, errInternal, http.StatusInternalServerError)
		return
	}

	writeJSON(w, r, items)
}
