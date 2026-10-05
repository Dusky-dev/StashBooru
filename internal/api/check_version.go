package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"golang.org/x/sys/cpu"

	"github.com/stashapp/stash/internal/build"
	"github.com/stashapp/stash/pkg/logger"
)

// we use the github REST V3 API as no login is required
const apiRepoBase string = "https://api.github.com/repos/"
const apiAcceptHeader string = "application/vnd.github.v3+json"
const developmentTag string = "latest_develop"
const defaultSHLength int = 8 // default length of SHA short hash returned by <git rev-parse --short HEAD>

// apiReleasesURL and apiTagsURL build the GitHub API endpoints for the
// repository configured via build.UpdateRepo (defaults to Dusky-dev/StashBooru).
func apiReleasesURL() string {
	return apiRepoBase + build.UpdateRepo() + "/releases"
}

func apiTagsURL() string {
	return apiRepoBase + build.UpdateRepo() + "/tags"
}

var stashReleases = func() map[string]string {
	return map[string]string{
		"darwin/amd64":  "stash-macos",
		"darwin/arm64":  "stash-macos",
		"linux/amd64":   "stash-linux",
		"windows/amd64": "stash-win.exe",
		"linux/arm":     "stash-linux-arm32v6",
		"linux/arm64":   "stash-linux-arm64v8",
		"linux/armv7":   "stash-linux-arm32v7",
	}
}

// isMacOSBundle checks if the application is running from within a macOS .app bundle
func isMacOSBundle() bool {
	exec, err := os.Executable()
	return err == nil && strings.Contains(exec, "Stash.app/")
}

// getWantedRelease determines which release variant to download based on platform and bundle type
func getWantedRelease(platform string) string {
	release := stashReleases()[platform]

	// On macOS, check if running from .app bundle
	if runtime.GOOS == "darwin" && isMacOSBundle() {
		return "Stash.app.zip"
	}

	return release
}

type githubReleasesResponse struct {
	Url              string
	Assets_url       string
	Upload_url       string
	Html_url         string
	Id               int64
	Node_id          string
	Tag_name         string
	Target_commitish string
	Name             string
	Draft            bool
	Author           githubAuthor
	Prerelease       bool
	Created_at       string
	Published_at     string
	Assets           []githubAsset
	Tarball_url      string
	Zipball_url      string
	Body             string
}

type githubAuthor struct {
	Login               string
	Id                  int64
	Node_id             string
	Avatar_url          string
	Gravatar_id         string
	Url                 string
	Html_url            string
	Followers_url       string
	Following_url       string
	Gists_url           string
	Starred_url         string
	Subscriptions_url   string
	Organizations_url   string
	Repos_url           string
	Events_url          string
	Received_events_url string
	Type                string
	Site_admin          bool
}

type githubAsset struct {
	Url                  string
	Id                   int64
	Node_id              string
	Name                 string
	Label                string
	Uploader             githubAuthor
	Content_type         string
	State                string
	Size                 int64
	Download_count       int64
	Created_at           string
	Updated_at           string
	Browser_download_url string
}

type githubTagResponse struct {
	Name        string
	Zipball_url string
	Tarball_url string
	Commit      struct {
		Sha string
		Url string
	}
	Node_id string
}

type LatestRelease struct {
	Version   string
	Hash      string
	ShortHash string
	Date      string
	Url       string
}

