package server

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/treefix50/primetime/internal/ffmpeg"
)

// TranscodingManager manages transcoding operations and cache
type TranscodingManager struct {
	ffmpegPath  string
	cacheDir    string
	store       MediaStore
	mu          sync.RWMutex
	activeJobs  map[string]*TranscodingJob
	maxCacheAge time.Duration
}

// TranscodingJob represents an active transcoding job
type TranscodingJob struct {
	ID         string
	MediaID    string
	ProfileID  string
	Status     string // "pending", "running", "completed", "failed"
	Progress   float64
	StartedAt  time.Time
	FinishedAt time.Time
	Error      string
	OutputPath string
	ctx        context.Context
	cancel     context.CancelFunc
}

// TranscodingJobSummary contains a safe JSON representation of a job.
type TranscodingJobSummary struct {
	ID         string    `json:"id"`
	MediaID    string    `json:"mediaId"`
	ProfileID  string    `json:"profileId"`
	Status     string    `json:"status"`
	Progress   float64   `json:"progress"`
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt,omitempty"`
	Error      string    `json:"error,omitempty"`
	OutputPath string    `json:"outputPath,omitempty"`
}

// NewTranscodingManager creates a new transcoding manager
func NewTranscodingManager(ffmpegPath, cacheDir string, store MediaStore) *TranscodingManager {
	return &TranscodingManager{
		ffmpegPath:  ffmpegPath,
		cacheDir:    cacheDir,
		store:       store,
		activeJobs:  make(map[string]*TranscodingJob),
		maxCacheAge: 24 * time.Hour, // Cache for 24 hours
	}
}

// StartTranscoding starts a transcoding job
func (tm *TranscodingManager) StartTranscoding(mediaID, profileID string, item MediaItem, profile TranscodingProfile, selection AudioSelection) (*TranscodingJob, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Check if job already exists
	jobID := generateJobID(mediaID, profileID)
	if job, exists := tm.activeJobs[jobID]; exists {
		if job.Status == "running" || job.Status == "pending" {
			return job, nil
		}
	}

	// Check cache first
	if tm.store != nil {
		cached, ok, err := tm.store.GetTranscodingCache(mediaID, profileID)
		if err == nil && ok {
			// Check if cache file still exists
			if _, err := os.Stat(cached.CachePath); err == nil {
				// Update last accessed
				cached.LastAccessed = time.Now()
				_ = tm.store.SaveTranscodingCache(*cached)

				return &TranscodingJob{
					ID:         jobID,
					MediaID:    mediaID,
					ProfileID:  profileID,
					Status:     "completed",
					Progress:   100,
					OutputPath: cached.CachePath,
				}, nil
			}
		}
	}

	// Create cache directory if needed
	if err := os.MkdirAll(tm.cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Prepare output path
	outputPath := filepath.Join(tm.cacheDir, fmt.Sprintf("%s_%s.%s", mediaID, profileID, profile.Container))

	// Create job
	ctx, cancel := context.WithCancel(context.Background())
	job := &TranscodingJob{
		ID:        jobID,
		MediaID:   mediaID,
		ProfileID: profileID,
		Status:    "pending",
		Progress:  0,
		StartedAt: time.Now(),
		ctx:       ctx,
		cancel:    cancel,
	}

	tm.activeJobs[jobID] = job

	// Start transcoding in background
	go tm.runTranscoding(job, item, profile, selection, outputPath)

	return job, nil
}

// runTranscoding executes the transcoding process
func (tm *TranscodingManager) runTranscoding(job *TranscodingJob, item MediaItem, profile TranscodingProfile, selection AudioSelection, outputPath string) {
	tm.mu.Lock()
	job.Status = "running"
	tm.mu.Unlock()

	audioDecision := decideAudioTranscode(profile, selection)
	log.Printf(
		"level=info msg=\"audio codec decision\" profile=%s source_codec=%s selected_codec=%s bitrate_kbps=%d note=%q",
		profile.Name,
		selection.SourceCodec,
		audioDecision.Codec,
		audioDecision.BitrateKbps,
		audioDecision.DecisionNote,
	)

	opts := ffmpeg.TranscodeOptions{
		InputPath:          item.VideoPath,
		OutputPath:         outputPath,
		VideoCodec:         profile.VideoCodec,
		AudioCodec:         audioDecision.Codec,
		AudioBitrateKbps:   audioDecision.BitrateKbps,
		AudioTrackIndex:    selection.TrackIndex,
		AudioChannels:      profile.MaxAudioChannels,
		AudioLayout:        profile.AudioLayout,
		AudioNormalization: profile.AudioNormalization,
		PreferredLanguage:  selection.PreferredLanguage,
		Resolution:         profile.Resolution,
		MaxBitrate:         profile.MaxBitrate,
		Container:          profile.Container,
	}

	result, err := ffmpeg.Transcode(job.ctx, tm.ffmpegPath, opts)

	tm.mu.Lock()
	defer tm.mu.Unlock()

	if err != nil {
		job.Status = "failed"
		job.Error = err.Error()
		job.FinishedAt = time.Now()
		return
	}

	job.Status = "completed"
	job.Progress = 100
	job.OutputPath = result.OutputPath
	job.FinishedAt = time.Now()

	// Save to cache
	if tm.store != nil {
		fileInfo, err := os.Stat(outputPath)
		if err != nil {
			_ = tm.store.DeleteTranscodingCache(job.ID)
			return
		}
		cache := TranscodingCache{
			ID:           job.ID,
			MediaID:      job.MediaID,
			ProfileID:    job.ProfileID,
			CachePath:    outputPath,
			CreatedAt:    time.Now(),
			LastAccessed: time.Now(),
			SizeBytes:    fileInfo.Size(),
		}
		_ = tm.store.SaveTranscodingCache(cache)
	}
}

// GetJob retrieves a transcoding job by ID
func (tm *TranscodingManager) GetJob(jobID string) (*TranscodingJob, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	job, ok := tm.activeJobs[jobID]
	return job, ok
}

// ListJobs returns a snapshot of all active jobs.
func (tm *TranscodingManager) ListJobs() []TranscodingJobSummary {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	jobs := make([]TranscodingJobSummary, 0, len(tm.activeJobs))
	for _, job := range tm.activeJobs {
		jobs = append(jobs, summarizeJob(job))
	}

	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].StartedAt.Before(jobs[j].StartedAt)
	})

	return jobs
}

