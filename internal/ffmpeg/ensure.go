package ffmpeg

import (
	"archive/zip"
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

const (
	// Windows build (BtbN FFmpeg-Builds "latest" asset)
	winZipURL       = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl-shared.zip"
	winChecksumsURL = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/checksums.sha256"
)

func Ensure(ctx context.Context, baseDir string) (string, error) {
	// Allow disabling downloads (CI / locked-down machines)
	if os.Getenv("PRIMETIME_NO_FFMPEG_DOWNLOAD") == "1" {
		if p, err := exec.LookPath("ffmpeg"); err == nil {
			return p, nil
		}
		local := filepath.Join(baseDir, "tools", "ffmpeg", exe("ffmpeg"))
		if fileExists(local) {
			return local, nil
		}
		return "", errors.New("ffmpeg not found and auto-download disabled (PRIMETIME_NO_FFMPEG_DOWNLOAD=1)")
	}

	// Auto-download (Windows only in this minimal version)
	if runtime.GOOS != "windows" {
		if p, err := exec.LookPath("ffmpeg"); err == nil {
			return p, nil
		}
		local := filepath.Join(baseDir, "tools", "ffmpeg", exe("ffmpeg"))
		if fileExists(local) {
			return local, nil
		}
		return "", errors.New("ffmpeg not found in PATH; auto-download is currently implemented for Windows only")
	}

	local := filepath.Join(baseDir, "tools", "ffmpeg", exe("ffmpeg"))
	if fileExists(local) {
		return local, nil
	}

	destDir := filepath.Join(baseDir, "tools", "ffmpeg")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}

	checksumsPath := filepath.Join(destDir, "checksums.sha256")
	zipPath := filepath.Join(destDir, "ffmpeg.zip")

	if err := downloadWithRetry(ctx, winChecksumsURL, checksumsPath, 3); err != nil {
		if p, pathErr := exec.LookPath("ffmpeg"); pathErr == nil {
			return p, nil
		}
		return "", fmt.Errorf("download checksums: %w", err)
	}
	if err := downloadWithRetry(ctx, winZipURL, zipPath, 3); err != nil {
		if p, pathErr := exec.LookPath("ffmpeg"); pathErr == nil {
			return p, nil
		}
		return "", fmt.Errorf("download ffmpeg zip: %w", err)
	}

	want, err := checksumFor(checksumsPath, filepath.Base(winZipURL))
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

	if err := extractWanted(zipPath, destDir, []string{"ffmpeg.exe", "ffprobe.exe"}); err != nil {
		return "", fmt.Errorf("extract: %w", err)
	}

	_ = os.Remove(zipPath)

	if fileExists(local) {
		return local, nil
	}
	return "", errors.New("ffmpeg download finished, but ffmpeg.exe not found")
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

func download(ctx context.Context, url, out string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 10 * time.Minute}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("download failed: http %d", res.StatusCode)
	}

	tmp := out + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, res.Body); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, out)
}

func downloadWithRetry(ctx context.Context, url, out string, attempts int) error {
	var lastErr error
	for i := 0; i < attempts; i++ {
		if err := download(ctx, url, out); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if i < attempts-1 {
			time.Sleep(2 * time.Second)
		}
	}
	return lastErr
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

// checksums.sha256 lines typically: "<sha256>  <filename>"
func checksumFor(checksumsPath, filename string) (string, error) {
	b, err := os.ReadFile(checksumsPath)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(b), "\n")
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		fields := strings.Fields(ln)
		if len(fields) < 2 {
			continue
		}
		sum := fields[0]
		name := fields[len(fields)-1]
		name = strings.TrimLeft(name, "*")
		if name == filename {
			return sum, nil
		}
	}
	return "", fmt.Errorf("checksum for %s not found", filename)
}

// Zip contains many files; we extract only wanted basenames (case-insensitive).
// In BtbN zips, binaries are usually under some */bin/*.exe path.
func extractWanted(zipPath, destDir string, wanted []string) error {
	want := map[string]bool{}
	for _, w := range wanted {
		want[strings.ToLower(w)] = false
	}

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	for _, f := range zr.File {
		base := strings.ToLower(filepath.Base(f.Name))
		if _, ok := want[base]; !ok {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		outPath := filepath.Join(destDir, base)
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
		want[base] = true
	}

	for k, ok := range want {
		if !ok {
			return fmt.Errorf("missing %s in zip", k)
		}
	}
	return nil
}
