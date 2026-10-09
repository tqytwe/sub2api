package service

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrForkSourceDeploymentRequired = infraerrors.Conflict("FORK_SOURCE_DEPLOYMENT_REQUIRED", "no verified tqytwe build is approved for in-place installation; build and deploy reviewed tqytwe/sub2api play/main source; see https://github.com/tqytwe/sub2api/blob/play/main/deploy/FORK_SOURCE_BUILD.md")
	ErrNoUpdateAvailable            = infraerrors.Conflict("ALREADY_UP_TO_DATE", "no update available; current version is latest")
	ErrRollbackVersionNotAllowed    = infraerrors.BadRequest("ROLLBACK_VERSION_NOT_ALLOWED", "version is not in the allowed rollback list")
)

const (
	updateCacheTTL     = 1200 // 20 minutes
	upstreamRepository = "ranxi2001/sub2api"
	forkRepository     = "tqytwe/sub2api"
	// This is the reviewed source pin, not the installed fork's version number.
	upstreamReviewVersion = "2.10.3"
	// Change this identity when source, review pin or installation policy changes.
	UpdateSourceIdentity = upstreamRepository + ":stable:v" + upstreamReviewVersion + "|" + forkRepository + ":source-only:v1"
)

var stableReleaseTag = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)

// UpdateCache defines cache operations for update service
type UpdateCache interface {
	GetUpdateInfo(ctx context.Context) (string, error)
	SetUpdateInfo(ctx context.Context, data string, ttl time.Duration) error
}

// GitHubReleaseClient 获取 GitHub release 信息的接口
type GitHubReleaseClient interface {
	FetchLatestRelease(ctx context.Context, repo string) (*GitHubRelease, error)
	FetchRecentReleases(ctx context.Context, repo string, perPage int) ([]*GitHubRelease, error)
	DownloadFile(ctx context.Context, url, dest string, maxSize int64) error
	FetchChecksumFile(ctx context.Context, url string) ([]byte, error)
}

// UpdateService handles software updates
type UpdateService struct {
	cache          UpdateCache
	githubClient   GitHubReleaseClient
	currentVersion string
	buildType      string // "source" for manual builds, "release" for CI builds
}

// NewUpdateService creates a new UpdateService
func NewUpdateService(cache UpdateCache, githubClient GitHubReleaseClient, version, buildType string) *UpdateService {
	return &UpdateService{
		cache:          cache,
		githubClient:   githubClient,
		currentVersion: version,
		buildType:      buildType,
	}
}

// UpdateInfo contains update information
type UpdateInfo struct {
	UpstreamRepository string       `json:"upstream_repository"`
	UpstreamBaseline   string       `json:"upstream_baseline"`
	InstallRepository  string       `json:"install_repository"`
	InstallSupported   bool         `json:"install_supported"`
	DeploymentGuideURL string       `json:"deployment_guide_url"`
	CurrentVersion     string       `json:"current_version"`
	LatestVersion      string       `json:"latest_version"`
	HasUpdate          bool         `json:"has_update"`
	ReleaseInfo        *ReleaseInfo `json:"release_info,omitempty"`
	Cached             bool         `json:"cached"`
	Warning            string       `json:"warning,omitempty"`
	BuildType          string       `json:"build_type"` // "source" or "release"
}

// ReleaseInfo contains GitHub release details
type ReleaseInfo struct {
	Name        string  `json:"name"`
	Body        string  `json:"body"`
	PublishedAt string  `json:"published_at"`
	HTMLURL     string  `json:"html_url"`
	Assets      []Asset `json:"assets,omitempty"`
}

