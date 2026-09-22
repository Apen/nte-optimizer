package updatecheck

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const latestReleaseURL = "https://api.github.com/repos/Apen/nte-optimizer/releases/latest"

type Result struct {
	Available      bool   `json:"available"`
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version,omitempty"`
	DownloadURL    string `json:"download_url,omitempty"`
}

type release struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func Check(ctx context.Context, client *http.Client, currentVersion string) (Result, error) {
	result := Result{CurrentVersion: currentVersion}
	if isDevelopmentVersion(currentVersion) {
		return result, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestReleaseURL, nil)
	if err != nil {
		return result, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "nte-optimizer/"+currentVersion)
	response, err := client.Do(req)
	if err != nil {
		return result, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return result, fmt.Errorf("GitHub releases returned %s", response.Status)
	}
	var latest release
	if err := json.NewDecoder(response.Body).Decode(&latest); err != nil {
		return result, fmt.Errorf("decode latest release: %w", err)
	}
	result.LatestVersion = latest.TagName
	result.DownloadURL = releaseDownloadURL(latest)
	result.Available = compareVersions(latest.TagName, currentVersion) > 0 && result.DownloadURL != ""
	return result, nil
}

func HTTPClient() *http.Client {
	return &http.Client{Timeout: 5 * time.Second}
}

func isDevelopmentVersion(version string) bool {
	version = strings.TrimSpace(version)
	return version == "" || strings.EqualFold(version, "dev") || strings.EqualFold(version, "unknown")
}

func releaseDownloadURL(value release) string {
	for _, asset := range value.Assets {
		name := strings.ToLower(asset.Name)
		if strings.Contains(name, "windows") && strings.HasSuffix(name, ".zip") && asset.BrowserDownloadURL != "" {
			return asset.BrowserDownloadURL
		}
	}
	return value.HTMLURL
}

func compareVersions(left, right string) int {
	a := versionParts(left)
	b := versionParts(right)
	for index := 0; index < 3; index++ {
		if a[index] < b[index] {
			return -1
		}
		if a[index] > b[index] {
			return 1
		}
	}
	return 0
}

func versionParts(value string) [3]int {
	value = strings.TrimPrefix(strings.TrimSpace(value), "v")
	value = strings.SplitN(value, "-", 2)[0]
	parts := strings.Split(value, ".")
	var result [3]int
	for index := 0; index < len(parts) && index < len(result); index++ {
		result[index], _ = strconv.Atoi(parts[index])
	}
	return result
}
