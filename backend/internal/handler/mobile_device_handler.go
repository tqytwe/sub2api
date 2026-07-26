package handler

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type mobileDeviceService interface {
	RegisterDevice(context.Context, int64, service.MobileDeviceRegistration) (*service.MobileDevice, error)
	DeleteDevice(context.Context, int64, string) error
}

type MobileDeviceHandler struct {
	service mobileDeviceService
}

type mobileDeviceRegistrationRequest struct {
	FCMToken   string `json:"fcm_token" binding:"required"`
	Platform   string `json:"platform" binding:"required"`
	AppVersion string `json:"app_version"`
	Locale     string `json:"locale"`
}

func NewMobileDeviceHandler(pushService mobileDeviceService) *MobileDeviceHandler {
	return &MobileDeviceHandler{service: pushService}
}

// Register creates or updates the current user's installation. The response
// contains only a hash-derived token fingerprint, never the FCM token itself.
func (h *MobileDeviceHandler) Register(c *gin.Context) {
	userID, ok := mobileDeviceUserID(c)
	if !ok {
		return
	}
	installationID := strings.TrimSpace(c.Param("installation_id"))
	var request mobileDeviceRegistrationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "设备信息格式不正确")
		return
	}
	if h == nil || h.service == nil {
		response.InternalError(c, "设备推送暂不可用")
		return
	}
	device, err := h.service.RegisterDevice(c.Request.Context(), userID, service.MobileDeviceRegistration{
		InstallationID: installationID,
		Platform:       request.Platform,
		FCMToken:       request.FCMToken,
		AppVersion:     request.AppVersion,
		Locale:         request.Locale,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	response.Success(c, device)
}

// Delete revokes only a device owned by the authenticated user.
func (h *MobileDeviceHandler) Delete(c *gin.Context) {
	userID, ok := mobileDeviceUserID(c)
	if !ok {
		return
	}
	if h == nil || h.service == nil {
		response.InternalError(c, "设备推送暂不可用")
		return
	}
	installationID := strings.TrimSpace(c.Param("installation_id"))
	if err := h.service.DeleteDevice(c.Request.Context(), userID, installationID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	response.Success(c, gin.H{"deleted": true, "installation_id": installationID})
}

func mobileDeviceUserID(c *gin.Context) (int64, bool) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "用户未登录")
		return 0, false
	}
	return subject.UserID, true
}
