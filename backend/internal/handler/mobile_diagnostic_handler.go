package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const mobileDiagnosticBodyLimit = 32 << 10

type MobileDiagnosticHandler struct {
	db *sql.DB
}

type mobileDiagnosticInput struct {
	InstallationID string         `json:"installation_id"`
	Operation      string         `json:"operation"`
	Category       string         `json:"category"`
	Path           string         `json:"path"`
	StatusCode     int            `json:"status_code"`
	NetworkType    string         `json:"network_type"`
	DurationMS     int64          `json:"duration_ms"`
	RetryCount     int            `json:"retry_count"`
	AppVersion     string         `json:"app_version"`
	OccurredAt     *time.Time     `json:"occurred_at"`
	Metadata       map[string]any `json:"metadata"`
}

func NewMobileDiagnosticHandler(db *sql.DB) *MobileDiagnosticHandler {
	return &MobileDiagnosticHandler{db: db}
}

func (h *MobileDiagnosticHandler) Create(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "请先登录")
		return
	}
	if h == nil || h.db == nil {
		response.Error(c, http.StatusServiceUnavailable, "诊断服务暂时不可用")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, mobileDiagnosticBodyLimit)
	var input mobileDiagnosticInput
	if err := c.ShouldBindJSON(&input); err != nil || !normalizeMobileDiagnostic(&input) {
		response.BadRequest(c, "诊断数据格式不正确")
		return
	}
	metadata, err := json.Marshal(input.Metadata)
	if err != nil {
		response.BadRequest(c, "诊断数据格式不正确")
		return
	}
	occurredAt := time.Now().UTC()
	if input.OccurredAt != nil {
		occurredAt = input.OccurredAt.UTC()
	}
	_, err = h.db.ExecContext(c.Request.Context(), `
		INSERT INTO mobile_diagnostics
			(user_id, installation_id, operation, category, path, status_code, network_type, duration_ms, retry_count, app_version, occurred_at, metadata)
		VALUES ($1, NULLIF($2, '')::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::jsonb)`,
		subject.UserID, input.InstallationID, input.Operation, input.Category, input.Path,
		input.StatusCode, input.NetworkType, input.DurationMS, input.RetryCount,
		input.AppVersion, occurredAt, string(metadata))
	if err != nil {
		response.InternalError(c, "诊断数据暂时无法保存")
		return
	}
	c.Header("Cache-Control", "private, no-store")
	response.Success(c, gin.H{"accepted": true})
}

func normalizeMobileDiagnostic(input *mobileDiagnosticInput) bool {
	input.InstallationID = strings.TrimSpace(input.InstallationID)
	if input.InstallationID != "" {
		if _, err := uuid.Parse(input.InstallationID); err != nil {
			return false
		}
	}
	input.Operation = strings.ToLower(strings.TrimSpace(input.Operation))
	input.Category = strings.ToLower(strings.TrimSpace(input.Category))
	input.Path = strings.TrimSpace(input.Path)
	input.NetworkType = strings.ToLower(strings.TrimSpace(input.NetworkType))
	input.AppVersion = strings.TrimSpace(input.AppVersion)
	if !oneOf(input.Operation, "sync", "chat", "image", "file", "payment", "support", "other") ||
		!oneOf(input.Category, "network", "timeout", "http", "server", "client", "cancelled", "other") ||
		!oneOf(input.NetworkType, "wifi", "cellular", "ethernet", "offline", "unknown") ||
		input.StatusCode < 0 || input.StatusCode > 599 || input.DurationMS < 0 || input.DurationMS > 600000 ||
		input.RetryCount < 0 || input.RetryCount > 20 || len(input.AppVersion) > 64 {
		return false
	}
	parsed, err := url.Parse(input.Path)
	if err != nil || parsed.IsAbs() || parsed.RawQuery != "" || parsed.Fragment != "" || !strings.HasPrefix(parsed.Path, "/") || len(parsed.Path) > 256 {
		return false
	}
	input.Path = parsed.Path
	if input.OccurredAt != nil {
		now := time.Now().UTC()
		when := input.OccurredAt.UTC()
		if when.Before(now.Add(-30*24*time.Hour)) || when.After(now.Add(10*time.Minute)) {
			return false
		}
	}
	allowed := map[string]bool{"native": true, "connection_validated": true, "dns_ok": true}
	clean := make(map[string]any)
	for key, value := range input.Metadata {
		key = strings.ToLower(strings.TrimSpace(key))
		if !allowed[key] || mobileDiagnosticSensitive(key, value) {
			continue
		}
		clean[key] = value
	}
	input.Metadata = clean
	return true
}

func mobileDiagnosticSensitive(key string, value any) bool {
	for _, fragment := range []string{"token", "key", "auth", "cookie", "prompt", "chat", "message", "content", "secret", "password", "url"} {
		if strings.Contains(key, fragment) {
			return true
		}
	}
	text, ok := value.(string)
	return ok && (len(text) > 128 || strings.Contains(strings.ToLower(text), "bearer "))
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
