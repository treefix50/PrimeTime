package ffmpeg

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

func Ensure(ctx context.Context, baseDir string) (string, error) {
	local := filepath.Join(baseDir, "tools", "ffmpeg", exe("ffmpeg"))
	if fileExists(local) {
		return local, nil
	}
	if runtime.GOOS == "windows" {
		return "", errors.New("ffmpeg.exe not found in tools/ffmpeg; place ffmpeg.exe and ffprobe.exe there")
	}
	return "", errors.New("ffmpeg not found in tools/ffmpeg; place ffmpeg and ffprobe there")
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
