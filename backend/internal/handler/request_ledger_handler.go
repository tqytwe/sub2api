package handler

import (
	"database/sql"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type RequestLedgerHandler struct{ Ledger *requestledger.Ledger }

func NewRequestLedgerHandler(l *requestledger.Ledger) *RequestLedgerHandler {
	return &RequestLedgerHandler{Ledger: l}
}
func (h *RequestLedgerHandler) ListUser(c *gin.Context)  { h.list(c, false) }
func (h *RequestLedgerHandler) ListAdmin(c *gin.Context) { h.list(c, true) }
func (h *RequestLedgerHandler) GetUser(c *gin.Context)   { h.get(c, false) }
func (h *RequestLedgerHandler) GetAdmin(c *gin.Context)  { h.get(c, true) }

func ledgerViewer(c *gin.Context, admin bool) (requestledger.Viewer, bool) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "Authentication required")
		return requestledger.Viewer{}, false
	}
	if admin && c.GetString(string(middleware.ContextKeyUserRole)) != "admin" {
		response.Forbidden(c, "Administrator access required")
		return requestledger.Viewer{}, false
	}
	return requestledger.Viewer{UserID: subject.UserID, Admin: admin}, true
}

func (h *RequestLedgerHandler) list(c *gin.Context, admin bool) {
	viewer, ok := ledgerViewer(c, admin)
	if !ok {
		return
	}
	f := requestledger.Filter{PrivateID: c.Query("private_id"), ExecutionState: c.Query("execution_state"), UsageState: c.Query("usage_state"), SettlementState: c.Query("settlement_state")}
	for name, target := range map[string]*int{"page": &f.Page, "page_size": &f.PageSize} {
		if raw := c.Query(name); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 1 || value > 10000 {
				response.BadRequest(c, "Invalid pagination")
				return
			}
			*target = value
		}
	}
	for name, target := range map[string]*int64{"user_id": &f.UserID, "api_key_id": &f.KeyID, "account_id": &f.AccountID} {
		if raw := c.Query(name); raw != "" {
			value, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || value < 1 {
				response.BadRequest(c, "Invalid filter")
				return
			}
			*target = value
		}
	}
	if f.PrivateID != "" {
		if _, err := uuid.Parse(f.PrivateID); err != nil {
			response.BadRequest(c, "Invalid private request ID")
			return
		}
	}
	for name, target := range map[string]**time.Time{"start_date": &f.Start, "end_date": &f.End} {
		if raw := c.Query(name); raw != "" {
			v, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				response.BadRequest(c, "Dates must use RFC3339")
				return
			}
			*target = &v
		}
	}
	if f.Start != nil && f.End != nil && !f.Start.Before(*f.End) {
		response.BadRequest(c, "Invalid date range")
		return
	}
	page, err := h.Ledger.List(c.Request.Context(), viewer, f)
	if err != nil {
		response.Error(c, http.StatusServiceUnavailable, "Request ledger is temporarily unavailable")
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, page)
}

func (h *RequestLedgerHandler) get(c *gin.Context, admin bool) {
	viewer, ok := ledgerViewer(c, admin)
	if !ok {
		return
	}
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		response.BadRequest(c, "Invalid private request ID")
		return
	}
	view, section, cursor, size, ok := detailReadOptions(c)
	if !ok {
		return
	}
	if section == "diagnostics" && !viewer.Admin {
		response.Forbidden(c, "Administrator access required")
		return
	}
	c.Header("Cache-Control", "no-store")
	if section != "" {
		var page any
		var err error
		switch section {
		case "attempts":
			page, err = h.Ledger.AttemptsPage(c.Request.Context(), viewer, id, cursor, size)
		case "billing":
			page, err = h.Ledger.BillingPage(c.Request.Context(), viewer, id, cursor, size)
		case "wallet":
			page, err = h.Ledger.WalletPage(c.Request.Context(), viewer, id, cursor, size)
		case "diagnostics":
			page, err = h.Ledger.DiagnosticsPage(c.Request.Context(), viewer, id, cursor, size)
		}
		if err != nil {
			ledgerDetailError(c, err)
			return
		}
		response.Success(c, page)
		return
	}
	record, err := h.Ledger.Get(c.Request.Context(), viewer, id)
	if errors.Is(err, sql.ErrNoRows) {
		response.NotFound(c, "Request not found")
		return
	}
	if err != nil {
		response.Error(c, 503, "Request ledger is temporarily unavailable")
		return
	}
	if view == "summary" {
		response.Success(c, gin.H{"request": record})
		return
	}
	links, err := h.Ledger.BillingReferences(c.Request.Context(), viewer, id)
	if err != nil {
		ledgerDetailError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	wallet, err := h.Ledger.WalletReferences(c.Request.Context(), viewer, id)
	if err != nil {
		ledgerDetailError(c, err)
		return
	}
	attempts, evidenceUnavailable, err := h.Ledger.AttemptsWithEvidence(c.Request.Context(), viewer, id)
	if err != nil {
		ledgerDetailError(c, err)
		return
	}
	result := gin.H{"request": record, "attempts": attempts, "billing": links, "wallet": wallet}
	if evidenceUnavailable {
		result["attempt_evidence_unavailable"] = true
	}
	if viewer.Admin {
		diagnostics, err := h.Ledger.Diagnostics(c.Request.Context(), viewer, id)
		if errors.Is(err, requestledger.ErrDetailPaginationRequired) {
			ledgerDetailError(c, err)
			return
		}
		if err != nil {
			result["diagnostics_unavailable"] = true
		} else {
			result["diagnostics"] = diagnostics
		}
	}
	response.Success(c, result)
}

