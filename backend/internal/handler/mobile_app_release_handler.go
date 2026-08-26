package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/web"
	"github.com/gin-gonic/gin"
)

type MobileAppReleaseHandler struct {
	service *service.MobileAppReleaseService
}

var readMobileReleaseEmbeddedAsset = web.ReadEmbeddedAsset

func NewMobileAppReleaseHandler(releaseService *service.MobileAppReleaseService) *MobileAppReleaseHandler {
	return &MobileAppReleaseHandler{service: releaseService}
}

// CompatibilityManifest preserves the original plain JSON manifest contract
// used by older domestic APKs while allowing published releases to be managed
// from the admin panel. Before the first managed release it serves the embedded
// legacy manifest unchanged.
func (h *MobileAppReleaseHandler) CompatibilityManifest(c *gin.Context) {
	if h != nil && h.service != nil {
		manifest, err := h.service.PublishedManifest(c.Request.Context(), service.MobileReleaseDistributionDirect)
		if err == nil {
			c.Header("Cache-Control", "no-store")
			c.JSON(http.StatusOK, manifest)
			return
		}
		if !errors.Is(err, service.ErrMobileReleaseNotFound) {
			response.ErrorFrom(c, err)
			return
		}
	}
	legacy, err := readMobileReleaseEmbeddedAsset("downloads/android-version.json")
	if err != nil {
		response.NotFound(c, "android release manifest not found")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "application/json; charset=utf-8", legacy)
}

// Check returns the published release metadata. It is deliberately public so
// domestic APK clients can check without a second authenticated session.
func (h *MobileAppReleaseHandler) Check(c *gin.Context) {
	if h == nil || h.service == nil {
		response.InternalError(c, "移动端发布服务不可用")
		return
	}
	distribution := strings.ToLower(strings.TrimSpace(c.DefaultQuery("distribution", service.MobileReleaseDistributionDirect)))
	manifest, err := h.service.PublishedManifest(c.Request.Context(), distribution)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	currentCode, _ := strconv.ParseInt(strings.TrimSpace(c.Query("version_code")), 10, 64)
	response.Success(c, gin.H{"update_available": currentCode <= 0 || manifest.VersionCode > currentCode, "manifest": manifest})
}

func (h *MobileAppReleaseHandler) Download(c *gin.Context) {
	if h == nil || h.service == nil {
		response.InternalError(c, "移动端发布服务不可用")
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "发布 ID 无效")
		return
	}
	release, reader, contentType, err := h.service.Open(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	defer func() { _ = reader.Close() }()
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.Header("Content-Disposition", `attachment; filename="`+release.Version+"."+release.ArtifactType+`"`)
	c.DataFromReader(http.StatusOK, release.Bytes, contentType, reader, nil)
}
