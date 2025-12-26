package ffmpeg

import (
	"archive/zip"
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var (
	// Windows build source (stable "latest" asset)
	ffmpegZipURL      = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl-shared.zip"
	ffmpegChecksumsURL = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/checksums.sha256"
)

// Ensure makes sure ffmpeg exists.
// Order:
// 1) ffmpeg in PATH
// 2) tools/ffmpeg/ffmpeg(.exe)
// 3) download + verify + extract (Windows only)
func Ensure(ctx context.Context, baseDir string) (string, error) {
	// allow disabling auto-download (useful for CI / locked-down envs)
	if os.Getenv("PRIMETIME_NO_FFMPEG_DOWNLOAD") == "1" {
		if p, err := exec.LookPath("ffmpeg"); err == nil {
			return p, nil
		}
		return "", errors.New("ffmpeg not found and auto-download disabled (PRIMETIME_NO_FFMPEG_DOWNLOAD=1)")
	}

	// 1) PATH
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		return p, nil
	}

	// 2) local tools path
	local := filepath.Join(baseDir, "tools", "ffmpeg", exeName("ffmpeg"))
	if fileExists(local) {
		return local, nil
	}

	// 3) auto-download (Windows only in this minimal version)
	if runtime.GOOS != "windows" {
		return "", errors.New("ffmpeg not found in PATH; auto-download is currently implemented for Windows only")
	}

	if err := os.MkdirAll(filepath.Dir(local), 0o755); err != nil {
		return "", err
	}

	zipPath := filepath.Join(baseDir, "tools", "ffmpeg", "ffmpeg.zip")

	// download checksums + zip
	if err := download(ctx, ffmpegChecksumsURL, filepath.Join(baseDir, "tools", "ffmpeg", "checksums.sha256")); err != nil {
		return "", fmt.Errorf("download checksums: %w", err)
	}
	if err := download(ctx, ffmpegZipURL, zipPath); err != nil {
		return "", fmt.Errorf("download ffmpeg zip: %w", err)
	}

	// verify sha256 from checksums file
	want, err := findSHA256(filepath.Join(baseDir, "tools", "ffmpeg", "checksums.sha256"), filepath.Base(ffmpegZipURL))
	if err != nil {
		return "", fmt.Errorf("parse checksums: %w", err)
	}
	got, err := sha256File(zipPath)
	if err != nil {
		return "", fmt.Errorf("hash zip: %w", err)
	}
	if !strings.EqualFold(want, got) {
		return "", fmt.Errorf("ffmpeg zip checksum mismatch: want %s got %s", want, got)
	}

	// extract ffmpeg.exe + ffprobe.exe from zip
	destDir := filepath.Join(baseDir, "tools", "ffmpeg")
	if err := extractBinaries(zipPath, destDir, []string{
		"ffmpeg.exe",
		"ffprobe.exe",
	}); err != nil {
		return "", fmt.Errorf("extract: %w", err)
	}

	// cleanup zip (optional)
	_ = os.Remove(zipPath)

	if fileExists(local) {
		return local, nil
	}
	return "", errors.New("ffmpeg download/extract finished, but ffmpeg.exe not found")
}

func exeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func download(ctx context.Context, url, out string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	cli := &http.Client{Timeout: 5 * time.Minute}
	res, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("http %d", res.StatusCode)
	}

	tmp := out + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(f, res.Body); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, out)
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// checksums.sha256 format typically: "<sha256>  <filename>"
func findSHA256(checksumsPath, filename string) (string, error) {
	f, err := os.Open(checksumsPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		sum := fields[0]
		name := fields[len(fields)-1]
		// sometimes filename might be prefixed with '*' or similar
		name = strings.TrimLeft(name, "*")
		if name == filename {
			return sum, nil
		}
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("checksum for %s not found", filename)
}

// The BtbN zip contains a top-level folder; binaries are usually under ".../bin/ffmpeg.exe".
func extractBinaries(zipPath, destDir string, wanted []string) error {
	wantSet := map[string]bool{}
	for _, w := range wanted {
		wantSet[strings.ToLower(w)] = false
	}

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	for _, f := range zr.File {
		low := strings.ToLower(filepath.Base(f.Name))
		if _, ok := wantSet[low]; !ok {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		outPath := filepath.Join(destDir, low)
		out, err := os.Create(outPath)
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			rc.Close()
			return err
		}
		out.Close()
		rc.Close()

		wantSet[low] = true
	}

	for name, ok := range wantSet {
		if !ok {
			return fmt.Errorf("missing %s in zip", name)
		}
	}
	return nil
}
