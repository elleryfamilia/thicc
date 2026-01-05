package update

import (
	"context"
	"strings"

	"github.com/blang/semver"
	"github.com/ellery/thicc/internal/util"
)

// UpdateInfo contains information about an available update
type UpdateInfo struct {
	CurrentVersion string
	LatestVersion  string
	ReleaseNotes   string
	DownloadSize   int64
	DownloadURL    string
	ChecksumURL    string
}

// CheckForUpdate checks if an update is available for the given channel
// Returns nil if no update is available or an error occurred
func CheckForUpdate(ctx context.Context, channel string) (*UpdateInfo, error) {
	var release *Release
	var err error

	if channel == "nightly" {
		release, err = FetchNightlyRelease(ctx)
	} else {
		release, err = FetchLatestRelease(ctx)
	}

	if err != nil {
		return nil, err
	}

	// No releases available
	if release == nil {
		return nil, nil
	}

	isNewer, err := IsNewerVersion(util.Version, release.TagName, channel)
	if err != nil {
		return nil, err
	}

	if !isNewer {
		return nil, nil
	}

	asset := GetAssetForPlatform(release)
	if asset == nil {
		return nil, nil // No asset for this platform
	}

	info := &UpdateInfo{
		CurrentVersion: util.Version,
		LatestVersion:  release.TagName,
		ReleaseNotes:   release.Body,
		DownloadSize:   asset.Size,
		DownloadURL:    asset.BrowserDownloadURL,
	}

	// Get checksum URL if available
	checksumAsset := GetChecksumAsset(release, asset)
	if checksumAsset != nil {
		info.ChecksumURL = checksumAsset.BrowserDownloadURL
	}

	return info, nil
}

// IsNewerVersion compares two version strings and returns true if latest is newer
func IsNewerVersion(current, latest, channel string) (bool, error) {
	// Clean up version strings
	current = cleanVersion(current)
	latest = cleanVersion(latest)

	// Handle "unknown" or empty versions
	if current == "" || current == "unknown" || current == "0.0.0-unknown" {
		// If we're on an unknown version, don't suggest updates
		// (likely a dev build)
		return false, nil
	}

	// For nightly channel, compare build metadata (timestamps)
	if channel == "nightly" {
		return isNewerNightly(current, latest), nil
	}

	// For stable channel, don't update to nightly versions
	if strings.Contains(latest, "nightly") || strings.Contains(latest, "dev") {
		return false, nil
	}

	currentVer, err := semver.ParseTolerant(current)
	if err != nil {
		return false, err
	}

	latestVer, err := semver.ParseTolerant(latest)
	if err != nil {
		return false, err
	}

	return latestVer.GT(currentVer), nil
}

// isNewerNightly compares nightly versions by their build timestamp
// Nightly versions look like: v0.1.3-dev.6+202601050002
func isNewerNightly(current, latest string) bool {
	// Extract build metadata (after the +)
	currentBuild := extractBuildMetadata(current)
	latestBuild := extractBuildMetadata(latest)

	// If we can't extract build metadata, compare as strings
	if currentBuild == "" || latestBuild == "" {
		return latest > current
	}

	// Build metadata is a timestamp like 202601050002
	// Simple string comparison works because format is YYYYMMDDHHMM
	return latestBuild > currentBuild
}

// extractBuildMetadata extracts the build metadata from a version string
// e.g., "0.1.3-dev.6+202601050002" -> "202601050002"
func extractBuildMetadata(version string) string {
	if idx := strings.LastIndex(version, "+"); idx != -1 {
		return version[idx+1:]
	}
	return ""
}

// cleanVersion removes common prefixes and cleans up version strings
func cleanVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	return v
}
