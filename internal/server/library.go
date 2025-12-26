package server

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type MediaItem struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	VideoPath string    `json:"videoPath"`
	NFOPath   string    `json:"nfoPath,omitempty"`
	Size      int64     `json:"size"`
	Modified  time.Time `json:"modified"`
}

type Library struct {
	root  string
	mu    sync.RWMutex
	items map[string]MediaItem
}

func NewLibrary(root string) (*Library, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return &Library{
		root:  root,
		items: map[string]MediaItem{},
	}, nil
}

func (l *Library) Scan() error {
	found := map[string]MediaItem{}
	var scanErrs []error

	err := filepath.WalkDir(l.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			scanErrs = append(scanErrs, err)
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(d.Name()))
		if ext != ".mkv" && ext != ".mp4" && ext != ".m4v" && ext != ".avi" {
			// minimal: video types; mkv requested; others optional
			return nil
		}

		info, err := d.Info()
		if err != nil {
			scanErrs = append(scanErrs, err)
			return nil
		}

		title := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
		nfo := guessNFOPath(path)

		id := stableID(path)

		found[id] = MediaItem{
			ID:        id,
			Title:     title,
			VideoPath: path,
			NFOPath:   nfo,
			Size:      info.Size(),
			Modified:  info.ModTime(),
		}
		return nil
	})
	if err != nil {
		return err
	}

	l.mu.Lock()
	l.items = found
	l.mu.Unlock()
	if len(scanErrs) > 0 {
		return errors.Join(scanErrs...)
	}
	return nil
}

func (l *Library) All() []MediaItem {
	l.mu.RLock()
	defer l.mu.RUnlock()

	out := make([]MediaItem, 0, len(l.items))
	for _, it := range l.items {
		out = append(out, it)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Title < out[j].Title })
	return out
}

func (l *Library) Get(id string) (MediaItem, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	it, ok := l.items[id]
	return it, ok
}

func guessNFOPath(videoPath string) string {
	base := strings.TrimSuffix(videoPath, filepath.Ext(videoPath))
	nfo := base + ".nfo"
	if _, err := os.Stat(nfo); err == nil {
		return nfo
	}
	return ""
}

func stableID(s string) string {
	h := sha1.Sum([]byte(s))
	return hex.EncodeToString(h[:8]) // short but stable
}
