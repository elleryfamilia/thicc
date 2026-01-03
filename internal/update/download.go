package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// DownloadAndInstall downloads and installs a new version of thicc
func DownloadAndInstall(info *UpdateInfo, progressFn func(downloaded, total int64)) error {
	// Get current executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("failed to resolve symlinks: %w", err)
	}

	// Create temp directory for download
	tempDir, err := os.MkdirTemp("", "thicc-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Download the archive
	archivePath := filepath.Join(tempDir, "thicc-archive")
	if err := downloadFile(info.DownloadURL, archivePath, info.DownloadSize, progressFn); err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}

	// Verify checksum if available
	if info.ChecksumURL != "" {
		if err := verifyChecksum(archivePath, info.ChecksumURL); err != nil {
			return fmt.Errorf("checksum verification failed: %w", err)
		}
	}

	// Extract binary from archive
	binaryPath, err := extractBinary(archivePath, tempDir, info.DownloadURL)
	if err != nil {
		return fmt.Errorf("failed to extract binary: %w", err)
	}

	// Replace the current binary
	if err := replaceBinary(execPath, binaryPath); err != nil {
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	return nil
}

// downloadFile downloads a file with progress reporting
func downloadFile(url, destPath string, expectedSize int64, progressFn func(downloaded, total int64)) error {
	client := &http.Client{
		Timeout: 5 * time.Minute,
	}

	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %s", resp.Status)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Use expected size from asset if content-length not provided
	total := resp.ContentLength
	if total <= 0 {
		total = expectedSize
	}

	if progressFn != nil {
		reader := &progressReader{
			reader:     resp.Body,
			total:      total,
			progressFn: progressFn,
		}
		_, err = io.Copy(out, reader)
	} else {
		_, err = io.Copy(out, resp.Body)
	}

	return err
}

type progressReader struct {
	reader     io.Reader
	downloaded int64
	total      int64
	progressFn func(downloaded, total int64)
}

func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.downloaded += int64(n)
	if r.progressFn != nil {
		r.progressFn(r.downloaded, r.total)
	}
	return n, err
}

// verifyChecksum downloads the checksum file and verifies the archive
func verifyChecksum(archivePath, checksumURL string) error {
	// Download checksum
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(checksumURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download checksum: %s", resp.Status)
	}

	checksumData, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Parse expected checksum (format: "hash  filename" or just "hash")
	parts := strings.Fields(string(checksumData))
	if len(parts) == 0 {
		return fmt.Errorf("empty checksum file")
	}
	expectedHash := strings.ToLower(parts[0])

	// Calculate actual checksum
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	actualHash := hex.EncodeToString(h.Sum(nil))

	if actualHash != expectedHash {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	return nil
}

// extractBinary extracts the thicc binary from the archive
func extractBinary(archivePath, tempDir, downloadURL string) (string, error) {
	// Determine archive type from URL
	isZip := strings.HasSuffix(downloadURL, ".zip")

	if isZip {
		return extractFromZip(archivePath, tempDir)
	}
	return extractFromTarGz(archivePath, tempDir)
}

// extractFromZip extracts the thicc binary from a zip archive
func extractFromZip(archivePath, tempDir string) (string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	binaryName := "thicc"
	if runtime.GOOS == "windows" {
		binaryName = "thicc.exe"
	}

	for _, f := range r.File {
		if filepath.Base(f.Name) == binaryName && !f.FileInfo().IsDir() {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			defer rc.Close()

			outPath := filepath.Join(tempDir, binaryName)
			out, err := os.Create(outPath)
			if err != nil {
				return "", err
			}
			defer out.Close()

			if _, err := io.Copy(out, rc); err != nil {
				return "", err
			}

			return outPath, nil
		}
	}

	return "", fmt.Errorf("binary not found in archive")
}

// extractFromTarGz extracts the thicc binary from a tar.gz archive
func extractFromTarGz(archivePath, tempDir string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	binaryName := "thicc"
	if runtime.GOOS == "windows" {
		binaryName = "thicc.exe"
	}

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		if filepath.Base(hdr.Name) == binaryName && hdr.Typeflag == tar.TypeReg {
			outPath := filepath.Join(tempDir, binaryName)
			out, err := os.Create(outPath)
			if err != nil {
				return "", err
			}
			defer out.Close()

			if _, err := io.Copy(out, tr); err != nil {
				return "", err
			}

			return outPath, nil
		}
	}

	return "", fmt.Errorf("binary not found in archive")
}

// replaceBinary replaces the current binary with the new one
func replaceBinary(oldPath, newPath string) error {
	// Get file info from old binary to preserve permissions
	oldInfo, err := os.Stat(oldPath)
	if err != nil {
		return err
	}
	oldMode := oldInfo.Mode()

	// Check if we can write to the directory
	dir := filepath.Dir(oldPath)
	if err := checkWritePermission(dir); err != nil {
		return fmt.Errorf("no write permission to %s: %w (try running with sudo or reinstalling to a user-writable location)", dir, err)
	}

	// Create backup
	backupPath := oldPath + ".backup"
	if err := os.Rename(oldPath, backupPath); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	// Copy new binary
	if err := copyFile(newPath, oldPath); err != nil {
		// Restore backup on failure
		os.Rename(backupPath, oldPath)
		return fmt.Errorf("failed to copy new binary: %w", err)
	}

	// Restore original permissions
	if err := os.Chmod(oldPath, oldMode); err != nil {
		// Try to restore backup
		os.Rename(backupPath, oldPath)
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Remove backup
	os.Remove(backupPath)

	return nil
}

// checkWritePermission checks if we have write permission to a directory
func checkWritePermission(dir string) error {
	testFile := filepath.Join(dir, ".thicc-update-test")
	f, err := os.Create(testFile)
	if err != nil {
		return err
	}
	f.Close()
	os.Remove(testFile)
	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return out.Close()
}
