package server

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
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
	StableKey string    `json:"-"`
}

type Library struct {
	root              string
	rootID            string
	mu                sync.RWMutex
	items             map[string]MediaItem
	store             MediaStore
	allowedExtensions map[string]bool
	// lastScan tracks the time the library last completed a scan.
	lastScan time.Time
}

var (
	ErrInvalidScanPath  = errors.New("invalid scan path")
	ErrScanPathNotFound = errors.New("scan path not found")
)

var defaultExtensions = []string{
	".avi",
	".m2ts",
	".m4v",
	".mkv",
	".mov",
	".mp4",
	".ts",
	".webm",
}

func storeReadOnly(store MediaStore) bool {
	if store == nil {
		return false
	}
	return store.ReadOnly()
}

func buildAllowedExtensions(extensions []string) map[string]bool {
	if len(extensions) == 0 {
		extensions = defaultExtensions
	}
	allowed := make(map[string]bool, len(extensions))
	for _, ext := range extensions {
		normalized := normalizeExtension(ext)
		if normalized == "" {
			continue
		}
		allowed[normalized] = true
	}
	return allowed
}

func normalizeExtension(ext string) string {
	ext = strings.TrimSpace(ext)
	if ext == "" {
		return ""
	}
	ext = strings.ToLower(ext)
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return ext
}

func NewLibrary(root string, store MediaStore, extensions []string) (*Library, error) {
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
		root:              root,
		rootID:            rootID,
		items:             items,
		store:             store,
		allowedExtensions: buildAllowedExtensions(extensions),
	}, nil
}

func (l *Library) ScanPath(path string) error {
	targetPath, info, err := l.resolveScanPath(path)
	if err != nil {
		return err
	}

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

	addFile := func(path string, info fs.FileInfo) {
		ext := strings.ToLower(filepath.Ext(info.Name()))
		if !l.allowedExtensions[ext] {
			return
		}

		rawTitle := strings.TrimSuffix(info.Name(), filepath.Ext(info.Name()))
		title := rawTitle
		if parsedTitle, _, _, ok := parseEpisodeInfo(rawTitle); ok {
			title = parsedTitle
		}
		nfo := guessNFOPath(path)

		stableKey := stableID(path, info)
		id := stableKey
		if l.store != nil {
			if existingID, ok, err := l.store.GetIDByPath(path); err != nil {
				scanErrs = append(scanErrs, err)
			} else if ok {
				id = existingID
			} else if !storeReadOnly(l.store) {
				id = newUUID()
			}
		}

		found[id] = MediaItem{
			ID:        id,
			Title:     title,
			VideoPath: path,
			NFOPath:   nfo,
			Size:      info.Size(),
			Modified:  info.ModTime(),
			StableKey: stableKey,
		}
	}

	if info.IsDir() {
		err = filepath.WalkDir(targetPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				scanErrs = append(scanErrs, err)
				return nil
			}
			if d.IsDir() {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				scanErrs = append(scanErrs, err)
				return nil
			}
			addFile(path, info)
			return nil
		})
		if err != nil {
			scanErrs = append(scanErrs, err)
		}
	} else {
		addFile(targetPath, info)
	}

	l.mu.Lock()
	previous := l.items
	lastScan := l.lastScan
	updated := make(map[string]MediaItem, len(previous)+len(found))
	previousWithin := make(map[string]MediaItem)
	for id, item := range previous {
		if pathWithin(targetPath, item.VideoPath) {
			previousWithin[id] = item
			continue
		}
		updated[id] = item
	}
	for id, item := range found {
		updated[id] = item
	}
	l.items = updated
	l.lastScan = time.Now()
	l.mu.Unlock()

	if l.store != nil {
		idsToDelete := removedIDs(previousWithin, found)
		if len(idsToDelete) > 0 {
			if err := l.store.DeleteItems(idsToDelete); err != nil {
				scanErrs = append(scanErrs, err)
			}
		}
		itemsToSave := diffItems(found, previousWithin, lastScan)
		if len(itemsToSave) > 0 {
			if err := l.store.SaveItems(itemsToSave); err != nil {
				scanErrs = append(scanErrs, err)
			}
		}
		for _, item := range found {
			if item.NFOPath == "" {
				if fallback, ok := fallbackNFOFromFilename(item.VideoPath); ok {
					if err := l.store.SaveNFO(item.ID, fallback); err != nil {
						scanErrs = append(scanErrs, err)
					}
					continue
				}
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
	err := filepath.WalkDir(l.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			scanErrs = append(scanErrs, err)
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(d.Name()))
		if !l.allowedExtensions[ext] {
			// minimal: video types; mkv requested; others optional
			return nil
		}

		info, err := d.Info()
		if err != nil {
			scanErrs = append(scanErrs, err)
			return nil
		}

		rawTitle := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
		title := rawTitle
		if parsedTitle, _, _, ok := parseEpisodeInfo(rawTitle); ok {
			title = parsedTitle
		}
		nfo := guessNFOPath(path)

		stableKey := stableID(path, info)
		id := stableKey
		if l.store != nil {
			if existingID, ok, err := l.store.GetIDByPath(path); err != nil {
				scanErrs = append(scanErrs, err)
			} else if ok {
				id = existingID
			} else if !storeReadOnly(l.store) {
				id = newUUID()
			}
		}

		found[id] = MediaItem{
			ID:        id,
			Title:     title,
			VideoPath: path,
			NFOPath:   nfo,
			Size:      info.Size(),
			Modified:  info.ModTime(),
			StableKey: stableKey,
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
				if fallback, ok := fallbackNFOFromFilename(item.VideoPath); ok {
					if err := l.store.SaveNFO(item.ID, fallback); err != nil {
						scanErrs = append(scanErrs, err)
					}
					continue
				}
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

func (l *Library) resolveScanPath(path string) (string, fs.FileInfo, error) {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return "", nil, ErrInvalidScanPath
	}
	cleanPath = filepath.Clean(cleanPath)
	if !filepath.IsAbs(cleanPath) {
		cleanPath = filepath.Join(l.root, cleanPath)
	}
	rootAbs, err := filepath.Abs(l.root)
	if err != nil {
		return "", nil, err
	}
	targetAbs, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", nil, err
	}
	if !pathWithin(rootAbs, targetAbs) {
		return "", nil, ErrInvalidScanPath
	}
	info, err := os.Stat(targetAbs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil, ErrScanPathNotFound
		}
		return "", nil, err
	}
	return targetAbs, info, nil
}

func pathWithin(basePath, targetPath string) bool {
	rel, err := filepath.Rel(basePath, targetPath)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	if rel == ".." {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator))
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
	sortItems(out, "title")
	return out
}

