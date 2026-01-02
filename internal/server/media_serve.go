package server

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func ServeVideoFile(w http.ResponseWriter, r *http.Request, path string) {
	f, err := os.Open(path)
	if err != nil {
		http.Error(w, errNotFound, http.StatusNotFound)
		return
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		http.Error(w, errInternal, http.StatusInternalServerError)
		return
	}

	etag := buildETag(st)
	// Content-Type best effort
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".avi":
		w.Header().Set("Content-Type", "video/x-msvideo")
	case ".mkv":
		w.Header().Set("Content-Type", "video/x-matroska")
	case ".m2ts", ".mts":
		w.Header().Set("Content-Type", "video/mp2t")
	case ".mov":
		w.Header().Set("Content-Type", "video/quicktime")
	case ".mp4", ".m4v":
		w.Header().Set("Content-Type", "video/mp4")
	case ".ts":
		w.Header().Set("Content-Type", "video/mp2t")
	case ".webm":
		w.Header().Set("Content-Type", "video/webm")
	}

	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("ETag", etag)

	if ifNoneMatch(r, etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	// ServeContent supports Range if the reader is seekable (os.File is).
	http.ServeContent(w, r, filepath.Base(path), st.ModTime(), f)
}

func ServeTextFile(w http.ResponseWriter, r *http.Request, path, contentType string) {
	st, err := os.Stat(path)
	if err != nil {
		http.Error(w, errNotFound, http.StatusNotFound)
		return
	}

	etag := buildETag(st)
	b, err := os.ReadFile(path)
	if err != nil {
		http.Error(w, errNotFound, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", normalizeTextContentType(contentType))
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("ETag", etag)

	if ifNoneMatch(r, etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	http.ServeContent(w, r, filepath.Base(path), st.ModTime(), bytes.NewReader(b))
}

func buildETag(info os.FileInfo) string {
	return fmt.Sprintf(`"%x-%x"`, info.Size(), info.ModTime().UnixNano())
}

func ifNoneMatch(r *http.Request, etag string) bool {
	ifNoneMatch := r.Header.Get("If-None-Match")
	if ifNoneMatch == "" {
		return false
	}
	for _, candidate := range strings.Split(ifNoneMatch, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || candidate == etag {
			return true
		}
	}
	return false
}
