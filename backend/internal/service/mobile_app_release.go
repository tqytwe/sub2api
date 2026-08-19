package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	MobileReleaseDistributionDirect = "direct"
	MobileReleaseDistributionPlay   = "play"
	MobileReleaseArtifactAPK        = "apk"
	MobileReleaseArtifactAAB        = "aab"
	MobileReleaseStatusDraft        = "draft"
	MobileReleaseStatusReady        = "ready"
	MobileReleaseStatusPublished    = "published"
	MobileReleaseStatusPaused       = "paused"
	MobileReleaseStatusRetired      = "retired"
)

var mobileReleaseHex64 = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)

var ErrMobileReleaseNotFound = infraerrors.NotFound("MOBILE_RELEASE_NOT_FOUND", "mobile release not found")

type MobileReleaseManifest struct {
	Platform                 string              `json:"platform"`
	Version                  string              `json:"version"`
	VersionCode              int64               `json:"versionCode"`
	Distribution             string              `json:"distribution,omitempty"`
	Channel                  string              `json:"channel,omitempty"`
	ArtifactType             string              `json:"artifactType"`
	PackageName              string              `json:"packageName"`
	APKURL                   string              `json:"apkUrl,omitempty"`
	Bytes                    int64               `json:"bytes"`
	SHA256                   string              `json:"sha256"`
	SigningCertificateSHA256 string              `json:"signingCertificateSha256"`
	MinAndroidVersion        string              `json:"minAndroidVersion,omitempty"`
	ReleaseDate              string              `json:"releaseDate,omitempty"`
	SourceCommit             string              `json:"sourceCommit,omitempty"`
	BuiltFromCommit          string              `json:"builtFromCommit,omitempty"`
	Notes                    []string            `json:"notes"`
	NotesI18n                map[string][]string `json:"notes_i18n"`
	LegacyNotesByLocale      map[string][]string `json:"notesByLocale,omitempty"`
}

type MobileAppRelease struct {
	ID                       int64                 `json:"id"`
	Distribution             string                `json:"distribution"`
	PackageName              string                `json:"package_name"`
	ArtifactType             string                `json:"artifact_type"`
	Version                  string                `json:"version"`
	VersionCode              int64                 `json:"version_code"`
	StorageKey               string                `json:"-"`
	DownloadURL              string                `json:"download_url,omitempty"`
	Bytes                    int64                 `json:"bytes"`
	SHA256                   string                `json:"sha256"`
	SigningCertificateSHA256 string                `json:"signing_certificate_sha256"`
	MinSupportedVersionCode  int64                 `json:"min_supported_version_code,omitempty"`
	Notes                    []string              `json:"notes"`
	NotesI18n                map[string][]string   `json:"notes_i18n"`
	VersionManifest          MobileReleaseManifest `json:"manifest"`
	Status                   string                `json:"status"`
	RolloutPercent           int                   `json:"rollout_percent"`
	CreatedBy                int64                 `json:"created_by"`
	CreatedAt                time.Time             `json:"created_at"`
	UpdatedAt                time.Time             `json:"updated_at"`
	PublishedAt              *time.Time            `json:"published_at,omitempty"`
}

type MobileAppReleaseRepository interface {
	Create(context.Context, MobileAppRelease) (*MobileAppRelease, error)
	List(context.Context, string) ([]MobileAppRelease, error)
	Get(context.Context, int64) (*MobileAppRelease, error)
	HighestVersionCode(context.Context, string) (int64, error)
	Publish(context.Context, int64) (*MobileAppRelease, error)
	SetStatus(context.Context, int64, string) (*MobileAppRelease, error)
	Published(context.Context, string) (*MobileAppRelease, error)
}

type MobileAppReleaseService struct {
	repo    MobileAppReleaseRepository
	storage MobileReleaseStorage
}

func NewMobileAppReleaseService(repo MobileAppReleaseRepository, storage MobileReleaseStorage) *MobileAppReleaseService {
	return &MobileAppReleaseService{repo: repo, storage: storage}
}

