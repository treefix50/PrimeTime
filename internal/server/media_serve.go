package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func ServeVideoFile(w http.ResponseWriter, r *http.Request, path string) {
	f, err := os.Open(path)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		http.Error(w, "file stat failed", http.StatusInternalServerError)
		return
	}

	// Content-Type best effort
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mkv":
		w.Header().Set("Content-Type", "video/x-matroska")
	case ".mp4", ".m4v":
		w.Header().Set("Content-Type", "video/mp4")
	}

	// ServeContent supports Range if the reader is seekable (os.File is).
	http.ServeContent(w, r, filepath.Base(path), st.ModTime(), f)
}

func ServeTextFile(w http.ResponseWriter, r *http.Request, path, contentType string) {
	b, err := os.ReadFile(path)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, filepath.Base(path), time.Now(), strings.NewReader(string(b)))
}
