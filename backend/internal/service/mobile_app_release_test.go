package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type releaseRepoFake struct {
	highest       int64
	created       *MobileAppRelease
	deletedStatus string
}

func (r *releaseRepoFake) Create(_ context.Context, release MobileAppRelease) (*MobileAppRelease, error) {
	release.ID = 9
	r.created = &release
	return &release, nil
}
func (r *releaseRepoFake) List(context.Context, string) ([]MobileAppRelease, error) { return nil, nil }
func (r *releaseRepoFake) Get(context.Context, int64) (*MobileAppRelease, error) {
	return r.created, nil
}
func (r *releaseRepoFake) HighestVersionCode(context.Context, string) (int64, error) {
	return r.highest, nil
}
func (r *releaseRepoFake) Publish(context.Context, int64) (*MobileAppRelease, error) {
	return r.created, nil
}
func (r *releaseRepoFake) SetStatus(_ context.Context, _ int64, status string) (*MobileAppRelease, error) {
	r.deletedStatus = status
	return r.created, nil
}
func (r *releaseRepoFake) Published(context.Context, string) (*MobileAppRelease, error) {
	return r.created, nil
}

type releaseStorageFake struct{ savedKey, deletedKey string }

func (s *releaseStorageFake) Save(_ context.Context, key, _ string, _ []byte) (string, error) {
	s.savedKey = key
	return "ignored", nil
}
func (s *releaseStorageFake) Open(context.Context, string) (io.ReadCloser, string, error) {
	return io.NopCloser(strings.NewReader("artifact")), "application/octet-stream", nil
}
func (s *releaseStorageFake) Delete(_ context.Context, key string) error {
	s.deletedKey = key
	return nil
}

func releaseManifestJSON(t *testing.T, artifact []byte) []byte {
	t.Helper()
	hash := sha256.Sum256(artifact)
	manifest := map[string]any{
		"platform": "android", "version": "3.0.1", "versionCode": 301,
		"packageName": "com.jisudeng.chat", "artifactType": "apk", "distribution": "direct",
		"bytes": len(artifact), "sha256": hex.EncodeToString(hash[:]),
		"signingCertificateSha256": "cd7abbd79daf6648a429ff34d7450b18cfb6b416e660b2f5169178e0a488627e",
		"notes":                    []string{"旧版兼容说明"},
		"notes_i18n":               map[string][]string{"zh": {"中文说明"}, "en": {"English notes"}, "ja": {"日本語の説明"}, "ko": {"한국어 설명"}},
	}
	data, err := json.Marshal(manifest)
	require.NoError(t, err)
	return data
}

func TestParseMobileReleaseManifestVerifiesArtifactAndLocales(t *testing.T) {
	artifact := []byte("apk-bytes")
	manifest, err := ParseMobileReleaseManifest(releaseManifestJSON(t, artifact), "release.apk", artifact)
	require.NoError(t, err)
	require.Equal(t, MobileReleaseDistributionDirect, manifest.Distribution)
	require.Equal(t, MobileReleaseArtifactAPK, manifest.ArtifactType)
	require.Equal(t, int64(301), manifest.VersionCode)
	require.Equal(t, "English notes", manifest.NotesI18n["en"][0])
}

func TestParseMobileReleaseManifestAcceptsLegacyLocalizedNotes(t *testing.T) {
	artifact := []byte("apk-bytes")
	data := releaseManifestJSON(t, artifact)
	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))
	raw["notes_i18n"] = nil
	raw["notesByLocale"] = map[string][]string{
		"zh-CN": {"中文说明"}, "en": {"English notes"},
		"ja": {"日本語の説明"}, "ko": {"한국어 설명"},
	}
	data, err := json.Marshal(raw)
	require.NoError(t, err)
	manifest, err := ParseMobileReleaseManifest(data, "release.apk", artifact)
	require.NoError(t, err)
	require.Equal(t, []string{"中文说明"}, manifest.NotesI18n["zh"])
	require.Nil(t, manifest.LegacyNotesByLocale)
}

func TestParseMobileReleaseManifestRejectsMissingLocale(t *testing.T) {
	artifact := []byte("apk-bytes")
	data := releaseManifestJSON(t, artifact)
	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))
	raw["notes_i18n"] = map[string][]string{"zh": {"中文"}, "en": {"English"}, "ja": {"日本語"}}
	data, err := json.Marshal(raw)
	require.NoError(t, err)
	_, err = ParseMobileReleaseManifest(data, "release.apk", artifact)
	require.ErrorContains(t, err, "notes_i18n.ko")
}

func TestParseMobileReleaseManifestRejectsHashOrBytesMismatch(t *testing.T) {
	artifact := []byte("apk-bytes")
	data := releaseManifestJSON(t, artifact)
	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))
	raw["bytes"] = len(artifact) + 1
	data, err := json.Marshal(raw)
	require.NoError(t, err)
	_, err = ParseMobileReleaseManifest(data, "release.apk", artifact)
	require.ErrorContains(t, err, "bytes")
}

func TestParseMobileReleaseManifestRejectsChannelArtifactMismatch(t *testing.T) {
	artifact := []byte("aab-bytes")
	data := releaseManifestJSON(t, artifact)
	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))
	raw["artifactType"] = "aab"
	raw["distribution"] = "play"
	data, err := json.Marshal(raw)
	require.NoError(t, err)
	_, err = ParseMobileReleaseManifest(data, "release.apk", artifact)
	require.ErrorContains(t, err, "extension")
}

func TestMobileAppReleaseServiceUploadUsesImmutableKeyAndCanonicalURL(t *testing.T) {
	artifact := []byte("apk-bytes")
	repo := &releaseRepoFake{}
	storage := &releaseStorageFake{}
	svc := NewMobileAppReleaseService(repo, storage)
	release, err := svc.Upload(context.Background(), 42, releaseManifestJSON(t, artifact), "release.apk", artifact)
	require.NoError(t, err)
	require.Equal(t, int64(42), release.CreatedBy)
	require.Equal(t, "/api/v1/mobile/releases/9/download", release.DownloadURL)
	require.Contains(t, storage.savedKey, "mobile-releases/direct/301-")
}
