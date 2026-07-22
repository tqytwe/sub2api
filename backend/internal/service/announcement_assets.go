package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/google/uuid"
)

const (
	AnnouncementAssetMaxBytes = 2 << 20
	announcementAssetPrefix   = "announcements"
	announcementAssetRoute    = "/api/v1/announcement-assets/"
)

var (
	ErrAnnouncementAssetStorageUnavailable = infraerrors.ServiceUnavailable(
		"ANNOUNCEMENT_ASSET_STORAGE_UNAVAILABLE",
		"announcement image storage is not configured",
	)
	ErrAnnouncementAssetInvalidImage = infraerrors.BadRequest(
		"ANNOUNCEMENT_ASSET_INVALID_IMAGE",
		"announcement image must be PNG, JPEG, WebP, or GIF",
	)
	ErrAnnouncementAssetTooLarge = infraerrors.BadRequest(
		"ANNOUNCEMENT_ASSET_TOO_LARGE",
		"announcement image is too large",
	)
	ErrAnnouncementContentUnsafe = infraerrors.BadRequest(
		"ANNOUNCEMENT_CONTENT_UNSAFE",
		"announcement content contains unsupported HTML, styles, or image links",
	)
)

type AnnouncementAsset struct {
	URL         string `json:"url"`
	Markdown    string `json:"markdown"`
	ContentType string `json:"content_type"`
	ByteSize    int64  `json:"byte_size"`
}

type AnnouncementAssetService struct {
	resolve ImageStorageResolver
}

func NewAnnouncementAssetService(settings *ImageStorageSettingService) *AnnouncementAssetService {
	if settings == nil {
		return &AnnouncementAssetService{}
	}
	return &AnnouncementAssetService{resolve: settings.Resolver()}
}

func NewAnnouncementAssetServiceWithResolver(resolve ImageStorageResolver) *AnnouncementAssetService {
	return &AnnouncementAssetService{resolve: resolve}
}

func (s *AnnouncementAssetService) Upload(ctx context.Context, filename, declaredContentType string, data []byte) (*AnnouncementAsset, error) {
	if len(data) == 0 {
		return nil, ErrAnnouncementAssetInvalidImage
	}
	if len(data) > AnnouncementAssetMaxBytes {
		return nil, ErrAnnouncementAssetTooLarge
	}
	contentType, err := normalizeAnnouncementAssetContentType(declaredContentType, data)
	if err != nil {
		return nil, err
	}
	if s == nil || s.resolve == nil {
		return nil, ErrAnnouncementAssetStorageUnavailable
	}
	uploader, enabled := s.resolve()
	if !enabled || uploader == nil {
		return nil, ErrAnnouncementAssetStorageUnavailable
	}

	assetID := uuid.NewString()
	keyBase := path.Join(announcementAssetPrefix, assetID)
	if _, err := uploader.SaveImageBytes(ctx, keyBase, contentType, data); err != nil {
		return nil, fmt.Errorf("store announcement image: %w", err)
	}
	publicURL := announcementAssetRoute + escapeAnnouncementAssetKey(keyBase+extensionForContentType(contentType))
	alt := sanitizeAnnouncementAltText(filename)
	if alt == "" {
		alt = "announcement image"
	}
	return &AnnouncementAsset{
		URL:         publicURL,
		Markdown:    fmt.Sprintf("![%s](%s)", alt, publicURL),
		ContentType: contentType,
		ByteSize:    int64(len(data)),
	}, nil
}

func (s *AnnouncementAssetService) Open(ctx context.Context, rawKey string) (io.ReadCloser, string, error) {
	key, err := normalizeAnnouncementAssetKey(rawKey)
	if err != nil {
		return nil, "", err
	}
	if s == nil || s.resolve == nil {
		return nil, "", ErrAnnouncementAssetStorageUnavailable
	}
	uploader, enabled := s.resolve()
	if !enabled || uploader == nil || uploader.storage == nil {
		return nil, "", ErrAnnouncementAssetStorageUnavailable
	}
	reader, ok := uploader.storage.(ImageAssetReader)
	if !ok || reader == nil {
		return nil, "", ErrAnnouncementAssetStorageUnavailable
	}
	storageKey, err := uploader.StorageKey(key)
	if err != nil {
		return nil, "", err
	}
	return reader.Open(ctx, storageKey)
}

func ValidateAnnouncementMarkdownContent(content string) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return ErrAnnouncementContentRequired
	}
	if announcementRawHTMLPattern.MatchString(content) {
		return ErrAnnouncementContentUnsafe
	}
	for _, match := range announcementMarkdownImagePattern.FindAllStringSubmatch(content, -1) {
		if len(match) < 2 || !IsAnnouncementAssetURLAllowed(match[1]) {
			return ErrAnnouncementContentUnsafe
		}
	}
	return nil
}

func IsAnnouncementAssetURLAllowed(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "data:") || strings.HasPrefix(raw, "//") {
		return false
	}
	if strings.HasPrefix(raw, announcementAssetRoute) {
		_, err := normalizeAnnouncementAssetKey(strings.TrimPrefix(raw, announcementAssetRoute))
		return err == nil
	}
	return false
}

func normalizeAnnouncementAssetContentType(declared string, data []byte) (string, error) {
	declared = strings.ToLower(strings.TrimSpace(strings.Split(declared, ";")[0]))
	detected := strings.ToLower(strings.TrimSpace(strings.Split(http.DetectContentType(data), ";")[0]))
	switch detected {
	case "image/png", "image/jpeg", "image/gif":
		return detected, nil
	}
	if isWebPBytes(data) && (declared == "" || declared == "image/webp") {
		return "image/webp", nil
	}
	return "", ErrAnnouncementAssetInvalidImage
}

func isWebPBytes(data []byte) bool {
	return len(data) >= 12 &&
		string(data[0:4]) == "RIFF" &&
		string(data[8:12]) == "WEBP"
}

func normalizeAnnouncementAssetKey(raw string) (string, error) {
	raw, err := url.PathUnescape(strings.TrimLeft(strings.TrimSpace(raw), "/"))
	if err != nil {
		return "", err
	}
	key, err := cleanImageStorageKeyBase(raw)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(key, announcementAssetPrefix+"/") {
		return "", fmt.Errorf("invalid announcement image key")
	}
	return key, nil
}

func escapeAnnouncementAssetKey(key string) string {
	parts := strings.Split(key, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func sanitizeAnnouncementAltText(filename string) string {
	filename = strings.TrimSpace(filename)
	filename = strings.TrimSuffix(filename, path.Ext(filename))
	filename = strings.NewReplacer("[", "", "]", "", "(", "", ")", "", "\n", " ", "\r", " ").Replace(filename)
	if len([]rune(filename)) > 80 {
		return string([]rune(filename)[:80])
	}
	return filename
}
