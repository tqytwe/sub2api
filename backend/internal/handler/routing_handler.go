package handler

import (
	"database/sql"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RoutingAdmin exposes only the private request UUID under the existing admin
// authorization. It is intentionally independent of the request detail read.
func (h *RequestLedgerHandler) RoutingAdmin(c *gin.Context) {
	viewer, ok := ledgerViewer(c, true)
	if !ok {
		return
	}
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		response.BadRequest(c, "Invalid private request ID")
		return
	}
	c.Header("Cache-Control", "no-store")
	evidence, err := h.Ledger.RoutingEvidence(c.Request.Context(), viewer, id)
	if errors.Is(err, sql.ErrNoRows) {
		response.NotFound(c, "Request not found")
		return
	}
	if err != nil {
		response.Error(c, 503, "Routing evidence is temporarily unavailable")
		return
	}
	response.Success(c, evidence)
}
