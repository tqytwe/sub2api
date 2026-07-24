package service

import (
	"context"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrMobileFeedbackUnavailable = infraerrors.ServiceUnavailable("MOBILE_FEEDBACK_UNAVAILABLE", "mobile feedback storage is unavailable")
	ErrMobileFeedbackInvalid     = infraerrors.BadRequest("MOBILE_FEEDBACK_INVALID", "mobile feedback is invalid")
	ErrMobileFeedbackNotFound    = infraerrors.NotFound("MOBILE_FEEDBACK_NOT_FOUND", "mobile feedback not found")
)

type MobileFeedbackScreenshot struct {
	URL         string `json:"url"`
	FileName    string `json:"file_name,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	ByteSize    int64  `json:"byte_size,omitempty"`
}

type MobileFeedbackRecord struct {
	ID             int64                      `json:"id"`
	UserID         int64                      `json:"user_id"`
	UserEmail      string                     `json:"user_email,omitempty"`
	UserName       string                     `json:"user_name,omitempty"`
	Title          string                     `json:"title"`
	Category       string                     `json:"category"`
	Content        string                     `json:"content"`
	Status         string                     `json:"status"`
	AppVersion     string                     `json:"app_version"`
	Platform       string                     `json:"platform"`
	DeviceModel    string                     `json:"device_model"`
	AndroidVersion string                     `json:"android_version"`
	SystemVersion  string                     `json:"system_version"`
	GroupName      string                     `json:"group_name"`
	GroupID        *int64                     `json:"group_id,omitempty"`
	BackendURL     string                     `json:"backend_url"`
	LastError      string                     `json:"last_error"`
	CrashLog       string                     `json:"crash_log"`
	DeviceInfo     map[string]any             `json:"device_info"`
	Screenshots    []MobileFeedbackScreenshot `json:"screenshots"`
	AdminNote      string                     `json:"admin_note"`
	CreatedAt      time.Time                  `json:"created_at"`
	UpdatedAt      time.Time                  `json:"updated_at"`
}

type MobileFeedbackInput struct {
	Title          string
	Category       string
	Content        string
	AppVersion     string
	Platform       string
	DeviceModel    string
	AndroidVersion string
	SystemVersion  string
	GroupName      string
	GroupID        *int64
	BackendURL     string
	LastError      string
	CrashLog       string
	DeviceInfo     map[string]any
	Screenshots    []MobileFeedbackScreenshot
}

type MobileFeedbackListFilter struct {
	Status   string
	Query    string
	Page     int
	PageSize int
}

type MobileFeedbackList struct {
	Items    []MobileFeedbackRecord `json:"items"`
	Total    int64                  `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
}

type MobileFeedbackUpdate struct {
	Status    string
	AdminNote string
}

func (s *PlayService) CreateMobileFeedback(ctx context.Context, userID int64, input MobileFeedbackInput) (*MobileFeedbackRecord, error) {
	if s == nil || s.repo == nil {
		return nil, ErrMobileFeedbackUnavailable
	}
	record := MobileFeedbackRecord{
		UserID:         userID,
		Title:          truncateMobileFeedbackText(input.Title, 120),
		Category:       normalizeMobileFeedbackCategory(input.Category),
		Content:        truncateMobileFeedbackText(input.Content, 3000),
		Status:         "new",
		AppVersion:     truncateMobileFeedbackText(input.AppVersion, 64),
		Platform:       truncateMobileFeedbackText(defaultString(input.Platform, "android"), 32),
		DeviceModel:    truncateMobileFeedbackText(input.DeviceModel, 120),
		AndroidVersion: truncateMobileFeedbackText(input.AndroidVersion, 64),
		SystemVersion:  truncateMobileFeedbackText(input.SystemVersion, 64),
		GroupName:      truncateMobileFeedbackText(input.GroupName, 120),
		GroupID:        input.GroupID,
		BackendURL:     truncateMobileFeedbackText(input.BackendURL, 300),
		LastError:      truncateMobileFeedbackText(input.LastError, 1000),
		CrashLog:       truncateMobileFeedbackText(input.CrashLog, 4000),
		DeviceInfo:     input.DeviceInfo,
		Screenshots:    normalizeMobileFeedbackScreenshots(input.Screenshots),
	}
	if err := validateMobileFeedbackForCreate(record); err != nil {
		return nil, err
	}
	return s.repo.CreateMobileFeedback(ctx, record)
}