func (l *Library) Stats() (int, time.Time, error) {
	l.mu.RLock()
	lastScan := l.lastScan
	count := len(l.items)
	l.mu.RUnlock()

	if l.store == nil {
		return count, lastScan, nil
	}

	items, err := l.store.GetAll()
	if err != nil {
		return count, lastScan, err
	}
	return len(items), lastScan, nil
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
	sortItems(out, "title")
	return out
}

func normalizeSortBy(sortBy string) string {
	sortBy = strings.ToLower(strings.TrimSpace(sortBy))
	switch sortBy {
	case "title", "modified", "size":
		return sortBy
	default:
		return "title"
	}
}

func sortItems(items []MediaItem, sortBy string) {
	sortBy = normalizeSortBy(sortBy)
	switch sortBy {
	case "modified":
		sort.Slice(items, func(i, j int) bool {
			if items[i].Modified.Equal(items[j].Modified) {
				return strings.ToLower(items[i].Title) < strings.ToLower(items[j].Title)
			}
			return items[i].Modified.After(items[j].Modified)
		})
	case "size":
		sort.Slice(items, func(i, j int) bool {
			if items[i].Size == items[j].Size {
				return strings.ToLower(items[i].Title) < strings.ToLower(items[j].Title)
			}
			return items[i].Size > items[j].Size
		})
	default:
		sort.Slice(items, func(i, j int) bool {
			return strings.ToLower(items[i].Title) < strings.ToLower(items[j].Title)
		})
	}
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
		a.Modified.Equal(b.Modified) &&
		a.StableKey == b.StableKey
}

func guessNFOPath(videoPath string) string {
	base := strings.TrimSuffix(videoPath, filepath.Ext(videoPath))
	nfo := base + ".nfo"
	if _, err := os.Stat(nfo); err == nil {
		return nfo
	}
	return ""
}

var episodePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^(.+?)[ ._-]*s(\d{1,2})[ ._-]*e(\d{1,2})`),
	regexp.MustCompile(`(?i)^(.+?)[ ._-]*(\d{1,2})x(\d{1,2})`),
}

func parseEpisodeInfo(name string) (string, string, string, bool) {
	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		return "", "", "", false
	}
	for _, pattern := range episodePatterns {
		matches := pattern.FindStringSubmatch(cleanName)
		if len(matches) != 4 {
			continue
		}
		title := normalizeEpisodeTitle(matches[1])
		if title == "" {
			return "", "", "", false
		}
		season, ok := normalizeEpisodeNumber(matches[2])
		if !ok {
			return "", "", "", false
		}
		episode, ok := normalizeEpisodeNumber(matches[3])
		if !ok {
			return "", "", "", false
		}
		return title, season, episode, true
	}
	return "", "", "", false
}

func normalizeEpisodeTitle(raw string) string {
	title := strings.NewReplacer(".", " ", "_", " ", "-", " ").Replace(raw)
	parts := strings.Fields(title)
	return strings.Join(parts, " ")
}

func normalizeEpisodeNumber(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	number, err := strconv.Atoi(value)
	if err != nil {
		return "", false
	}
	if number < 0 {
		return "", false
	}
	return strconv.Itoa(number), true
}

func fallbackNFOFromFilename(videoPath string) (*NFO, bool) {
	base := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	title, season, episode, ok := parseEpisodeInfo(base)
	if !ok {
		return nil, false
	}
	return &NFO{
		Type:        "episode",
		Title:       title,
		Season:      season,
		Episode:     episode,
		RawRootName: "filename",
	}, true
}

func stableID(path string, info fs.FileInfo) string {
	identity := path
	if fileID, ok := fileIdentity(info); ok {
		identity = fmt.Sprintf("%s|%s", path, fileID)
	} else {
		identity = fmt.Sprintf("%s|%d|%d", path, info.Size(), info.ModTime().UnixNano())
	}
	h := sha1.Sum([]byte(identity))
	return hex.EncodeToString(h[:8]) // short but stable
}

func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		h := sha1.Sum([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
		return hex.EncodeToString(h[:16])
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
