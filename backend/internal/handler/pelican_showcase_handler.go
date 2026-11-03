package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// PelicanShowcaseHandler serves the user-facing Pelican gallery (read-only) and the
// admin actions on it: the gallery settings and removing a snapshot.
type PelicanShowcaseHandler struct {
	showcase *service.PelicanShowcaseService
	access   pelicanShowcaseGroupAccess
	public   *pelicanPublicCache
}

type pelicanShowcaseGroupAccess interface {
	GetAvailableGroups(context.Context, int64) ([]service.Group, error)
}

func NewPelicanShowcaseHandler(showcase *service.PelicanShowcaseService) *PelicanShowcaseHandler {
	return &PelicanShowcaseHandler{showcase: showcase, public: newPelicanPublicCache(showcase)}
}

// List GET /api/v1/pelican-showcase
// A disabled gallery answers enabled=false with no groups rather than an error.
func (h *PelicanShowcaseHandler) List(c *gin.Context) {
	view, err := h.showcase.View(c.Request.Context(), time.Now())
	if err != nil {
		response.InternalError(c, "Failed to load pelican showcase")
		return
	}
	groups, ok := h.visibleGroups(c)
	if !ok {
		return
	}
	visible := make([]*service.PelicanShowcaseGroup, 0, len(view.Groups))
	for _, group := range view.Groups {
		if !groups[group.ID] {
			continue
		}
		if groupID := c.Query("group_id"); groupID != "" && strconv.FormatInt(group.ID, 10) != groupID {
			continue
		}
		visible = append(visible, group)
	}
	view.Groups = visible
	response.Success(c, view)
}

func (h *PelicanShowcaseHandler) visibleGroups(c *gin.Context) (map[int64]bool, bool) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "user not found in context")
		return nil, false
	}
	if h.access == nil {
		response.Error(c, http.StatusInternalServerError, "group authorization unavailable")
		return nil, false
	}
	groups, err := h.access.GetAvailableGroups(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return nil, false
	}
	allowed := make(map[int64]bool, len(groups))
	for _, group := range groups {
		allowed[group.ID] = true
	}
	return allowed, true
}

// GetItem GET /api/v1/pelican-showcase/items/:id
// Returns the raw model output; the frontend renders it in a sandboxed iframe.
func (h *PelicanShowcaseHandler) GetItem(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid item id")
		return
	}
	item, err := h.showcase.Item(c.Request.Context(), id, time.Now())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	allowed, ok := h.visibleGroups(c)
	if !ok {
		return
	}
	if !allowed[item.GroupID] {
		response.Error(c, http.StatusNotFound, "pelican showcase item not found")
		return
	}
	response.Success(c, item)
}

// DeleteItem DELETE /api/v1/admin/pelican-showcase/items/:id
func (h *PelicanShowcaseHandler) DeleteItem(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid item id")
		return
	}
	if err := h.showcase.Remove(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	h.public.invalidate()
	response.Success(c, gin.H{"deleted": true})
}

// pelicanShowcaseSettings is the gallery switch plus its limits, as edited on the admin page.
type pelicanShowcaseSettings struct {
	Enabled       bool `json:"enabled"`
	APIEnabled    bool `json:"api_enabled"`
	MaxItems      int  `json:"max_items"`
	AutoCleanup   bool `json:"auto_cleanup"`
	RetentionDays int  `json:"retention_days"`
}

func pelicanShowcaseSettingsFrom(runtime service.PelicanShowcaseRuntime) pelicanShowcaseSettings {
	return pelicanShowcaseSettings{
		Enabled:       runtime.Enabled,
		APIEnabled:    runtime.APIEnabled,
		MaxItems:      runtime.Config.MaxItems,
		AutoCleanup:   runtime.Config.AutoCleanup,
		RetentionDays: runtime.Config.RetentionDays,
	}
}

// GetSettings GET /api/v1/admin/pelican-showcase/settings
func (h *PelicanShowcaseHandler) GetSettings(c *gin.Context) {
	runtime, err := h.showcase.Settings(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Failed to load pelican showcase settings")
		return
	}
	response.Success(c, pelicanShowcaseSettingsFrom(runtime))
}

// UpdateSettings PUT /api/v1/admin/pelican-showcase/settings
func (h *PelicanShowcaseHandler) UpdateSettings(c *gin.Context) {
	var req struct {
		pelicanShowcaseSettings
		APIEnabled *bool `json:"api_enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	runtime, err := h.showcase.UpdateSettings(c.Request.Context(), req.Enabled, service.PelicanShowcaseConfig{
		MaxItems:      req.MaxItems,
		AutoCleanup:   req.AutoCleanup,
		RetentionDays: req.RetentionDays,
	}, req.APIEnabled)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	subject, _ := middleware.GetAuthSubjectFromContext(c)
	h.public.invalidate()
	role, _ := middleware.GetUserRoleFromContext(c)
	changed := []string{service.SettingKeyPelicanShowcaseEnabled, service.SettingKeyPelicanShowcaseConfig}
	if req.APIEnabled != nil {
		changed = append(changed, service.SettingKeyPelicanShowcaseAPIEnabled)
	}
	slog.Info("settings updated", "audit", true, "user_id", subject.UserID, "role", role, "changed", changed)
	response.Success(c, pelicanShowcaseSettingsFrom(runtime))
}