// CancelJob cancels a transcoding job
func (tm *TranscodingManager) CancelJob(jobID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	job, ok := tm.activeJobs[jobID]
	if !ok {
		return fmt.Errorf("job not found")
	}

	if job.cancel != nil {
		job.cancel()
	}
	job.Status = "cancelled"
	return nil
}

// CleanupOldCache removes old cached files
func (tm *TranscodingManager) CleanupOldCache() error {
	if tm.store == nil {
		return nil
	}

	cutoff := time.Now().Add(-tm.maxCacheAge)
	return tm.store.CleanOldTranscodingCache(cutoff)
}

// StartHLSTranscoding starts HLS transcoding for adaptive streaming
func (tm *TranscodingManager) StartHLSTranscoding(mediaID, profileID string, item MediaItem, profile TranscodingProfile, selection AudioSelection) (*TranscodingJob, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	jobID := generateJobID(mediaID, profileID) + "_hls"

	// Check if job already exists
	if job, exists := tm.activeJobs[jobID]; exists {
		if job.Status == "running" || job.Status == "pending" {
			return job, nil
		}
	}

	// Create HLS directory
	hlsDir := filepath.Join(tm.cacheDir, "hls", mediaID, profileID)
	if err := os.MkdirAll(hlsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create HLS directory: %w", err)
	}

	// Prepare output path (playlist file)
	playlistPath := filepath.Join(hlsDir, "master.m3u8")

	// Create job
	ctx, cancel := context.WithCancel(context.Background())
	job := &TranscodingJob{
		ID:         jobID,
		MediaID:    mediaID,
		ProfileID:  profileID,
		Status:     "pending",
		Progress:   0,
		StartedAt:  time.Now(),
		OutputPath: playlistPath,
		ctx:        ctx,
		cancel:     cancel,
	}

	tm.activeJobs[jobID] = job

	// Start HLS transcoding in background
	go tm.runHLSTranscoding(job, item, profile, selection, playlistPath)

	return job, nil
}