func ParseMobileReleaseManifest(data []byte, filename string, artifact []byte) (MobileReleaseManifest, error) {
	var manifest MobileReleaseManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, fmt.Errorf("parse manifest: %w", err)
	}
	manifest.Platform = strings.ToLower(strings.TrimSpace(manifest.Platform))
	manifest.Version = strings.TrimSpace(manifest.Version)
	manifest.PackageName = strings.TrimSpace(manifest.PackageName)
	manifest.ArtifactType = strings.ToLower(strings.TrimSpace(manifest.ArtifactType))
	manifest.Distribution = strings.ToLower(strings.TrimSpace(manifest.Distribution))
	if manifest.Distribution == "" {
		manifest.Distribution = strings.ToLower(strings.TrimSpace(manifest.Channel))
	}
	if manifest.Distribution == "" {
		if strings.EqualFold(filepath.Ext(filename), ".aab") {
			manifest.Distribution = MobileReleaseDistributionPlay
		} else {
			manifest.Distribution = MobileReleaseDistributionDirect
		}
	}
	if manifest.ArtifactType == "" {
		manifest.ArtifactType = strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	}
	if manifest.Platform != "android" || manifest.Version == "" || manifest.VersionCode <= 0 {
		return manifest, errors.New("manifest requires android platform, version, and positive versionCode")
	}
	if manifest.PackageName == "" || manifest.PackageName != "com.jisudeng.chat" {
		return manifest, errors.New("packageName does not match the Android application")
	}
	if manifest.Distribution != MobileReleaseDistributionDirect && manifest.Distribution != MobileReleaseDistributionPlay {
		return manifest, errors.New("distribution must be direct or play")
	}
	if manifest.ArtifactType != MobileReleaseArtifactAPK && manifest.ArtifactType != MobileReleaseArtifactAAB {
		return manifest, errors.New("artifactType must be apk or aab")
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	if ext != manifest.ArtifactType {
		return manifest, errors.New("artifact extension does not match artifactType")
	}
	if (manifest.Distribution == MobileReleaseDistributionDirect && manifest.ArtifactType != MobileReleaseArtifactAPK) ||
		(manifest.Distribution == MobileReleaseDistributionPlay && manifest.ArtifactType != MobileReleaseArtifactAAB) {
		return manifest, errors.New("distribution does not match artifactType")
	}
	if !mobileReleaseHex64.MatchString(strings.TrimSpace(manifest.SHA256)) {
		return manifest, errors.New("sha256 must be a 64-character hexadecimal digest")
	}
	if !mobileReleaseHex64.MatchString(strings.TrimSpace(manifest.SigningCertificateSHA256)) {
		return manifest, errors.New("signingCertificateSha256 must be a 64-character hexadecimal digest")
	}
	manifest.SHA256 = strings.ToLower(strings.TrimSpace(manifest.SHA256))
	manifest.SigningCertificateSHA256 = strings.ToLower(strings.TrimSpace(manifest.SigningCertificateSHA256))
	// Older Dell builds used notesByLocale and zh-CN. Normalize those manifests
	// at the upload boundary so an otherwise valid release is not rejected just
	// because it was produced before the admin API contract was renamed.
	if manifest.NotesI18n == nil {
		manifest.NotesI18n = make(map[string][]string)
	}
	for locale, notes := range manifest.LegacyNotesByLocale {
		canonicalLocale := locale
		if locale == "zh-CN" {
			canonicalLocale = "zh"
		}
		if len(manifest.NotesI18n[canonicalLocale]) == 0 {
			manifest.NotesI18n[canonicalLocale] = notes
		}
	}
	manifest.LegacyNotesByLocale = nil
	hash := sha256.Sum256(artifact)
	actualHash := hex.EncodeToString(hash[:])
	if manifest.Bytes != int64(len(artifact)) {
		return manifest, fmt.Errorf("bytes mismatch: manifest=%d actual=%d", manifest.Bytes, len(artifact))
	}
	if manifest.SHA256 != actualHash {
		return manifest, fmt.Errorf("sha256 mismatch: manifest=%s actual=%s", manifest.SHA256, actualHash)
	}
	if len(manifest.Notes) == 0 {
		manifest.Notes = append([]string(nil), manifest.NotesI18n["zh"]...)
	}
	for _, locale := range []string{"zh", "en", "ja", "ko"} {
		if len(nonEmptyNotes(manifest.NotesI18n[locale])) == 0 {
			return manifest, fmt.Errorf("notes_i18n.%s is required", locale)
		}
		manifest.NotesI18n[locale] = nonEmptyNotes(manifest.NotesI18n[locale])
	}
	return manifest, nil
}

func nonEmptyNotes(notes []string) []string {
	out := make([]string, 0, len(notes))
	for _, note := range notes {
		if note = strings.TrimSpace(note); note != "" {
			out = append(out, note)
		}
	}
	return out
}