func (s *PlayService) ListAdminMobileFeedback(ctx context.Context, filter MobileFeedbackListFilter) (*MobileFeedbackList, error) {
	if s == nil || s.repo == nil {
		return nil, ErrMobileFeedbackUnavailable
	}
	filter.Status = normalizeOptionalMobileFeedbackStatus(filter.Status)
	filter.Query = truncateMobileFeedbackText(filter.Query, 120)
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	items, total, err := s.repo.ListAdminMobileFeedback(ctx, filter)
	if err != nil {
		return nil, err
	}
	return &MobileFeedbackList{
		Items:    items,
		Total:    total,
		Page:     filter.Page,
		PageSize: filter.PageSize,
	}, nil
}

func (s *PlayService) GetAdminMobileFeedback(ctx context.Context, id int64) (*MobileFeedbackRecord, error) {
	if s == nil || s.repo == nil {
		return nil, ErrMobileFeedbackUnavailable
	}
	if id <= 0 {
		return nil, ErrMobileFeedbackNotFound
	}
	return s.repo.GetAdminMobileFeedback(ctx, id)
}

func (s *PlayService) UpdateAdminMobileFeedback(ctx context.Context, id int64, input MobileFeedbackUpdate) (*MobileFeedbackRecord, error) {
	if s == nil || s.repo == nil {
		return nil, ErrMobileFeedbackUnavailable
	}
	if id <= 0 {
		return nil, ErrMobileFeedbackNotFound
	}
	status := normalizeMobileFeedbackStatus(input.Status)
	if status == "" {
		return nil, ErrMobileFeedbackInvalid.WithMetadata(map[string]string{"field": "status"})
	}
	note := truncateMobileFeedbackText(input.AdminNote, 1000)
	return s.repo.UpdateAdminMobileFeedback(ctx, id, status, note)
}

func validateMobileFeedbackForCreate(record MobileFeedbackRecord) error {
	if record.UserID <= 0 {
		return ErrMobileFeedbackInvalid.WithMetadata(map[string]string{"field": "user_id"})
	}
	if runeLen(record.Title) < 2 {
		return ErrMobileFeedbackInvalid.WithMetadata(map[string]string{"field": "title"})
	}
	if runeLen(record.Content) < 5 {
		return ErrMobileFeedbackInvalid.WithMetadata(map[string]string{"field": "content"})
	}
	return nil
}

func normalizeMobileFeedbackCategory(category string) string {
	value := strings.ToLower(strings.TrimSpace(category))
	switch value {
	case "bug", "experience", "image", "chat", "payment", "account", "request", "other":
		return value
	default:
		return "other"
	}
}

func normalizeOptionalMobileFeedbackStatus(status string) string {
	value := normalizeMobileFeedbackStatus(status)
	if value == "" && strings.TrimSpace(status) == "" {
		return ""
	}
	return value
}

func normalizeMobileFeedbackStatus(status string) string {
	value := strings.ToLower(strings.TrimSpace(status))
	switch value {
	case "new", "viewed", "handled", "deferred", "ignored":
		return value
	default:
		return ""
	}
}

func normalizeMobileFeedbackScreenshots(items []MobileFeedbackScreenshot) []MobileFeedbackScreenshot {
	if len(items) == 0 {
		return []MobileFeedbackScreenshot{}
	}
	limit := len(items)
	if limit > 3 {
		limit = 3
	}
	out := make([]MobileFeedbackScreenshot, 0, limit)
	for _, item := range items[:limit] {
		url := truncateMobileFeedbackText(item.URL, 500)
		if strings.TrimSpace(url) == "" {
			continue
		}
		out = append(out, MobileFeedbackScreenshot{
			URL:         url,
			FileName:    truncateMobileFeedbackText(item.FileName, 160),
			ContentType: truncateMobileFeedbackText(item.ContentType, 80),
			ByteSize:    item.ByteSize,
		})
	}
	return out
}

func truncateMobileFeedbackText(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

func runeLen(value string) int {
	return len([]rune(strings.TrimSpace(value)))
}

func defaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
