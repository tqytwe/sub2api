package admin

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"sort"
	"strconv"
	"strings"
	"time"
)

func RegisterAffiliateRecordOperationsRoutes(group *gin.RouterGroup, handler *AffiliateHandler) {
	for _, recordKind := range []string{"invites", "rebates", "transfers"} {
		kind := recordKind
		group.GET("/"+kind+"/summary", func(c *gin.Context) { handler.recordReport(c, kind, false) })
		group.GET("/"+kind+"/export", func(c *gin.Context) { handler.recordReport(c, kind, true) })
	}
}

func (h *AffiliateHandler) recordReport(c *gin.Context, kind string, export bool) {
	filter := parseAffiliateRecordFilter(c, 1, 20)
	if !validateAffiliateExactFilter(c) {
		return
	}
	report, err := h.affiliateService.AdminRecordReport(c.Request.Context(), kind, filter, export, currentAdminActorID(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if !export {
		response.Success(c, report.Summary)
		return
	}
	// Resolve the entire bounded result before writing response headers, so
	// database failures and oversized reports return a normal error response.
	columns := map[string]bool{}
	var fields []string
	switch kind {
	case "invites":
		fields = []string{"inviter_id", "inviter_email", "invitee_id", "invitee_email", "aff_code", "total_rebate", "created_at"}
	case "rebates":
		fields = []string{"ledger_id", "order_id", "inviter_id", "invitee_id", "action", "source_type", "currency", "rebate_amount", "status", "frozen_until", "campaign_id", "created_at"}
	case "transfers":
		fields = []string{"ledger_id", "action", "user_id", "user_email", "amount", "external_reference", "paid_at", "note", "actor_id", "created_at"}
	}
	for _, field := range fields {
		columns[field] = true
	}
	for _, row := range report.Rows {
		for key := range row {
			columns[key] = true
		}
	}
	keys := make([]string, 0, len(columns))
	for key := range columns {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	metadata := []string{"export_timezone", "export_generated_at", "export_filters"}
	filters, _ := json.Marshal(map[string]any{"search": filter.Search, "start_at": filter.StartAt, "end_at": filter.EndAt, "user_id": filter.UserID, "inviter_id": filter.InviterID, "invitee_id": filter.InviteeID, "source_order_id": filter.SourceOrderID, "campaign_id": filter.CampaignID, "action": filter.Action, "source_type": filter.SourceType, "currency": filter.Currency, "status": filter.Status})
	var buffer strings.Builder
	_, _ = buffer.WriteString("\ufeff")
	writer := csv.NewWriter(&buffer)
	if err = writer.Write(append(append([]string{}, keys...), metadata...)); err != nil {
		response.InternalError(c, "Failed to create export")
		return
	}
	generated := time.Now().UTC().Format(time.RFC3339)
	if len(report.Rows) == 0 {
		values := make([]string, len(keys))
		values = append(values, filter.Timezone, generated, string(filters))
		if err = writer.Write(values); err != nil {
			response.InternalError(c, "Failed to create export")
			return
		}
	}
	for _, row := range report.Rows {
		values := make([]string, 0, len(keys)+len(metadata))
		for _, key := range keys {
			values = append(values, affiliateCSVCell(row[key]))
		}
		values = append(values, filter.Timezone, generated, string(filters))
		if err = writer.Write(values); err != nil {
			response.InternalError(c, "Failed to create export")
			return
		}
	}
	writer.Flush()
	if writer.Error() != nil {
		response.InternalError(c, "Failed to create export")
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="affiliate-%s-%s.csv"`, kind, time.Now().UTC().Format("20060102-150405")))
	c.Header("Cache-Control", "no-store")
	c.Data(200, "text/csv; charset=utf-8", []byte(buffer.String()))
}

func affiliateCSVCell(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case string:
		// Spreadsheet exports must never execute user-controlled formulas.
		trimmed := strings.TrimLeft(v, " \t\r\n")
		if trimmed != "" && strings.ContainsRune("=+-@", rune(trimmed[0])) {
			return "'" + v
		}
		return v
	default:
		return fmt.Sprint(v)
	}
}

func validateAffiliateExactFilter(c *gin.Context) bool {
	for _, key := range []string{"user_id", "inviter_id", "invitee_id", "source_order_id", "campaign_id"} {
		if raw := c.Query(key); raw != "" {
			value, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || value <= 0 {
				response.BadRequest(c, "Invalid "+key)
				return false
			}
		}
	}
	if action := c.Query("action"); action != "" && action != "accrue" && action != "revoke" && action != "transfer" && action != "withdraw" {
		response.BadRequest(c, "Invalid action")
		return false
	}
	if currency := c.Query("currency"); currency != "" {
		if len(currency) != 3 {
			response.BadRequest(c, "Invalid currency")
			return false
		}
		for _, ch := range currency {
			if ch < 'A' || ch > 'Z' {
				response.BadRequest(c, "Invalid currency")
				return false
			}
		}
	}
	if source := c.Query("source_type"); source != "" && source != "regular" && source != "campaign" && source != "wallet" && source != "other" {
		response.BadRequest(c, "Invalid source_type")
		return false
	}
	if state := c.Query("status"); state != "" && state != "frozen" && state != "available" && state != "revoked" {
		response.BadRequest(c, "Invalid status")
		return false
	}
	for _, key := range []string{"start_at", "end_at"} {
		if raw := c.Query(key); raw != "" && parseAffiliateRecordStartTime(raw, c.Query("timezone")) == nil {
			response.BadRequest(c, "Invalid "+key)
			return false
		}
	}
	filter := parseAffiliateRecordFilter(c, 1, 20)
	if filter.StartAt != nil && filter.EndAt != nil && filter.StartAt.After(*filter.EndAt) {
		response.BadRequest(c, "Invalid date range")
		return false
	}
	return true
}