func makeGithubRequest(ctx context.Context, url string, output interface{}) error {
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment}

	client := &http.Client{
		Timeout:   3 * time.Second,
		Transport: transport,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	req.Header.Add("Accept", apiAcceptHeader) // gh api recommendation , send header with api version
	logger.Debugf("Github API request: %s", url)
	response, err := client.Do(req)

	if err != nil {
		//nolint:staticcheck // ST1005 Github is a proper capitalized noun
		return fmt.Errorf("Github API request failed: %w", err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		//nolint:staticcheck // ST1005 Github is a proper capitalized noun
		return fmt.Errorf("Github API request failed: %s", response.Status)
	}

	data, err := io.ReadAll(response.Body)
	if err != nil {
		//nolint:staticcheck // ST1005 Github is a proper capitalized noun
		return fmt.Errorf("Github API read response failed: %w", err)
	}

	err = json.Unmarshal(data, output)
	if err != nil {
		return fmt.Errorf("unmarshalling Github API response failed: %w", err)
	}

	return nil
}

// GetLatestRelease checks stable StashBooru releases, independently of the
// upstream Stash version and the latest_develop snapshot.
func GetLatestRelease(ctx context.Context) (*LatestRelease, error) {
	arch := runtime.GOARCH

	// https://en.wikipedia.org/wiki/Comparison_of_ARM_cores
	// armv6 doesn't support any of these features
	isARMv7 := cpu.ARM.HasNEON || cpu.ARM.HasVFPv3 || cpu.ARM.HasVFPv3D16 || cpu.ARM.HasVFPv4
	if arch == "arm" && isARMv7 {
		arch = "armv7"
	}

	platform := fmt.Sprintf("%s/%s", runtime.GOOS, arch)
	wantedRelease := getWantedRelease(platform)

	url := apiReleasesURL() + "/latest"

	var release githubReleasesResponse
	err := makeGithubRequest(ctx, url, &release)
	if err != nil {
		return nil, err
	}

	version, err := stashBooruReleaseVersion(release)
	if err != nil {
		return nil, err
	}

	latestHash, err := getReleaseHash(ctx, release.Tag_name)
	if err != nil {
		return nil, err
	}

	var releaseDate string
	if publishedAt, err := time.Parse(time.RFC3339, release.Published_at); err == nil {
		releaseDate = publishedAt.Format("2006-01-02")
	}

	releaseUrl := release.Html_url
	if wantedRelease != "" {
		for _, asset := range release.Assets {
			if asset.Name == wantedRelease {
				releaseUrl = asset.Browser_download_url
				break
			}
		}
	}

	_, githash, _ := build.Version()
	shLength := len(githash)
	if shLength == 0 {
		shLength = defaultSHLength
	}

	return &LatestRelease{
		Version:   version,
		Hash:      latestHash,
		ShortHash: latestHash[:min(shLength, len(latestHash))],
		Date:      releaseDate,
		Url:       releaseUrl,
	}, nil
}

func getReleaseHash(ctx context.Context, tagName string) (string, error) {
	// Start with a small page size if not searching for latest_develop
	perPage := 10
	if tagName == developmentTag {
		perPage = 100
	}

	// Limit to 5 pages, ie 500 tags - should be plenty
	for page := 1; page <= 5; {
		url := fmt.Sprintf("%s?per_page=%d&page=%d", apiTagsURL(), perPage, page)
		tags := []githubTagResponse{}
		err := makeGithubRequest(ctx, url, &tags)
		if err != nil {
			return "", err
		}

		for _, tag := range tags {
			if tag.Name == tagName {
				if len(tag.Commit.Sha) != 40 {
					return "", errors.New("invalid Github API response")
				}
				return tag.Commit.Sha, nil
			}
		}

		if len(tags) == 0 {
			break
		}

		// if not found in the first 10, search again on page 1 with the first 100
		if perPage == 10 {
			perPage = 100
		} else {
			page++
		}
	}

	return "", errors.New("invalid Github API response")
}

func stashBooruReleaseVersion(release githubReleasesResponse) (string, error) {
	const prefix = "stashbooru-v"
	version := strings.TrimPrefix(release.Tag_name, prefix)
	if release.Draft || release.Prerelease || !strings.HasPrefix(release.Tag_name, prefix) {
		return "", errors.New("no stable StashBooru release is available")
	}
	if _, valid := build.CompareStableVersions(version, build.StashBooruVersion()); !valid {
		return "", errors.New("invalid StashBooru release version")
	}
	return version, nil
}

func printLatestVersion(ctx context.Context) {
	latestRelease, err := GetLatestRelease(ctx)
	if err != nil {
		logger.Warnf("Couldn't check StashBooru updates: %v", err)
		return
	}
	comparison, valid := build.CompareStableVersions(latestRelease.Version, build.StashBooruVersion())
	if valid && comparison > 0 {
		logger.Infof("New StashBooru version available: %s (%s)", latestRelease.Version, latestRelease.Url)
	} else {
		logger.Infof("StashBooru %s; latest stable release: %s", build.StashBooruVersion(), latestRelease.Version)
	}
}
