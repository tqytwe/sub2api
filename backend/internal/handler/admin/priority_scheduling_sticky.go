package admin

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type priorityStickyBindingRow struct {
	AccountID int64 `json:"account_id"`
	Count     int   `json:"count"`
}

func priorityStickyGroupID(c *gin.Context) (int64, bool) {
	value, err := strconv.ParseInt(strings.TrimSpace(c.Query("group_id")), 10, 64)
	if err != nil || value <= 0 {
		response.BadRequest(c, "group_id must be a positive integer")
		return 0, false
	}
	return value, true
}

func (h *AccountHandler) PrioritySchedulingStickyBindings(c *gin.Context) {
	groupID, ok := priorityStickyGroupID(c)
	if !ok {
		return
	}
	if h.openAIGatewayService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Sticky session bindings unavailable")
		return
	}
	counts, err := h.openAIGatewayService.StickySessionBindingCounts(c.Request.Context(), groupID)
	if err != nil {
		response.Error(c, http.StatusServiceUnavailable, "Sticky session bindings unavailable")
		return
	}
	rows := make([]priorityStickyBindingRow, 0, len(counts))
	total := 0
	for accountID, count := range counts {
		rows = append(rows, priorityStickyBindingRow{AccountID: accountID, Count: count})
		total += count
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Count != rows[j].Count {
			return rows[i].Count > rows[j].Count
		}
		return rows[i].AccountID < rows[j].AccountID
	})
	response.Success(c, gin.H{"group_id": groupID, "total": total, "by_account": rows})
}

func (h *AccountHandler) ClearPrioritySchedulingStickyBindings(c *gin.Context) {
	var request struct {
		GroupID    int64   `json:"group_id"`
		AccountIDs []int64 `json:"account_ids"`
		DryRun     bool    `json:"dry_run"`
	}
	if c.ShouldBindJSON(&request) != nil || request.GroupID <= 0 {
		response.BadRequest(c, "group_id must be a positive integer")
		return
	}
	for _, id := range request.AccountIDs {
		if id <= 0 {
			response.BadRequest(c, "account_ids must be positive integers")
			return
		}
	}
	if h.openAIGatewayService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Sticky session cleanup unavailable")
		return
	}
	if request.DryRun {
		counts, err := h.openAIGatewayService.StickySessionBindingCounts(c.Request.Context(), request.GroupID)
		if err != nil {
			response.Error(c, http.StatusServiceUnavailable, "Sticky session bindings unavailable")
			return
		}
		total := 0
		for _, count := range counts {
			total += count
		}
		response.Success(c, gin.H{"group_id": request.GroupID, "dry_run": true, "deleted": 0, "remaining": total})
		return
	}
	deleted, err := h.openAIGatewayService.ClearStickySessionBindings(c.Request.Context(), request.GroupID, request.AccountIDs)
	if err != nil {
		response.Error(c, http.StatusServiceUnavailable, "Sticky session cleanup unavailable")
		return
	}
	response.Success(c, gin.H{"group_id": request.GroupID, "deleted": deleted})
}
