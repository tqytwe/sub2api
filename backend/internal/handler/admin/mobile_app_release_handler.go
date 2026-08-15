package admin

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const mobileReleaseMaxUploadBytes int64 = 512 << 20

func (h *AdminPlayHandler) ListMobileReleases(c *gin.Context) {
	if h.releaseService == nil {
		response.InternalError(c, "移动端发布服务不可用")
		return
	}
	items, err := h.releaseService.List(c.Request.Context(), c.Query("distribution"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *AdminPlayHandler) UploadMobileRelease(c *gin.Context) {
	if h.releaseService == nil {
		response.InternalError(c, "移动端发布服务不可用")
		return
	}
	// Cap multipart parsing before FormFile allocates buffers. The manifest is
	// tiny, while Dell-produced AABs can legitimately be large.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, mobileReleaseMaxUploadBytes+(2<<20))
	artifact, artifactHeader, err := c.Request.FormFile("artifact")
	if err != nil {
		response.BadRequest(c, "请上传 APK 或 AAB 文件")
		return
	}
	defer func() { _ = artifact.Close() }()
	manifestFile, _, err := c.Request.FormFile("manifest")
	if err != nil {
		response.BadRequest(c, "请同时上传 android-version.json")
		return
	}
	defer func() { _ = manifestFile.Close() }()
	if artifactHeader.Size > mobileReleaseMaxUploadBytes {
		response.BadRequest(c, "发布文件不能超过 512 MB")
		return
	}
	artifactBytes, err := io.ReadAll(io.LimitReader(artifact, mobileReleaseMaxUploadBytes+1))
	if err != nil || int64(len(artifactBytes)) > mobileReleaseMaxUploadBytes {
		response.BadRequest(c, "发布文件无法读取或超过 512 MB")
		return
	}
	manifestBytes, err := io.ReadAll(io.LimitReader(manifestFile, 1<<20))
	if err != nil || len(manifestBytes) == 0 {
		response.BadRequest(c, "android-version.json 无法读取")
		return
	}
	release, err := h.releaseService.Upload(c.Request.Context(), adminActorID(c), manifestBytes, artifactHeader.Filename, artifactBytes)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Created(c, release)
}

func (h *AdminPlayHandler) PublishMobileRelease(c *gin.Context) {
	h.setMobileReleaseStatus(c, "publish")
}
func (h *AdminPlayHandler) PauseMobileRelease(c *gin.Context) {
	h.setMobileReleaseStatus(c, service.MobileReleaseStatusPaused)
}
func (h *AdminPlayHandler) RetireMobileRelease(c *gin.Context) {
	h.setMobileReleaseStatus(c, service.MobileReleaseStatusRetired)
}

func (h *AdminPlayHandler) setMobileReleaseStatus(c *gin.Context, operation string) {
	if h.releaseService == nil {
		response.InternalError(c, "移动端发布服务不可用")
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "发布 ID 无效")
		return
	}
	var release *service.MobileAppRelease
	if operation == "publish" {
		release, err = h.releaseService.Publish(c.Request.Context(), id)
	} else {
		release, err = h.releaseService.SetStatus(c.Request.Context(), id, operation)
	}
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, release)
}
