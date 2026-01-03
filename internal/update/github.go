package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"
)

const (
	repoOwner   = "elleryfamilia"
	repoName    = "thicc"
	releasesURL = "https://api.github.com/repos/%s/%s/releases"
)

// Release represents a GitHub release
type Release struct {
	TagName    string  `json:"tag_name"`
	Name       string  `json:"name"`
	Body       string  `json:"body"`
	Prerelease bool    `json:"prerelease"`
	Assets     []Asset `json:"assets"`
}

// Asset represents a release asset (downloadable file)
type Asset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// FetchLatestRelease fetches the latest stable release from GitHub
// It returns the first non-prerelease release, or nil if none found
func FetchLatestRelease(ctx context.Context) (*Release, error) {
	// First try the /latest endpoint which only returns non-prereleases
	url := fmt.Sprintf(releasesURL+"/latest", repoOwner, repoName)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "thicc-update-checker")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// If /latest returns 404, there may be only prereleases
	// Fall back to fetching all releases
	if resp.StatusCode == http.StatusNotFound {
		return fetchFirstStableRelease(ctx)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error: %s - %s", resp.Status, string(body))
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

// fetchFirstStableRelease fetches all releases and returns the first non-prerelease
func fetchFirstStableRelease(ctx context.Context) (*Release, error) {
	url := fmt.Sprintf(releasesURL, repoOwner, repoName)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "thicc-update-checker")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error: %s - %s", resp.Status, string(body))
	}

	var releases []Release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}

	// Find first non-prerelease
	for i := range releases {
		if !releases[i].Prerelease {
			return &releases[i], nil
		}
	}

	// No stable releases found
	return nil, nil
}

// GetPlatformAssetSuffix returns the asset suffix for the current platform
func GetPlatformAssetSuffix() string {
	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			return "macos-arm64.tar.gz"
		}
		return "osx.tar.gz"
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			return "linux64.tar.gz"
		case "386":
			return "linux32.tar.gz"
		case "arm":
			return "linux-arm.tar.gz"
		case "arm64":
			return "linux-arm64.tar.gz"
		}
	case "windows":
		switch runtime.GOARCH {
		case "amd64":
			return "win64.zip"
		case "arm64":
			return "win-arm64.zip"
		case "386":
			return "win32.zip"
		}
	case "freebsd":
		switch runtime.GOARCH {
		case "amd64":
			return "freebsd64.tar.gz"
		case "386":
			return "freebsd32.tar.gz"
		}
	case "openbsd":
		switch runtime.GOARCH {
		case "amd64":
			return "openbsd64.tar.gz"
		case "386":
			return "openbsd32.tar.gz"
		}
	case "netbsd":
		switch runtime.GOARCH {
		case "amd64":
			return "netbsd64.tar.gz"
		case "386":
			return "netbsd32.tar.gz"
		}
	}
	return ""
}

// GetAssetForPlatform finds the correct asset for the current platform
func GetAssetForPlatform(release *Release) *Asset {
	suffix := GetPlatformAssetSuffix()
	if suffix == "" {
		return nil
	}

	for i := range release.Assets {
		if strings.HasSuffix(release.Assets[i].Name, suffix) {
			return &release.Assets[i]
		}
	}

	return nil
}

// GetChecksumAsset finds the SHA checksum file for an asset
func GetChecksumAsset(release *Release, asset *Asset) *Asset {
	shaName := asset.Name + ".sha"
	for i := range release.Assets {
		if release.Assets[i].Name == shaName {
			return &release.Assets[i]
		}
	}
	return nil
}
