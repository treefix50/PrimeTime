package ffmpeg

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// TranscodeOptions defines the parameters for transcoding
type TranscodeOptions struct {
	InputPath          string
	OutputPath         string
	VideoCodec         string
	AudioCodec         string
	AudioBitrateKbps   int64
	AudioTrackIndex    int
	AudioChannels      int
	AudioLayout        string
	AudioNormalization string
	PreferredLanguage  string
	Resolution         string
	MaxBitrate         int64
	Container          string
	StartTime          float64 // in seconds
	Duration           float64 // in seconds (0 = until end)
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

	args = appendAudioMapArgs(args, opts)

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
		if opts.AudioBitrateKbps > 0 {
			args = append(args, "-b:a", fmt.Sprintf("%dk", opts.AudioBitrateKbps))
		} else if opts.AudioCodec == "aac" {
			args = append(args, "-b:a", "128k")
		}
		args = appendAudioProcessingArgs(args, opts)
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

	args = appendAudioMapArgs(args, opts)

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
		if opts.AudioBitrateKbps > 0 {
			args = append(args, "-b:a", fmt.Sprintf("%dk", opts.AudioBitrateKbps))
		} else if opts.AudioCodec == "aac" {
			args = append(args, "-b:a", "128k")
		}
		args = appendAudioProcessingArgs(args, opts)
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

func appendAudioMapArgs(args []string, opts TranscodeOptions) []string {
	if opts.AudioTrackIndex >= 0 || opts.PreferredLanguage != "" {
		args = append(args, "-map", "0:v:0")
		if opts.AudioTrackIndex >= 0 {
			args = append(args, "-map", fmt.Sprintf("0:a:%d", opts.AudioTrackIndex))
			return args
		}
		args = append(args, "-map", fmt.Sprintf("0:a:m:language:%s", opts.PreferredLanguage))
	}
	return args
}

func appendAudioProcessingArgs(args []string, opts TranscodeOptions) []string {
	if opts.AudioChannels > 0 {
		args = append(args, "-ac", strconv.Itoa(opts.AudioChannels))
	}

	filters := make([]string, 0, 2)
	if strings.TrimSpace(opts.AudioLayout) != "" {
		filters = append(filters, fmt.Sprintf("pan=%s", opts.AudioLayout))
	}
	if strings.TrimSpace(opts.AudioNormalization) != "" {
		filters = append(filters, opts.AudioNormalization)
	}
	if len(filters) > 0 {
		args = append(args, "-af", strings.Join(filters, ","))
	}
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

	var probe struct {
		Format struct {
			Duration string `json:"duration"`
			BitRate  string `json:"bit_rate"`
		} `json:"format"`
		Streams []struct {
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
			BitRate   string `json:"bit_rate"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(output, &probe); err != nil {
		return nil, fmt.Errorf("ffprobe json parse failed: %w", err)
	}

	info := &VideoInfo{
		Path: videoPath,
	}

	if probe.Format.Duration != "" {
		if duration, err := strconv.ParseFloat(probe.Format.Duration, 64); err == nil {
			info.Duration = duration
		}
	}
	if probe.Format.BitRate != "" {
		if bitrate, err := strconv.ParseInt(probe.Format.BitRate, 10, 64); err == nil {
			info.Bitrate = bitrate
		}
	}

	for _, stream := range probe.Streams {
		if stream.CodecType == "video" {
			info.Codec = stream.CodecName
			if stream.Width > 0 {
				info.Width = stream.Width
			}
			if stream.Height > 0 {
				info.Height = stream.Height
			}
			if stream.BitRate != "" {
				if bitrate, err := strconv.ParseInt(stream.BitRate, 10, 64); err == nil {
					info.Bitrate = bitrate
				}
			}
			break
		}
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