// runHLSTranscoding executes the HLS transcoding process
func (tm *TranscodingManager) runHLSTranscoding(job *TranscodingJob, item MediaItem, profile TranscodingProfile, selection AudioSelection, playlistPath string) {
	tm.mu.Lock()
	job.Status = "running"
	tm.mu.Unlock()

	hlsDir := filepath.Dir(playlistPath)
	videoPlaylistPath := filepath.Join(hlsDir, "video.m3u8")
	audioVariants := tm.collectHLSAudioVariants(item, profile, selection)

	videoOpts := ffmpeg.TranscodeOptions{
		InputPath:    item.VideoPath,
		OutputPath:   videoPlaylistPath,
		VideoCodec:   profile.VideoCodec,
		Resolution:   profile.Resolution,
		MaxBitrate:   profile.MaxBitrate,
		Container:    "mpegts",
		DisableAudio: true,
	}

	_, err := ffmpeg.TranscodeToHLS(job.ctx, tm.ffmpegPath, videoOpts, 6)
	if err == nil {
		for i := range audioVariants {
			variant := audioVariants[i]
			audioSelection := AudioSelection{
				TrackIndex:        variant.TrackIndex,
				PreferredLanguage: variant.Language,
				SourceCodec:       variant.Codec,
			}
			audioDecision := decideAudioTranscode(profile, audioSelection)
			log.Printf(
				"level=info msg=\"audio codec decision\" profile=%s track=%d source_codec=%s selected_codec=%s bitrate_kbps=%d note=%q",
				profile.Name,
				variant.TrackIndex,
				variant.Codec,
				audioDecision.Codec,
				audioDecision.BitrateKbps,
				audioDecision.DecisionNote,
			)
			audioOpts := ffmpeg.TranscodeOptions{
				InputPath:          item.VideoPath,
				OutputPath:         filepath.Join(hlsDir, variant.PlaylistFilename),
				AudioCodec:         audioDecision.Codec,
				AudioBitrateKbps:   audioDecision.BitrateKbps,
				AudioTrackIndex:    variant.TrackIndex,
				AudioChannels:      profile.MaxAudioChannels,
				AudioLayout:        profile.AudioLayout,
				AudioNormalization: profile.AudioNormalization,
				PreferredLanguage:  variant.Language,
				Container:          "mpegts",
				DisableVideo:       true,
			}
			if _, audioErr := ffmpeg.TranscodeToHLS(job.ctx, tm.ffmpegPath, audioOpts, 6); audioErr != nil {
				err = audioErr
				break
			}
		}
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	if err != nil {
		job.Status = "failed"
		job.Error = err.Error()
		job.FinishedAt = time.Now()
		return
	}

	if err := writeHLSMasterPlaylist(playlistPath, profile, filepath.Base(videoPlaylistPath), audioVariants); err != nil {
		job.Status = "failed"
		job.Error = err.Error()
		job.FinishedAt = time.Now()
		return
	}

	job.Status = "completed"
	job.Progress = 100
	job.OutputPath = playlistPath
	job.FinishedAt = time.Now()
}

// ServeHLSPlaylist serves an HLS playlist file
func (tm *TranscodingManager) ServeHLSPlaylist(w http.ResponseWriter, r *http.Request, playlistPath string) {
	// Check if file exists
	content, err := os.ReadFile(playlistPath)
	if err != nil {
		http.Error(w, "playlist not found", http.StatusNotFound)
		return
	}

	basePath := hlsBasePath(r.URL.Path)
	mapped := rewriteHLSPlaylist(content, basePath, r.URL.RawQuery)

	// Set appropriate headers
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "no-cache")

	_, _ = w.Write(mapped)
}

// ServeHLSSegment serves an HLS segment file
func (tm *TranscodingManager) ServeHLSSegment(w http.ResponseWriter, r *http.Request, segmentPath string) {
	// Check if file exists
	if _, err := os.Stat(segmentPath); os.IsNotExist(err) {
		http.Error(w, "segment not found", http.StatusNotFound)
		return
	}

	// Set appropriate headers
	w.Header().Set("Content-Type", "video/mp2t")
	w.Header().Set("Cache-Control", "public, max-age=31536000")

	http.ServeFile(w, r, segmentPath)
}