func detailReadOptions(c *gin.Context) (view, section, cursor string, size int, ok bool) {
	bad := func() (string, string, string, int, bool) {
		response.BadRequest(c, "Invalid detail pagination")
		return "", "", "", 0, false
	}
	q, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil {
		return bad()
	}
	for key, values := range q {
		if len(values) != 1 {
			return bad()
		}
		switch key {
		case "view", "section", "cursor", "page_size", "timezone":
		default:
			return bad()
		}
	}
	// The shared browser HTTP client adds this to every GET. Detail timestamps
	// are absolute; this transport hint does not select or alter a page.
	q.Del("timezone")
	view, section, cursor, size = q.Get("view"), q.Get("section"), q.Get("cursor"), 20
	if len(q) == 0 {
		return "", "", "", size, true
	}
	if view == "summary" && len(q) == 1 {
		return view, "", "", size, true
	}
	if q.Has("view") || !q.Has("section") {
		return bad()
	}
	switch section {
	case "attempts", "billing", "wallet", "diagnostics":
	default:
		return bad()
	}
	if q.Has("page_size") {
		var err error
		size, err = strconv.Atoi(q.Get("page_size"))
		if err != nil || size < 1 || size > requestledger.MaxDetailPageSize {
			return bad()
		}
	}
	if len(cursor) > 2048 {
		return bad()
	}
	return "", section, cursor, size, true
}

func ledgerDetailError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		response.NotFound(c, "Request not found")
	case errors.Is(err, requestledger.ErrDetailCursor):
		response.BadRequest(c, "Invalid detail pagination")
	case errors.Is(err, requestledger.ErrDetailPaginationRequired):
		response.ErrorWithDetails(c, http.StatusConflict, "This request requires the paginated detail API. Refresh or update the client.", "PAGINATION_REQUIRED", map[string]string{"view": "summary", "sections": "attempts,billing,wallet,diagnostics"})
	default:
		response.Error(c, http.StatusServiceUnavailable, "Request ledger is temporarily unavailable")
	}
}

func (h *RequestLedgerHandler) UsageUser(c *gin.Context)  { h.linkedUsage(c, false) }
func (h *RequestLedgerHandler) UsageAdmin(c *gin.Context) { h.linkedUsage(c, true) }
func (h *RequestLedgerHandler) linkedUsage(c *gin.Context, admin bool) {
	viewer, ok := ledgerViewer(c, admin)
	if !ok {
		return
	}
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		response.BadRequest(c, "Invalid private request ID")
		return
	}
	usageID, err := strconv.ParseInt(c.Param("usage_id"), 10, 64)
	if err != nil || usageID <= 0 {
		response.BadRequest(c, "Invalid usage ID")
		return
	}
	usage, err := h.Ledger.Usage(c.Request.Context(), viewer, id, usageID)
	if errors.Is(err, sql.ErrNoRows) {
		response.NotFound(c, "Usage reference not found")
		return
	}
	if err != nil {
		response.Error(c, 503, "Request ledger is temporarily unavailable")
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, usage)
}
