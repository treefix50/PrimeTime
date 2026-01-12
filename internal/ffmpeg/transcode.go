package ffmpeg

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// TranscodeOptions defines the parameters for transcoding
type TranscodeOptions struct {
	InputPath  string
	OutputPath string
	VideoCodec string
	AudioCodec string
	Resolution string
	MaxBitrate int64
	Container  string
	StartTime  float64 // in seconds
	Duration   float64 // in seconds (0 = until end)
}

// TranscodeResult contains information about the transcoding result
type TranscodeResult struct {
	OutputPath string
	Success    bool
	Error      error
}

// Transcode transcodes a video file using ffmpeg
func Transcode(ctx context.Context, ffmpegPath string, opts TranscodeOptions) (*TranscodeResult, error) {
	if ffmpegPath == "" {
		return nil, fmt.Errorf("ffmpeg path is empty")
	}
	if opts.InputPath == "" {
		return nil, fmt.Errorf("input path is required")
	}
	if opts.OutputPath == "" {
		return nil, fmt.Errorf("output path is required")
	}

	args := buildTranscodeArgs(opts)

	cmd := exec.CommandContext(ctx, ffmpegPath, args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return &TranscodeResult{
			OutputPath: opts.OutputPath,
			Success:    false,
			Error:      fmt.Errorf("ffmpeg failed: %w (output: %s)", err, string(output)),
		}, err
	}

	return &TranscodeResult{
		OutputPath: opts.OutputPath,
		Success:    true,
		Error:      nil,
	}, nil
}

// buildTranscodeArgs builds the ffmpeg command arguments
func buildTranscodeArgs(opts TranscodeOptions) []string {
	args := []string{
		"-y", // Overwrite output file
	}

	// Input file
	if opts.StartTime > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.2f", opts.StartTime))
	}
	args = append(args, "-i", opts.InputPath)

	// Duration
	if opts.Duration > 0 {
		args = append(args, "-t", fmt.Sprintf("%.2f", opts.Duration))
	}

	// Video codec
	if opts.VideoCodec == "copy" {
		args = append(args, "-c:v", "copy")
	} else {
		args = append(args, "-c:v", opts.VideoCodec)

		// Resolution
		if opts.Resolution != "" {
			args = append(args, "-s", opts.Resolution)
		}

		// Bitrate
		if opts.MaxBitrate > 0 {
			bitrateStr := fmt.Sprintf("%dk", opts.MaxBitrate/1000)
			args = append(args, "-b:v", bitrateStr)
			args = append(args, "-maxrate", bitrateStr)
			bufsize := opts.MaxBitrate * 2 / 1000
			args = append(args, "-bufsize", fmt.Sprintf("%dk", bufsize))
		}

		// Preset for x264/x265
		if strings.Contains(opts.VideoCodec, "264") || strings.Contains(opts.VideoCodec, "265") {
			args = append(args, "-preset", "veryfast")
		}
	}

	// Audio codec
	if opts.AudioCodec == "copy" {
		args = append(args, "-c:a", "copy")
	} else {
		args = append(args, "-c:a", opts.AudioCodec)
		if opts.AudioCodec == "aac" {
			args = append(args, "-b:a", "128k")
		}
	}

	// Container-specific options
	if opts.Container == "mp4" {
		args = append(args, "-movflags", "+faststart")
	}

	// Output file
	args = append(args, opts.OutputPath)

	return args
}

// TranscodeToHLS transcodes a video to HLS format
func TranscodeToHLS(ctx context.Context, ffmpegPath string, opts TranscodeOptions, segmentDuration int) (*TranscodeResult, error) {
	if ffmpegPath == "" {
		return nil, fmt.Errorf("ffmpeg path is empty")
	}
	if opts.InputPath == "" {
		return nil, fmt.Errorf("input path is required")
	}
	if opts.OutputPath == "" {
		return nil, fmt.Errorf("output path is required")
	}
	if segmentDuration <= 0 {
		segmentDuration = 6 // Default 6 seconds
	}

	args := buildHLSArgs(opts, segmentDuration)

	cmd := exec.CommandContext(ctx, ffmpegPath, args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return &TranscodeResult{
			OutputPath: opts.OutputPath,
			Success:    false,
			Error:      fmt.Errorf("ffmpeg HLS failed: %w (output: %s)", err, string(output)),
		}, err
	}

	return &TranscodeResult{
		OutputPath: opts.OutputPath,
		Success:    true,
		Error:      nil,
	}, nil
}