// ServeTranscodedFile serves a transcoded file with range support
func (tm *TranscodingManager) ServeTranscodedFile(w http.ResponseWriter, r *http.Request, filePath string) {
	file, err := os.Open(filePath)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Set content type based on file extension
	ext := strings.ToLower(filepath.Ext(filePath))
	contentType := "video/mp4"
	switch ext {
	case ".mp4":
		contentType = "video/mp4"
	case ".webm":
		contentType = "video/webm"
	case ".mkv":
		contentType = "video/x-matroska"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Accept-Ranges", "bytes")

	// Handle range requests
	rangeHeader := r.Header.Get("Range")
	if rangeHeader == "" {
		// No range, serve entire file
		w.Header().Set("Content-Length", strconv.FormatInt(fileInfo.Size(), 10))
		w.WriteHeader(http.StatusOK)
		io.Copy(w, file)
		return
	}

	// Parse range header
	ranges := strings.Split(strings.TrimPrefix(rangeHeader, "bytes="), "-")
	if len(ranges) != 2 {
		http.Error(w, "invalid range", http.StatusRequestedRangeNotSatisfiable)
		return
	}

	start, err := strconv.ParseInt(ranges[0], 10, 64)
	if err != nil {
		http.Error(w, "invalid range", http.StatusRequestedRangeNotSatisfiable)
		return
	}

	end := fileInfo.Size() - 1
	if ranges[1] != "" {
		if parsedEnd, err := strconv.ParseInt(ranges[1], 10, 64); err == nil {
			end = parsedEnd
		}
	}

	if start > end || start < 0 || end >= fileInfo.Size() {
		http.Error(w, "invalid range", http.StatusRequestedRangeNotSatisfiable)
		return
	}

	contentLength := end - start + 1

	// Seek to start position
	if _, err := file.Seek(start, 0); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Set headers for partial content
	w.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileInfo.Size()))
	w.WriteHeader(http.StatusPartialContent)

	// Copy the requested range
	io.CopyN(w, file, contentLength)
}

// Helper functions

func generateJobID(mediaID, profileID string) string {
	sum := sha1.Sum([]byte(mediaID + ":" + profileID))
	return "job_" + hex.EncodeToString(sum[:8])
}

func summarizeJob(job *TranscodingJob) TranscodingJobSummary {
	if job == nil {
		return TranscodingJobSummary{}
	}

	return TranscodingJobSummary{
		ID:         job.ID,
		MediaID:    job.MediaID,
		ProfileID:  job.ProfileID,
		Status:     job.Status,
		Progress:   job.Progress,
		StartedAt:  job.StartedAt,
		FinishedAt: job.FinishedAt,
		Error:      job.Error,
		OutputPath: job.OutputPath,
	}
}

type hlsAudioVariant struct {
	TrackIndex       int
	Language         string
	Name             string
	Codec            string
	PlaylistFilename string
	Default          bool
}

func (tm *TranscodingManager) collectHLSAudioVariants(item MediaItem, profile TranscodingProfile, selection AudioSelection) []hlsAudioVariant {
	streams := tm.lookupAudioStreams(item)
	var variants []hlsAudioVariant
	if len(streams) > 0 {
		for i, stream := range streams {
			if !withinChannelLimit(stream, profile.MaxAudioChannels) {
				continue
			}
			variants = append(variants, hlsAudioVariant{
				TrackIndex: i,
				Language:   strings.TrimSpace(stream.Language),
				Codec:      strings.TrimSpace(stream.Codec),
			})
		}
	}
	if len(variants) == 0 {
		index := selection.TrackIndex
		if index < 0 {
			index = 0
		}
		variants = append(variants, hlsAudioVariant{
			TrackIndex: index,
			Language:   strings.TrimSpace(selection.PreferredLanguage),
			Codec:      strings.TrimSpace(selection.SourceCodec),
		})
	}

	defaultIndex := 0
	for i, variant := range variants {
		if selection.TrackIndex >= 0 && variant.TrackIndex == selection.TrackIndex {
			defaultIndex = i
			break
		}
		if selection.PreferredLanguage != "" && languageMatches(variant.Language, selection.PreferredLanguage) {
			defaultIndex = i
			break
		}
	}

	nameCounts := make(map[string]int)
	for i := range variants {
		name := variantDisplayName(variants[i].Language, variants[i].TrackIndex)
		nameCounts[name]++
		if nameCounts[name] > 1 {
			name = fmt.Sprintf("%s %d", name, nameCounts[name])
		}
		filename := hlsAudioFilename(name)
		if filename == "" {
			filename = fmt.Sprintf("audio_%d.m3u8", i+1)
		}
		variants[i].Name = name
		variants[i].PlaylistFilename = filename
		variants[i].Default = i == defaultIndex
	}

	return variants
}