func (s *MobileAppReleaseService) Upload(ctx context.Context, actorID int64, manifestJSON []byte, filename string, artifact []byte) (*MobileAppRelease, error) {
	if s == nil || s.repo == nil || s.storage == nil {
		return nil, errors.New("mobile release storage is unavailable")
	}
	manifest, err := ParseMobileReleaseManifest(manifestJSON, filename, artifact)
	if err != nil {
		return nil, err
	}
	highest, err := s.repo.HighestVersionCode(ctx, manifest.Distribution)
	if err != nil {
		return nil, fmt.Errorf("check release version: %w", err)
	}
	if manifest.VersionCode <= highest {
		return nil, fmt.Errorf("versionCode %d must be greater than current %d", manifest.VersionCode, highest)
	}
	key := fmt.Sprintf("mobile-releases/%s/%d-%s.%s", manifest.Distribution, manifest.VersionCode, manifest.SHA256, manifest.ArtifactType)
	contentType := "application/vnd.android.package-archive"
	if manifest.ArtifactType == MobileReleaseArtifactAAB {
		contentType = "application/octet-stream"
	}
	_, err = s.storage.Save(ctx, key, contentType, artifact)
	if err != nil {
		return nil, fmt.Errorf("store release artifact: %w", err)
	}
	release := MobileAppRelease{
		Distribution: manifest.Distribution, PackageName: manifest.PackageName, ArtifactType: manifest.ArtifactType,
		Version: manifest.Version, VersionCode: manifest.VersionCode, StorageKey: key,
		Bytes: manifest.Bytes, SHA256: manifest.SHA256, SigningCertificateSHA256: manifest.SigningCertificateSHA256,
		Notes: manifest.Notes, NotesI18n: manifest.NotesI18n, VersionManifest: manifest,
		Status: MobileReleaseStatusReady, RolloutPercent: 100, CreatedBy: actorID,
	}
	created, err := s.repo.Create(ctx, release)
	if err != nil {
		_ = s.storage.Delete(ctx, key)
		return nil, fmt.Errorf("save release metadata: %w", err)
	}
	created.DownloadURL = mobileReleaseDownloadURL(created.ID)
	return created, nil
}

func (s *MobileAppReleaseService) List(ctx context.Context, distribution string) ([]MobileAppRelease, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("mobile release repository is unavailable")
	}
	items, err := s.repo.List(ctx, strings.ToLower(strings.TrimSpace(distribution)))
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].DownloadURL = mobileReleaseDownloadURL(items[i].ID)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].VersionCode > items[j].VersionCode })
	return items, nil
}

func (s *MobileAppReleaseService) Publish(ctx context.Context, id int64) (*MobileAppRelease, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("mobile release repository is unavailable")
	}
	release, err := s.repo.Publish(ctx, id)
	if err == nil && release != nil {
		release.DownloadURL = mobileReleaseDownloadURL(release.ID)
	}
	return release, err
}

func (s *MobileAppReleaseService) SetStatus(ctx context.Context, id int64, status string) (*MobileAppRelease, error) {
	if status != MobileReleaseStatusPaused && status != MobileReleaseStatusRetired {
		return nil, errors.New("invalid release status")
	}
	if s == nil || s.repo == nil {
		return nil, errors.New("mobile release repository is unavailable")
	}
	release, err := s.repo.SetStatus(ctx, id, status)
	if err == nil && release != nil {
		release.DownloadURL = mobileReleaseDownloadURL(release.ID)
	}
	return release, err
}

func (s *MobileAppReleaseService) PublishedManifest(ctx context.Context, distribution string) (*MobileReleaseManifest, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("mobile release repository is unavailable")
	}
	release, err := s.repo.Published(ctx, distribution)
	if err != nil {
		return nil, err
	}
	manifest := release.VersionManifest
	manifest.APKURL = mobileReleaseDownloadURL(release.ID)
	manifest.Notes = append([]string(nil), release.Notes...)
	manifest.NotesI18n = release.NotesI18n
	return &manifest, nil
}

func mobileReleaseDownloadURL(id int64) string {
	if id <= 0 {
		return ""
	}
	return fmt.Sprintf("/api/v1/mobile/releases/%d/download", id)
}

func (s *MobileAppReleaseService) Open(ctx context.Context, id int64) (MobileAppRelease, io.ReadCloser, string, error) {
	if s == nil || s.repo == nil || s.storage == nil {
		return MobileAppRelease{}, nil, "", errors.New("mobile release storage is unavailable")
	}
	release, err := s.repo.Get(ctx, id)
	if err != nil {
		return MobileAppRelease{}, nil, "", err
	}
	if release.Status != MobileReleaseStatusPublished {
		return MobileAppRelease{}, nil, "", errors.New("release is not published")
	}
	reader, contentType, err := s.storage.Open(ctx, release.StorageKey)
	if err != nil {
		return MobileAppRelease{}, nil, "", err
	}
	return *release, reader, contentType, nil
}
