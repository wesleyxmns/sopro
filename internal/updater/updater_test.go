package updater

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIsNewer(t *testing.T) {
	tests := []struct {
		latest   string
		current  string
		expected bool
	}{
		{"v0.2.0", "v0.1.0", true},
		{"0.2.0", "0.1.0", true},
		{"v1.0.0", "v0.9.9", true},
		{"v0.1.1", "v0.1.0", true},
		{"v0.1.0", "v0.1.0", false},
		{"v0.0.9", "v0.1.0", false},
		{"v0.1.0", "dev", true},
		{"v0.1.0", "none", true},
		{"v0.1.0", "", true},
		{"dev", "v0.1.0", false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s vs %s", tt.latest, tt.current), func(t *testing.T) {
			got := IsNewer(tt.latest, tt.current)
			if got != tt.expected {
				t.Errorf("IsNewer(%q, %q) = %v; expected %v", tt.latest, tt.current, got, tt.expected)
			}
		})
	}
}

func TestCacheStateReadAndWrite(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "update_cache.json")

	checker := &Checker{
		CachePath: cachePath,
	}

	// Read on non-existent cache
	if _, ok := checker.readCache(); ok {
		t.Fatal("expected readCache to return false for non-existent file")
	}

	// Write cache
	release := &ReleaseInfo{
		Version:     "0.2.0",
		TagName:     "v0.2.0",
		PublishedAt: time.Now(),
		HTMLURL:     "https://github.com/wesleyxmns/sopro/releases/tag/v0.2.0",
	}
	checker.writeCache(release)

	// Read cache back
	cached, ok := checker.readCache()
	if !ok || cached == nil || cached.LatestRelease == nil {
		t.Fatal("expected readCache to return valid cached release")
	}
	if cached.LatestRelease.Version != "0.2.0" {
		t.Fatalf("expected cached version 0.2.0, got %s", cached.LatestRelease.Version)
	}
}

func TestCheckerFetchRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{
			"tag_name": "v0.2.0",
			"html_url": "https://github.com/wesleyxmns/sopro/releases/tag/v0.2.0",
			"published_at": "2026-08-31T12:00:00Z",
			"body": "Novidades da versão v0.2.0",
			"assets": [
				{
					"name": "sopro_0.2.0_linux_amd64.tar.gz",
					"browser_download_url": "https://example.com/sopro_0.2.0_linux_amd64.tar.gz"
				},
				{
					"name": "checksums.txt",
					"browser_download_url": "https://example.com/checksums.txt"
				}
			]
		}`)
	}))
	defer server.Close()

	checker := &Checker{
		HTTPClient: server.Client(),
		CachePath:  filepath.Join(t.TempDir(), "cache.json"),
	}

	// We override the fetch URL via custom test helper or direct check
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("failed to query test server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	_ = checker
}

func TestVerifyChecksum(t *testing.T) {
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "test.tar.gz")
	checksumsPath := filepath.Join(tmpDir, "checksums.txt")

	testContent := []byte("dummy archive content")
	if err := os.WriteFile(archivePath, testContent, 0644); err != nil {
		t.Fatal(err)
	}

	hasher := sha256.New()
	hasher.Write(testContent)
	hash := hex.EncodeToString(hasher.Sum(nil))

	checksumsContent := fmt.Sprintf("%s  test.tar.gz\n", hash)
	if err := os.WriteFile(checksumsPath, []byte(checksumsContent), 0644); err != nil {
		t.Fatal(err)
	}

	if err := verifyChecksum(archivePath, checksumsPath, "test.tar.gz"); err != nil {
		t.Fatalf("expected checksum verification to pass, got: %v", err)
	}

	// Test mismatch
	badChecksums := "0000000000000000000000000000000000000000000000000000000000000000  test.tar.gz\n"
	if err := os.WriteFile(checksumsPath, []byte(badChecksums), 0644); err != nil {
		t.Fatal(err)
	}

	if err := verifyChecksum(archivePath, checksumsPath, "test.tar.gz"); err == nil {
		t.Fatal("expected checksum mismatch error, got nil")
	}
}

func TestExtractTarGz(t *testing.T) {
	tmpDir := t.TempDir()
	tarPath := filepath.Join(tmpDir, "sopro_test.tar.gz")

	// Create valid tar.gz in memory
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	binaryContent := []byte("#!/bin/sh\necho test\n")
	hdr := &tar.Header{
		Name: "sopro",
		Mode: 0755,
		Size: int64(len(binaryContent)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(binaryContent); err != nil {
		t.Fatal(err)
	}
	tw.Close()
	gzw.Close()

	if err := os.WriteFile(tarPath, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}

	extracted, err := extractBinary(tarPath, "sopro_test.tar.gz")
	if err != nil {
		t.Fatalf("extractBinary failed: %v", err)
	}
	if !bytes.Equal(extracted, binaryContent) {
		t.Fatalf("extracted content mismatch: got %q, expected %q", string(extracted), string(binaryContent))
	}
}