func (tm *TranscodingManager) lookupAudioStreams(item MediaItem) []AudioStream {
	var nfo *NFO
	if tm.store != nil {
		if stored, ok, err := tm.store.GetNFOExtended(item.ID); err == nil && ok {
			nfo = stored
		}
	}
	if nfo == nil && item.NFOPath != "" {
		if parsed, err := ParseNFOFile(item.NFOPath); err == nil {
			nfo = parsed
		}
	}
	if nfo == nil || nfo.StreamDetails == nil || len(nfo.StreamDetails.Audio) == 0 {
		return nil
	}
	return nfo.StreamDetails.Audio
}

func variantDisplayName(language string, index int) string {
	if strings.TrimSpace(language) != "" {
		return strings.ToUpper(strings.TrimSpace(language))
	}
	return fmt.Sprintf("Audio %d", index+1)
}

func hlsAudioFilename(name string) string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	if normalized == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range normalized {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		if r == ' ' || r == '-' || r == '_' {
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return ""
	}
	return fmt.Sprintf("audio_%s.m3u8", b.String())
}

func writeHLSMasterPlaylist(path string, profile TranscodingProfile, videoPlaylist string, audioVariants []hlsAudioVariant) error {
	var builder strings.Builder
	builder.WriteString("#EXTM3U\n")
	builder.WriteString("#EXT-X-VERSION:3\n")

	if len(audioVariants) > 0 {
		for _, variant := range audioVariants {
			line := fmt.Sprintf("#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID=\"audio\",NAME=\"%s\"", variant.Name)
			if variant.Language != "" {
				line += fmt.Sprintf(",LANGUAGE=\"%s\"", strings.ToLower(variant.Language))
			}
			if variant.Default {
				line += ",DEFAULT=YES,AUTOSELECT=YES"
			} else {
				line += ",DEFAULT=NO,AUTOSELECT=YES"
			}
			line += fmt.Sprintf(",URI=\"%s\"", variant.PlaylistFilename)
			builder.WriteString(line)
			builder.WriteString("\n")
		}
	}

	bandwidth := profile.MaxBitrate
	if bandwidth <= 0 {
		bandwidth = 2_000_000
	}
	if len(audioVariants) > 0 {
		bandwidth += int64(len(audioVariants)) * 128_000
	}

	streamLine := fmt.Sprintf("#EXT-X-STREAM-INF:BANDWIDTH=%d", bandwidth)
	if profile.Resolution != "" {
		streamLine += fmt.Sprintf(",RESOLUTION=%s", profile.Resolution)
	}
	if len(audioVariants) > 0 {
		streamLine += ",AUDIO=\"audio\""
	}
	builder.WriteString(streamLine)
	builder.WriteString("\n")
	builder.WriteString(videoPlaylist)
	builder.WriteString("\n")

	return os.WriteFile(path, []byte(builder.String()), 0644)
}

func hlsBasePath(requestPath string) string {
	if strings.HasSuffix(requestPath, "stream.m3u8") {
		return strings.TrimSuffix(requestPath, "stream.m3u8") + "stream/"
	}
	if strings.HasSuffix(requestPath, ".m3u8") {
		return path.Dir(requestPath) + "/"
	}
	return path.Dir(requestPath) + "/"
}

func rewriteHLSPlaylist(content []byte, basePath, rawQuery string) []byte {
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	var builder strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#EXT-X-MEDIA") {
			builder.WriteString(rewriteHLSMediaLine(line, basePath, rawQuery))
			builder.WriteString("\n")
			continue
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			builder.WriteString(line)
			builder.WriteString("\n")
			continue
		}
		builder.WriteString(mapHLSURI(trimmed, basePath, rawQuery))
		builder.WriteString("\n")
	}
	return []byte(builder.String())
}

func rewriteHLSMediaLine(line, basePath, rawQuery string) string {
	start := strings.Index(line, "URI=\"")
	if start == -1 {
		return line
	}
	start += len("URI=\"")
	end := strings.Index(line[start:], "\"")
	if end == -1 {
		return line
	}
	uri := line[start : start+end]
	mapped := mapHLSURI(uri, basePath, rawQuery)
	return line[:start] + mapped + line[start+end:]
}

func mapHLSURI(uri, basePath, rawQuery string) string {
	if uri == "" {
		return uri
	}
	if strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://") || strings.HasPrefix(uri, "/") {
		return uri
	}
	mapped := basePath + uri
	if rawQuery != "" && !strings.Contains(mapped, "?") {
		mapped += "?" + rawQuery
	}
	return mapped
}
