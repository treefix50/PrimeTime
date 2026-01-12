package server

import (
	"os"
	"path/filepath"
	"strings"
)

// FindPosterForVideo searches for poster images next to the video file
func FindPosterForVideo(videoPath string) (string, bool) {
	if videoPath == "" {
		return "", false
	}

	base := strings.TrimSuffix(videoPath, filepath.Ext(videoPath))
	dir := filepath.Dir(videoPath)

	// Candidate poster filenames
	candidates := []string{
		base + ".jpg",
		base + ".jpeg",
		base + ".png",
		base + "-poster.jpg",
		base + "-poster.jpeg",
		base + "-poster.png",
		filepath.Join(dir, "poster.jpg"),
		filepath.Join(dir, "poster.jpeg"),
		filepath.Join(dir, "poster.png"),
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true
		}
	}

	return "", false
}

// GetPosterContentType returns the content type for a poster file
func GetPosterContentType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}
