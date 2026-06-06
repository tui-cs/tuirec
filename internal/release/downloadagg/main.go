package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type asset struct {
	target string
	name   string
	output string
}

func main() {
	version := flag.String("version", "v1.10.1-sixel", "agg release version")
	outputDir := flag.String("output", filepath.Join("build", "agg"), "output directory")
	flag.Parse()

	if err := downloadAssets(*version, *outputDir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func downloadAssets(version, outputDir string) error {
	// tig/agg is the sixel-capable fork pinned by pkg/gif.DefaultAggVersion.
	baseURL := fmt.Sprintf("https://github.com/tig/agg/releases/download/%s", version)
	assets := []asset{
		{target: "darwin_amd64", name: "agg-x86_64-apple-darwin", output: "agg"},
		{target: "darwin_arm64", name: "agg-aarch64-apple-darwin", output: "agg"},
		{target: "linux_amd64", name: "agg-x86_64-unknown-linux-musl", output: "agg"},
		{target: "linux_arm64", name: "agg-aarch64-unknown-linux-gnu", output: "agg"},
		{target: "windows_amd64", name: "agg-x86_64-pc-windows-msvc.exe", output: "agg.exe"},
		// The agg release does not publish a native Windows ARM64 binary.
		// Windows ARM64 can run the x64 binary through OS emulation.
		{target: "windows_arm64", name: "agg-x86_64-pc-windows-msvc.exe", output: "agg.exe"},
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	for _, asset := range assets {
		if err := downloadAsset(client, baseURL, outputDir, asset); err != nil {
			return err
		}
	}

	return nil
}

func downloadAsset(client *http.Client, baseURL, outputDir string, asset asset) error {
	targetDir := filepath.Join(outputDir, asset.target)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", targetDir, err)
	}

	url := fmt.Sprintf("%s/%s", baseURL, asset.name)
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", url, resp.Status)
	}

	outputPath := filepath.Join(targetDir, asset.output)
	outputFile, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("create %s: %w", outputPath, err)
	}
	_, copyErr := io.Copy(outputFile, resp.Body)
	closeErr := outputFile.Close()
	if copyErr != nil {
		return fmt.Errorf("write %s: %w", outputPath, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close %s: %w", outputPath, closeErr)
	}

	return nil
}