// buildHLSArgs builds the ffmpeg command arguments for HLS
func buildHLSArgs(opts TranscodeOptions, segmentDuration int) []string {
	args := []string{
		"-y", // Overwrite output file
	}

	// Input file
	if opts.StartTime > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.2f", opts.StartTime))
	}
	args = append(args, "-i", opts.InputPath)

	// Duration
	if opts.Duration > 0 {
		args = append(args, "-t", fmt.Sprintf("%.2f", opts.Duration))
	}

	// Video codec
	if opts.VideoCodec == "copy" {
		args = append(args, "-c:v", "copy")
	} else {
		args = append(args, "-c:v", opts.VideoCodec)

		// Resolution
		if opts.Resolution != "" {
			args = append(args, "-s", opts.Resolution)
		}

		// Bitrate
		if opts.MaxBitrate > 0 {
			bitrateStr := fmt.Sprintf("%dk", opts.MaxBitrate/1000)
			args = append(args, "-b:v", bitrateStr)
			args = append(args, "-maxrate", bitrateStr)
			bufsize := opts.MaxBitrate * 2 / 1000
			args = append(args, "-bufsize", fmt.Sprintf("%dk", bufsize))
		}

		// Preset for x264/x265
		if strings.Contains(opts.VideoCodec, "264") || strings.Contains(opts.VideoCodec, "265") {
			args = append(args, "-preset", "veryfast")
		}

		// GOP size for HLS
		args = append(args, "-g", strconv.Itoa(segmentDuration*30)) // Assuming 30fps
		args = append(args, "-keyint_min", strconv.Itoa(segmentDuration*30))
		args = append(args, "-sc_threshold", "0")
	}

	// Audio codec
	if opts.AudioCodec == "copy" {
		args = append(args, "-c:a", "copy")
	} else {
		args = append(args, "-c:a", opts.AudioCodec)
		if opts.AudioCodec == "aac" {
			args = append(args, "-b:a", "128k")
		}
	}

	// HLS-specific options
	args = append(args,
		"-f", "hls",
		"-hls_time", strconv.Itoa(segmentDuration),
		"-hls_list_size", "0",
		"-hls_segment_type", "mpegts",
		"-hls_flags", "independent_segments",
	)

	// Segment filename pattern
	segmentPattern := strings.TrimSuffix(opts.OutputPath, filepath.Ext(opts.OutputPath)) + "_%03d.ts"
	args = append(args, "-hls_segment_filename", segmentPattern)

	// Output playlist file
	args = append(args, opts.OutputPath)

	return args
}

// GetVideoInfo retrieves information about a video file using ffprobe
func GetVideoInfo(ctx context.Context, ffprobePath, videoPath string) (*VideoInfo, error) {
	if ffprobePath == "" {
		return nil, fmt.Errorf("ffprobe path is empty")
	}
	if videoPath == "" {
		return nil, fmt.Errorf("video path is required")
	}

	args := []string{
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		videoPath,
	}

	cmd := exec.CommandContext(ctx, ffprobePath, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	// Parse JSON output (simplified - in production use encoding/json)
	info := &VideoInfo{
		Path: videoPath,
	}

	// Extract duration from output (simplified parsing)
	outputStr := string(output)
	if strings.Contains(outputStr, "duration") {
		// This is a simplified version - proper implementation should use JSON parsing
		info.Duration = 0 // TODO: Parse JSON properly
	}

	return info, nil
}

// VideoInfo contains information about a video file
type VideoInfo struct {
	Path     string
	Duration float64
	Width    int
	Height   int
	Bitrate  int64
	Codec    string
}

// EstimateTranscodingTime estimates how long transcoding will take
func EstimateTranscodingTime(duration float64, profile string) float64 {
	// Rough estimates based on profile
	// These are very approximate and depend heavily on hardware
	multipliers := map[string]float64{
		"original": 0.1, // Just copying
		"mobile":   0.5, // Fast encoding
		"720p":     0.7, // Medium encoding
		"1080p":    1.0, // Slower encoding
		"4k":       2.0, // Very slow encoding
	}

	multiplier, ok := multipliers[profile]
	if !ok {
		multiplier = 1.0
	}

	return duration * multiplier
}