// Asset represents a release asset
type Asset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"download_url"`
	Size        int64  `json:"size"`
}

// GitHubRelease represents GitHub API response
type GitHubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Body        string        `json:"body"`
	PublishedAt string        `json:"published_at"`
	HTMLURL     string        `json:"html_url"`
	Draft       bool          `json:"draft"`
	Prerelease  bool          `json:"prerelease"`
	Assets      []GitHubAsset `json:"assets"`
}

// RollbackVersion describes a release version the system can roll back to
type RollbackVersion struct {
	Version     string `json:"version"` // without "v" prefix, e.g. "0.1.146"
	PublishedAt string `json:"published_at"`
	HTMLURL     string `json:"html_url"`
}

type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// CheckUpdate checks for available updates
func (s *UpdateService) CheckUpdate(ctx context.Context, force bool) (*UpdateInfo, error) {
	// Try cache first
	if !force {
		if cached, err := s.getFromCache(ctx); err == nil && cached != nil {
			return cached, nil
		}
	}

	// Fetch from GitHub
	info, err := s.fetchLatestRelease(ctx)
	if err != nil {
		// Return cached on error
		if cached, cacheErr := s.getFromCache(ctx); cacheErr == nil && cached != nil {
			cached.Warning = "Using cached data: " + err.Error()
			return cached, nil
		}
		info := s.baseInfo()
		info.Warning = err.Error()
		return info, nil
	}

	// Cache result
	s.saveToCache(ctx, info)
	return info, nil
}

// In-place binary replacement is deliberately unavailable until a reviewed fork
// artifact trust policy exists. BuildType=release is not provenance: historical
// CI/Docker builds used that flag while downloading the original upstream binary.
// Every API entry point fails before network or executable/backup file access.
func (s *UpdateService) PerformUpdate(context.Context) error {
	return ErrForkSourceDeploymentRequired
}

func (s *UpdateService) Rollback() error {
	return ErrForkSourceDeploymentRequired
}

func (s *UpdateService) RollbackToVersion(context.Context, string) error {
	return ErrForkSourceDeploymentRequired
}

func (s *UpdateService) ListRollbackVersions(context.Context) ([]RollbackVersion, error) {
	return []RollbackVersion{}, nil
}

func (s *UpdateService) baseInfo() *UpdateInfo {
	return &UpdateInfo{
		CurrentVersion:     s.currentVersion,
		BuildType:          s.buildType,
		UpstreamRepository: upstreamRepository,
		UpstreamBaseline:   upstreamReviewVersion,
		InstallRepository:  forkRepository,
		InstallSupported:   false,
		DeploymentGuideURL: "https://github.com/tqytwe/sub2api/blob/play/main/deploy/FORK_SOURCE_BUILD.md",
	}
}

func validUpstreamRelease(tag, htmlURL string) bool {
	return stableReleaseTag.MatchString(tag) && htmlURL == "https://github.com/"+upstreamRepository+"/releases/tag/"+tag
}

func (s *UpdateService) fetchLatestRelease(ctx context.Context) (*UpdateInfo, error) {
	release, err := s.githubClient.FetchLatestRelease(ctx, upstreamRepository)
	if err != nil {
		return nil, err
	}
	if release == nil || release.Draft || release.Prerelease || !validUpstreamRelease(release.TagName, release.HTMLURL) {
		return nil, fmt.Errorf("expected a stable release from %s", upstreamRepository)
	}
	info := s.baseInfo()
	info.LatestVersion = strings.TrimPrefix(release.TagName, "v")
	info.HasUpdate = compareVersions(upstreamReviewVersion, info.LatestVersion) < 0
	// Discovery data must never carry upstream binaries into an installation flow.
	info.ReleaseInfo = &ReleaseInfo{Name: release.Name, Body: release.Body, PublishedAt: release.PublishedAt, HTMLURL: release.HTMLURL}
	return info, nil
}

func (s *UpdateService) getFromCache(ctx context.Context) (*UpdateInfo, error) {
	data, err := s.cache.GetUpdateInfo(ctx)
	if err != nil {
		return nil, err
	}

	var cached struct {
		SourceIdentity string       `json:"source_identity"`
		Latest         string       `json:"latest"`
		ReleaseInfo    *ReleaseInfo `json:"release_info"`
		Timestamp      int64        `json:"timestamp"`
	}
	if err := json.Unmarshal([]byte(data), &cached); err != nil {
		return nil, err
	}

	if cached.SourceIdentity != UpdateSourceIdentity || cached.ReleaseInfo == nil ||
		!validUpstreamRelease("v"+cached.Latest, cached.ReleaseInfo.HTMLURL) {
		return nil, fmt.Errorf("update cache source identity mismatch")
	}
	if time.Now().Unix()-cached.Timestamp > updateCacheTTL || cached.Timestamp > time.Now().Unix() {
		return nil, fmt.Errorf("cache expired")
	}

	info := s.baseInfo()
	info.LatestVersion = cached.Latest
	info.HasUpdate = compareVersions(upstreamReviewVersion, cached.Latest) < 0
	cached.ReleaseInfo.Assets = nil
	info.ReleaseInfo = cached.ReleaseInfo
	info.Cached = true
	return info, nil
}

func (s *UpdateService) saveToCache(ctx context.Context, info *UpdateInfo) {
	cacheData := struct {
		SourceIdentity string       `json:"source_identity"`
		Latest         string       `json:"latest"`
		ReleaseInfo    *ReleaseInfo `json:"release_info"`
		Timestamp      int64        `json:"timestamp"`
	}{
		SourceIdentity: UpdateSourceIdentity,
		Latest:         info.LatestVersion,
		ReleaseInfo:    info.ReleaseInfo,
		Timestamp:      time.Now().Unix(),
	}

	data, _ := json.Marshal(cacheData)
	_ = s.cache.SetUpdateInfo(ctx, string(data), time.Duration(updateCacheTTL)*time.Second)
}

// compareVersions compares two semantic versions
func compareVersions(current, latest string) int {
	currentParts := parseVersion(current)
	latestParts := parseVersion(latest)

	for i := 0; i < 3; i++ {
		if currentParts[i] < latestParts[i] {
			return -1
		}
		if currentParts[i] > latestParts[i] {
			return 1
		}
	}
	return 0
}

func parseVersion(v string) [3]int {
	v = strings.TrimPrefix(v, "v")
	if idx := strings.IndexByte(v, '-'); idx != -1 {
		v = v[:idx]
	}
	parts := strings.Split(v, ".")
	result := [3]int{0, 0, 0}
	for i := 0; i < len(parts) && i < 3; i++ {
		if parsed, err := strconv.Atoi(parts[i]); err == nil {
			result[i] = parsed
		}
	}
	return result
}
