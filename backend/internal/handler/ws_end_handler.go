package handler

import (
	"database/sql"
	"errors"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *RequestLedgerHandler) WSEndAdmin(c *gin.Context) {
	viewer, ok := ledgerViewer(c, true)
	if !ok {
		return
	}
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		response.BadRequest(c, "Invalid private request ID")
		return
	}
	after := 0
	if raw := c.Query("after_attempt"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 32)
		if err != nil || value < 0 {
			response.BadRequest(c, "Invalid attempt cursor")
			return
		}
		after = int(value)
	}
	c.Header("Cache-Control", "no-store")
	page, err := h.Ledger.WSEndEvidence(c.Request.Context(), viewer, id, after)
	if errors.Is(err, sql.ErrNoRows) {
		response.NotFound(c, "Request not found")
		return
	}
	if err != nil {
		response.Error(c, 503, "WebSocket end evidence is temporarily unavailable")
		return
	}
	response.Success(c, page)
}
