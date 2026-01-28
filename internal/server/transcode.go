package server

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
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

	opts := ffmpeg.TranscodeOptions{
		InputPath:         item.VideoPath,
		OutputPath:        outputPath,
		VideoCodec:        profile.VideoCodec,
		AudioCodec:        profile.AudioCodec,
		AudioTrackIndex:   selection.TrackIndex,
		PreferredLanguage: selection.PreferredLanguage,
		Resolution:        profile.Resolution,
		MaxBitrate:        profile.MaxBitrate,
		Container:         profile.Container,
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
	playlistPath := filepath.Join(hlsDir, "playlist.m3u8")

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

	opts := ffmpeg.TranscodeOptions{
		InputPath:         item.VideoPath,
		OutputPath:        playlistPath,
		VideoCodec:        profile.VideoCodec,
		AudioCodec:        profile.AudioCodec,
		AudioTrackIndex:   selection.TrackIndex,
		PreferredLanguage: selection.PreferredLanguage,
		Resolution:        profile.Resolution,
		MaxBitrate:        profile.MaxBitrate,
		Container:         "mpegts", // HLS uses MPEG-TS segments
	}

	result, err := ffmpeg.TranscodeToHLS(job.ctx, tm.ffmpegPath, opts, 6)

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
}

// ServeHLSPlaylist serves an HLS playlist file
func (tm *TranscodingManager) ServeHLSPlaylist(w http.ResponseWriter, r *http.Request, playlistPath string) {
	// Check if file exists
	if _, err := os.Stat(playlistPath); os.IsNotExist(err) {
		http.Error(w, "playlist not found", http.StatusNotFound)
		return
	}

	// Set appropriate headers
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "no-cache")

	http.ServeFile(w, r, playlistPath)
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
