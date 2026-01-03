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

// CheckForUpdate checks if an update is available
// Returns nil if no update is available or an error occurred
func CheckForUpdate(ctx context.Context) (*UpdateInfo, error) {
	release, err := FetchLatestRelease(ctx)
	if err != nil {
		return nil, err
	}

	// No stable releases available
	if release == nil {
		return nil, nil
	}

	isNewer, err := IsNewerVersion(util.Version, release.TagName)
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
func IsNewerVersion(current, latest string) (bool, error) {
	// Clean up version strings
	current = cleanVersion(current)
	latest = cleanVersion(latest)

	// Handle "unknown" or empty versions
	if current == "" || current == "unknown" || current == "0.0.0-unknown" {
		// If we're on an unknown version, don't suggest updates
		// (likely a dev build)
		return false, nil
	}

	// Handle nightly/dev versions specially
	if strings.Contains(latest, "nightly") {
		// Don't auto-update to nightly from stable
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

// cleanVersion removes common prefixes and cleans up version strings
func cleanVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	return v
}
