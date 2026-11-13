package admin

import (
	"context"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"strconv"
	"time"
)

func (h *AdminPlayHandler) ListCheckinRecords(c *gin.Context) {
	filter := service.PlayAdminCheckinFilter{Account: c.Query("account"), StartDate: c.Query("start_date"), EndDate: c.Query("end_date"), RewardMode: c.Query("reward_mode"), AsOf: c.Query("as_of")}
	values := []struct {
		key    string
		target *int
	}{{"page", &filter.Page}, {"page_size", &filter.PageSize}}
	for _, v := range values {
		if raw := c.Query(v.key); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 1 {
				response.ErrorFrom(c, infraerrors.BadRequest("PLAY_CHECKIN_QUERY_INVALID", "invalid pagination"))
				return
			}
			*v.target = parsed
		}
	}
	if raw := c.Query("user_id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			response.ErrorFrom(c, infraerrors.BadRequest("PLAY_CHECKIN_QUERY_INVALID", "invalid user id"))
			return
		}
		filter.UserID = id
	}
	result, err := h.playService.ListAdminCheckins(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *AdminPlayHandler) ArenaUserReport(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest("PLAY_ARENA_USER_INVALID", "invalid user id"))
		return
	}
	report, err := h.playService.GetAdminArenaUserReport(c.Request.Context(), id, c.Query("date"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, report)
}

func (h *AdminPlayHandler) ArenaAccountingPreview(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest("PLAY_ARENA_PERIOD_INVALID", "invalid period id"))
		return
	}
	report, err := h.playService.PreviewArenaAccounting(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, report)
}
func (h *AdminPlayHandler) ActivateArenaAccounting(c *gin.Context) {
	var input service.PlayArenaAccountingActivation
	if err := c.ShouldBindJSON(&input); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("PLAY_ARENA_ACTIVATION_INVALID", "invalid activation request"))
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest("PLAY_ARENA_PERIOD_INVALID", "invalid period id"))
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.ErrorFrom(c, infraerrors.Unauthorized("UNAUTHORIZED", "authorization required"))
		return
	}
	input.PeriodID, input.ActorUserID = id, subject.UserID
	middleware.SetAuditAction(c, "admin.play.arena.accounting.activate")
	middleware.SetAuditExtra(c, map[string]any{"period_id": id, "target_version": 2})
	executeAdminIdempotentJSON(c, "admin:play:arena-accounting:"+strconv.FormatInt(id, 10), input, 24*time.Hour, func(ctx context.Context) (any, error) { return h.playService.ActivateArenaAccounting(ctx, input) })
}

func (h *AdminPlayHandler) PreviewCheckinRepair(c *gin.Context) {
	var target service.PlayCheckinRepairTarget
	if err := c.ShouldBindJSON(&target); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("PLAY_CHECKIN_REPAIR_INVALID", "invalid repair target"))
		return
	}
	result, err := h.playService.PreviewAdminCheckinRepair(c.Request.Context(), target)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *AdminPlayHandler) RepairCheckins(c *gin.Context) {
	var input service.PlayCheckinRepairInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("PLAY_CHECKIN_REPAIR_INVALID", "invalid repair request"))
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.ErrorFrom(c, infraerrors.Unauthorized("UNAUTHORIZED", "authorization required"))
		return
	}
	input.ActorUserID = subject.UserID
	middleware.SetAuditAction(c, "admin.play.checkin.repair")
	middleware.SetAuditExtra(c, map[string]any{"record_count": len(input.Items)})
	executeAdminIdempotentJSON(c, "admin:play:checkin-repair", input, 24*time.Hour, func(ctx context.Context) (any, error) { return h.playService.RepairAdminCheckins(ctx, input) })
}

func (h *AdminPlayHandler) ListArenaPeriods(c *gin.Context) {
	kind := c.DefaultQuery("period_type", "monthly")
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, sizeErr := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if err != nil || sizeErr != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("PLAY_ARENA_QUERY_INVALID", "invalid pagination"))
		return
	}
	result, err := h.playService.ListAdminArenaPeriods(c.Request.Context(), kind, page, size)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
