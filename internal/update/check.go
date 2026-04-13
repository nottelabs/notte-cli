package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	// DefaultGitHubReleasesURL is the endpoint for the latest release.
	DefaultGitHubReleasesURL = "https://api.github.com/repos/nottelabs/notte-cli/releases/latest"
)

// githubReleasesURL is the URL used for checking the latest release.
// It can be overridden in tests via setReleasesURL.
var githubReleasesURL = DefaultGitHubReleasesURL

// ReleaseInfo holds the relevant fields from the GitHub API response.
type ReleaseInfo struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// CheckLatestVersion queries GitHub for the latest release.
// Returns nil, nil if the request fails (failures are silent and non-blocking).
func CheckLatestVersion(ctx context.Context, httpClient *http.Client) (*ReleaseInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubReleasesURL, nil)
	if err != nil {
		return nil, nil //nolint:nilerr // silent failure by design
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "notte-cli-update-checker")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, nil //nolint:nilerr // silent failure by design
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil // silent failure for non-200 responses
	}

	var release ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, nil //nolint:nilerr // silent failure by design
	}

	if release.TagName == "" {
		return nil, fmt.Errorf("empty tag_name in GitHub response")
	}

	return &release, nil
}
