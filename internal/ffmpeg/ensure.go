package ffmpeg

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func Ensure(ctx context.Context, baseDir string) (string, error) {
	local := filepath.Join(baseDir, "tools", "ffmpeg", exe("ffmpeg"))
	ffprobe := filepath.Join(baseDir, "tools", "ffmpeg", exe("ffprobe"))
	if !fileExists(local) || !fileExists(ffprobe) {
		if runtime.GOOS == "windows" {
			return "", errors.New("ffmpeg.exe/ffprobe.exe not found in tools/ffmpeg; copy the bin folder contents there")
		}
		return "", errors.New("ffmpeg/ffprobe not found in tools/ffmpeg; copy the bin folder contents there")
	}

	if err := validateBinary(ctx, local, "-version"); err != nil {
		return "", fmt.Errorf("ffmpeg validation failed: %w", err)
	}
	if err := validateBinary(ctx, ffprobe, "-version"); err != nil {
		return "", fmt.Errorf("ffprobe validation failed: %w", err)
	}

	return local, nil
}

func validateBinary(ctx context.Context, path string, args ...string) error {
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

func exe(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}
