package gif

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const (
	// DefaultAggVersion is the agg release version downloaded when auto-fetching.
	// The fork (see aggReleaseRepo) ships sixel and Kitty graphics rendering;
	// pinned here until that support lands in an upstream asciinema/agg release.
	DefaultAggVersion = "v1.11.1-sixel"

	// aggReleaseRepo is the GitHub owner/name the agg binary is fetched from.
	aggReleaseRepo = "tig/agg"

	aggCacheDirName = "tuirec"
)

// ErrDownload indicates that automatic agg download failed.
var ErrDownload = fmt.Errorf("agg download failed")

// aggAssetName returns the platform-specific agg binary name from the GitHub release.
func aggAssetName() (string, error) {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "darwin/amd64":
		return "agg-x86_64-apple-darwin", nil
	case "darwin/arm64":
		return "agg-aarch64-apple-darwin", nil
	case "linux/amd64":
		return "agg-x86_64-unknown-linux-musl", nil
	case "linux/arm64":
		return "agg-aarch64-unknown-linux-gnu", nil
	case "windows/amd64", "windows/arm64":
		return "agg-x86_64-pc-windows-msvc.exe", nil
	default:
		return "", fmt.Errorf("%w: unsupported platform %s/%s", ErrDownload, runtime.GOOS, runtime.GOARCH)
	}
}

// aggOutputName returns the local binary name for agg on the current OS.
func aggOutputName() string {
	if runtime.GOOS == "windows" {
		return "agg.exe"
	}
	return "agg"
}

// CacheDir returns the default cache directory for auto-downloaded agg.
func CacheDir() string {
	base, err := os.UserCacheDir()
	if err != nil {
		base = filepath.Join(os.TempDir(), ".cache")
	}
	return filepath.Join(base, aggCacheDirName, "agg-"+DefaultAggVersion)
}

// CachedAggPath returns the expected path of the cached agg binary.
func CachedAggPath() string {
	return filepath.Join(CacheDir(), aggOutputName())
}

// DownloadAgg downloads the agg binary for the current platform to the cache dir.
// Returns the path to the downloaded binary.
func DownloadAgg() (string, error) {
	return downloadAggWith(http.DefaultClient)
}

func downloadAggWith(client *http.Client) (string, error) {
	assetName, err := aggAssetName()
	if err != nil {
		return "", err
	}

	cacheDir := CacheDir()
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", fmt.Errorf("%w: create cache dir: %w", ErrDownload, err)
	}

	outputPath := filepath.Join(cacheDir, aggOutputName())

	// Already cached — skip download.
	if _, err := os.Stat(outputPath); err == nil {
		return outputPath, nil
	}

	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", aggReleaseRepo, DefaultAggVersion, assetName)

	if client.Timeout == 0 {
		client = &http.Client{Timeout: 5 * time.Minute}
	}

	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("%w: GET %s: %w", ErrDownload, url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: GET %s: %s", ErrDownload, url, resp.Status)
	}

	tmpFile, err := os.CreateTemp(cacheDir, "agg-download-*")
	if err != nil {
		return "", fmt.Errorf("%w: create temp file: %w", ErrDownload, err)
	}
	tmpPath := tmpFile.Name()

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("%w: write agg binary: %w", ErrDownload, err)
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("%w: close temp file: %w", ErrDownload, err)
	}

	if err := os.Chmod(tmpPath, 0o755); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("%w: chmod: %w", ErrDownload, err)
	}

	if err := os.Rename(tmpPath, outputPath); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("%w: rename to final path: %w", ErrDownload, err)
	}

	return outputPath, nil
}
