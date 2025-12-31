package server

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"io/fs"
	"log"
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
	root   string
	rootID string
	mu     sync.RWMutex
	items  map[string]MediaItem
	store  MediaStore
	// lastScan tracks the time the library last completed a scan.
	lastScan time.Time
}

func storeReadOnly(store MediaStore) bool {
	if store == nil {
		return false
	}
	return store.ReadOnly()
}

func NewLibrary(root string, store MediaStore) (*Library, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	items := map[string]MediaItem{}
	var rootID string
	if store != nil && !storeReadOnly(store) {
		rootEntry, err := store.AddRoot(root, "library")
		if err != nil {
			return nil, err
		}
		rootID = rootEntry.ID
	}
	if store != nil {
		storedItems, err := store.GetAll()
		if err != nil {
			return nil, err
		}
		for _, item := range storedItems {
			items[item.ID] = item
		}
	}
	return &Library{
		root:   root,
		rootID: rootID,
		items:  items,
		store:  store,
	}, nil
}

func (l *Library) Scan() error {
	found := map[string]MediaItem{}
	var scanErrs []error
	var scanRunID string
	if l.store != nil && l.rootID != "" {
		run, err := l.store.StartScanRun(l.rootID, time.Now())
		if err != nil {
			scanErrs = append(scanErrs, err)
		} else {
			scanRunID = run.ID
		}
	}
	allowedExtensions := map[string]bool{
		".avi":  true,
		".m2ts": true,
		".m4v":  true,
		".mkv":  true,
		".mov":  true,
		".mp4":  true,
		".ts":   true,
		".webm": true,
	}

	err := filepath.WalkDir(l.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			scanErrs = append(scanErrs, err)
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(d.Name()))
		if !allowedExtensions[ext] {
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
		scanErrs = append(scanErrs, err)
	}

	l.mu.Lock()
	previous := l.items
	lastScan := l.lastScan
	l.items = found
	l.lastScan = time.Now()
	l.mu.Unlock()

	if l.store != nil {
		idsToDelete := removedIDs(previous, found)
		if len(idsToDelete) > 0 {
			if err := l.store.DeleteItems(idsToDelete); err != nil {
				scanErrs = append(scanErrs, err)
			}
		}
		itemsToSave := diffItems(found, previous, lastScan)
		if len(itemsToSave) > 0 {
			if err := l.store.SaveItems(itemsToSave); err != nil {
				scanErrs = append(scanErrs, err)
			}
		}
		for _, item := range found {
			if item.NFOPath == "" {
				if err := l.store.DeleteNFO(item.ID); err != nil {
					scanErrs = append(scanErrs, err)
				}
				continue
			}
			nfo, err := ParseNFOFile(item.NFOPath)
			if err != nil {
				log.Printf("level=warn msg=\"nfo parse failed\" path=%s err=%v", item.NFOPath, err)
				continue
			}
			if err := l.store.SaveNFO(item.ID, nfo); err != nil {
				scanErrs = append(scanErrs, err)
			}
		}
	}

	var scanErr error
	if len(scanErrs) > 0 {
		scanErr = errors.Join(scanErrs...)
	}
	if scanRunID != "" {
		finishedAt := time.Now()
		if scanErr != nil {
			if err := l.store.FailScanRun(scanRunID, finishedAt, scanErr.Error()); err != nil {
				scanErrs = append(scanErrs, err)
				scanErr = errors.Join(scanErrs...)
			}
		} else {
			if err := l.store.FinishScanRun(scanRunID, finishedAt); err != nil {
				scanErrs = append(scanErrs, err)
				scanErr = errors.Join(scanErrs...)
			}
		}
	}
	if scanErr != nil {
		return scanErr
	}
	return nil
}

func (l *Library) All() []MediaItem {
	if l.store != nil {
		dbItems, err := l.store.GetAll()
		if err == nil {
			cacheItems := l.snapshotItems()
			return mergeItems(dbItems, cacheItems)
		}
	}

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
	if l.store != nil {
		item, ok, err := l.store.GetByID(id)
		if err == nil && ok {
			return item, true
		}
	}

	l.mu.RLock()
	defer l.mu.RUnlock()
	it, ok := l.items[id]
	return it, ok
}

func (l *Library) snapshotItems() []MediaItem {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]MediaItem, 0, len(l.items))
	for _, item := range l.items {
		out = append(out, item)
	}
	return out
}

func mergeItems(dbItems, cacheItems []MediaItem) []MediaItem {
	merged := make(map[string]MediaItem, len(dbItems)+len(cacheItems))
	for _, item := range dbItems {
		merged[item.ID] = item
	}
	for _, item := range cacheItems {
		merged[item.ID] = item
	}
	out := make([]MediaItem, 0, len(merged))
	for _, item := range merged {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Title < out[j].Title })
	return out
}

func diffItems(found, previous map[string]MediaItem, lastScan time.Time) []MediaItem {
	if lastScan.IsZero() {
		out := make([]MediaItem, 0, len(found))
		for _, item := range found {
			out = append(out, item)
		}
		return out
	}

	out := make([]MediaItem, 0, len(found))
	for id, item := range found {
		prev, ok := previous[id]
		if !ok || !mediaItemEqual(item, prev) {
			out = append(out, item)
		}
	}
	return out
}

func removedIDs(previous, found map[string]MediaItem) []string {
	if len(previous) == 0 {
		return nil
	}
	out := make([]string, 0, len(previous))
	for id := range previous {
		if _, ok := found[id]; !ok {
			out = append(out, id)
		}
	}
	return out
}

func mediaItemEqual(a, b MediaItem) bool {
	return a.ID == b.ID &&
		a.Title == b.Title &&
		a.VideoPath == b.VideoPath &&
		a.NFOPath == b.NFOPath &&
		a.Size == b.Size &&
		a.Modified.Equal(b.Modified)
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
