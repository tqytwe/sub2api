package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/util/logredact"
)

var (
	ErrMobileFeedbackUnavailable = infraerrors.ServiceUnavailable("MOBILE_FEEDBACK_UNAVAILABLE", "客服工单服务暂时不可用")
	ErrMobileFeedbackInvalid     = infraerrors.BadRequest("MOBILE_FEEDBACK_INVALID", "工单信息不正确")
	ErrMobileFeedbackNotFound    = infraerrors.NotFound("MOBILE_FEEDBACK_NOT_FOUND", "工单不存在")
	ErrMobileFeedbackClosed      = infraerrors.Conflict("MOBILE_FEEDBACK_CLOSED", "工单已关闭，无法继续回复")
	ErrMobileFeedbackSensitive   = infraerrors.BadRequest("MOBILE_FEEDBACK_SENSITIVE_DIAGNOSTICS", "诊断信息包含不允许提交的敏感字段")
)

const (
	MobileSupportStatusOpen       = "open"
	MobileSupportStatusInProgress = "in_progress"
	MobileSupportStatusWaiting    = "waiting_user"
	MobileSupportStatusResolved   = "resolved"
	MobileSupportStatusClosed     = "closed"
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
	WorkItems      []MobileFeedbackWorkItem   `json:"work_items,omitempty"`
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
	AdminNote *string
	WorkItem  *MobileFeedbackWorkItemUpdate
}

type MobileFeedbackWorkItem struct {
	ID                 int64     `json:"id"`
	FeedbackID         int64     `json:"feedback_id"`
	Type               string    `json:"type"`
	Priority           string    `json:"priority"`
	Status             string    `json:"status"`
	Title              string    `json:"title"`
	Summary            string    `json:"summary"`
	AcceptanceCriteria string    `json:"acceptance_criteria"`
	TargetVersion      string    `json:"target_version"`
	ReleasedVersion    string    `json:"released_version"`
	Owner              string    `json:"owner"`
	Source             string    `json:"source"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type MobileFeedbackWorkItemUpdate struct {
	ID              int64
	Status          string
	Priority        string
	TargetVersion   string
	ReleasedVersion string
	Owner           string
}

type MobileFeedbackMessage struct {
	ID         int64     `json:"id"`
	FeedbackID int64     `json:"feedback_id"`
	SenderType string    `json:"sender_type"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}

type MobileSupportTicket struct {
	ID          int64                      `json:"id"`
	Title       string                     `json:"title"`
	Category    string                     `json:"category"`
	Content     string                     `json:"content"`
	Status      string                     `json:"status"`
	AppVersion  string                     `json:"app_version,omitempty"`
	Platform    string                     `json:"platform,omitempty"`
	DeviceModel string                     `json:"device_model,omitempty"`
	Screenshots []MobileFeedbackScreenshot `json:"screenshots"`
	Messages    []MobileFeedbackMessage    `json:"messages,omitempty"`
	CreatedAt   time.Time                  `json:"created_at"`
	UpdatedAt   time.Time                  `json:"updated_at"`
}

type MobileSupportTicketList struct {
	Items    []MobileSupportTicket `json:"items"`
	Total    int64                 `json:"total"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"page_size"`
}

func (s *PlayService) CreateMobileFeedback(ctx context.Context, userID int64, input MobileFeedbackInput) (*MobileFeedbackRecord, error) {
	if s == nil || s.repo == nil {
		return nil, ErrMobileFeedbackUnavailable
	}
	deviceInfo, err := normalizeMobileFeedbackDeviceInfo(input.DeviceInfo)
	if err != nil {
		return nil, err
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
		BackendURL:     mobileFeedbackBackendOrigin(input.BackendURL),
		LastError:      mobileFeedbackSafeError(input.LastError),
		CrashLog:       "",
		DeviceInfo:     deviceInfo,
		Screenshots:    normalizeMobileFeedbackScreenshots(input.Screenshots),
	}
	if err := validateMobileFeedbackForCreate(record); err != nil {
		return nil, err
	}
	created, err := s.repo.CreateMobileFeedback(ctx, record)
	if err != nil {
		return nil, err
	}
	if workRepo, ok := s.repo.(mobileFeedbackWorkItemRepository); ok && created != nil {
		if item, itemErr := workRepo.EnsureMobileFeedbackWorkItem(ctx, created.ID, defaultMobileFeedbackWorkItem(*created)); itemErr == nil && item != nil {
			created.WorkItems = []MobileFeedbackWorkItem{*item}
		}
	}
	return created, nil
}

func (s *PlayService) ListUserMobileFeedback(ctx context.Context, userID int64, filter MobileFeedbackListFilter) (*MobileSupportTicketList, error) {
	if s == nil || s.repo == nil {
		return nil, ErrMobileFeedbackUnavailable
	}
	if userID <= 0 {
		return nil, ErrMobileFeedbackNotFound
	}
	storageStatus, ok := mobileSupportStorageStatus(filter.Status)
	if !ok {
		return nil, ErrMobileFeedbackInvalid.WithMetadata(map[string]string{"field": "status"})
	}
	filter.Status = storageStatus
	filter.Query = ""
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 50 {
		filter.PageSize = 50
	}
	items, total, err := s.repo.ListUserMobileFeedback(ctx, userID, filter)
	if err != nil {
		return nil, mobileFeedbackUserFacingError(err)
	}
	out := make([]MobileSupportTicket, 0, len(items))
	for i := range items {
		out = append(out, mobileSupportTicketFromRecord(items[i], nil))
	}
	return &MobileSupportTicketList{Items: out, Total: total, Page: filter.Page, PageSize: filter.PageSize}, nil
}

func (s *PlayService) GetUserMobileFeedback(ctx context.Context, userID, id int64) (*MobileSupportTicket, error) {
	if s == nil || s.repo == nil {
		return nil, ErrMobileFeedbackUnavailable
	}
	if userID <= 0 || id <= 0 {
		return nil, ErrMobileFeedbackNotFound
	}
	record, err := s.repo.GetUserMobileFeedback(ctx, userID, id)
	if err != nil {
		return nil, mobileFeedbackUserFacingError(err)
	}
	messages, err := s.repo.ListMobileFeedbackMessages(ctx, userID, id)
	if err != nil {
		return nil, mobileFeedbackUserFacingError(err)
	}
	hasSupportMessage := false
	for _, message := range messages {
		if message.SenderType == "support" {
			hasSupportMessage = true
			break
		}
	}
	if note := strings.TrimSpace(record.AdminNote); note != "" && !hasSupportMessage {
		messages = append(messages, MobileFeedbackMessage{
			FeedbackID: record.ID,
			SenderType: "support",
			Content:    truncateMobileFeedbackText(note, 2000),
			CreatedAt:  record.UpdatedAt,
		})
	}
	sort.SliceStable(messages, func(i, j int) bool { return messages[i].CreatedAt.Before(messages[j].CreatedAt) })
	ticket := mobileSupportTicketFromRecord(*record, messages)
	return &ticket, nil
}

func mobileFeedbackBackendOrigin(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" || parsed.User != nil {
		return ""
	}
	return truncateMobileFeedbackText(parsed.Scheme+"://"+parsed.Host, 300)
}

func mobileFeedbackSafeError(raw string) string {
	lower := strings.ToLower(logredact.RedactText(raw,
		"authorization", "token", "api_key", "apikey", "cookie", "prompt", "chat", "conversation", "messages", "content", "url", "secret"))
	parts := make([]string, 0, 3)
	if match := mobileFeedbackHTTPStatusPattern.FindStringSubmatch(lower); len(match) == 2 {
		parts = append(parts, "HTTP "+match[1])
	}
	for _, category := range mobileFeedbackErrorCategories {
		if slices.ContainsFunc(category.keywords, func(keyword string) bool { return strings.Contains(lower, keyword) }) {
			parts = append(parts, category.label)
			break
		}
	}
	if len(parts) == 0 && strings.TrimSpace(raw) != "" {
		parts = append(parts, "网络请求失败")
	}
	return strings.Join(parts, "；")
}

var mobileFeedbackHTTPStatusPattern = regexp.MustCompile(`(?i)(?:http(?:\/\d(?:\.\d)?)?[^0-9]*)?\b([1-5][0-9]{2})\b`)

var mobileFeedbackErrorCategories = []struct {
	label    string
	keywords []string
}{
	{label: "请求超时", keywords: []string{"timeout", "timed out", "超时"}},
	{label: "域名解析失败", keywords: []string{"dns", "resolve host", "name not resolved"}},
	{label: "安全连接失败", keywords: []string{"tls", "ssl", "certificate"}},
	{label: "网络连接失败", keywords: []string{"failed to fetch", "network", "connection", "连接失败"}},
	{label: "请求已取消", keywords: []string{"abort", "cancel", "取消"}},
	{label: "认证失败", keywords: []string{"unauthorized", "forbidden", "认证失败"}},
}

func (s *PlayService) AddUserMobileFeedbackMessage(ctx context.Context, userID, id int64, content string) (*MobileSupportTicket, error) {
	if s == nil || s.repo == nil {
		return nil, ErrMobileFeedbackUnavailable
	}
	content = truncateMobileFeedbackText(content, 2000)
	if userID <= 0 || id <= 0 || runeLen(content) < 1 {
		return nil, ErrMobileFeedbackInvalid.WithMetadata(map[string]string{"field": "content"})
	}
	record, err := s.repo.GetUserMobileFeedback(ctx, userID, id)
	if err != nil {
		return nil, mobileFeedbackUserFacingError(err)
	}
	if mobileSupportPublicStatus(record.Status) == MobileSupportStatusClosed {
		return nil, ErrMobileFeedbackClosed
	}
	if _, err := s.repo.CreateUserMobileFeedbackMessage(ctx, userID, id, content); err != nil {
		return nil, mobileFeedbackUserFacingError(err)
	}
	return s.GetUserMobileFeedback(ctx, userID, id)
}

func (s *PlayService) CloseUserMobileFeedback(ctx context.Context, userID, id int64) (*MobileSupportTicket, error) {
	if s == nil || s.repo == nil {
		return nil, ErrMobileFeedbackUnavailable
	}
	if userID <= 0 || id <= 0 {
		return nil, ErrMobileFeedbackNotFound
	}
	if _, err := s.repo.CloseUserMobileFeedback(ctx, userID, id); err != nil {
		return nil, mobileFeedbackUserFacingError(err)
	}
	return s.GetUserMobileFeedback(ctx, userID, id)
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
	record, err := s.repo.GetAdminMobileFeedback(ctx, id)
	if err != nil {
		return nil, err
	}
	s.attachMobileFeedbackWorkItems(ctx, record)
	return record, nil
}

func (s *PlayService) UpdateAdminMobileFeedback(ctx context.Context, id int64, input MobileFeedbackUpdate) (*MobileFeedbackRecord, error) {
	if s == nil || s.repo == nil {
		return nil, ErrMobileFeedbackUnavailable
	}
	if id <= 0 {
		return nil, ErrMobileFeedbackNotFound
	}
	var current *MobileFeedbackRecord
	status := normalizeOptionalMobileFeedbackStatus(input.Status)
	if status == "" && strings.TrimSpace(input.Status) != "" {
		return nil, ErrMobileFeedbackInvalid.WithMetadata(map[string]string{"field": "status"})
	}
	if status == "" {
		var err error
		current, err = s.repo.GetAdminMobileFeedback(ctx, id)
		if err != nil {
			return nil, err
		}
		status = current.Status
	}
	note := ""
	if input.AdminNote != nil {
		note = truncateMobileFeedbackText(*input.AdminNote, 1000)
	} else {
		if current == nil {
			var err error
			current, err = s.repo.GetAdminMobileFeedback(ctx, id)
			if err != nil {
				return nil, err
			}
		}
		note = current.AdminNote
	}
	record, err := s.repo.UpdateAdminMobileFeedback(ctx, id, status, note)
	if err != nil {
		return nil, err
	}
	if input.WorkItem != nil {
		normalizedWorkItem, err := normalizeMobileFeedbackWorkItemUpdate(*input.WorkItem)
		if err != nil {
			return nil, err
		}
		if workRepo, ok := s.repo.(mobileFeedbackWorkItemRepository); ok {
			if _, workErr := workRepo.UpdateMobileFeedbackWorkItem(ctx, id, normalizedWorkItem); workErr != nil {
				return nil, workErr
			}
		}
	}
	s.attachMobileFeedbackWorkItems(ctx, record)
	if input.AdminNote != nil && note != "" && record != nil && record.UserID > 0 && s.mobilePush != nil {
		_, _, _ = s.mobilePush.Enqueue(ctx, MobilePushEvent{
			UserID: record.UserID, IdempotencyKey: fmt.Sprintf("support-reply:%d:%d", record.ID, record.UpdatedAt.UnixNano()),
			EventType: "support.reply", SourceType: "mobile_feedback", SourceID: strconv.FormatInt(record.ID, 10),
			TitleZh: "客服回复了你的工单", BodyZh: "你的问题有了新进展，打开极速蹬查看详情",
			Data: map[string]string{"ticket_id": strconv.FormatInt(record.ID, 10)},
		})
	}
	return record, nil
}

func normalizeMobileFeedbackWorkItemUpdate(input MobileFeedbackWorkItemUpdate) (MobileFeedbackWorkItemUpdate, error) {
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	input.Priority = strings.ToLower(strings.TrimSpace(input.Priority))
	input.TargetVersion = truncateMobileFeedbackText(input.TargetVersion, 64)
	input.ReleasedVersion = truncateMobileFeedbackText(input.ReleasedVersion, 64)
	input.Owner = truncateMobileFeedbackText(input.Owner, 120)
	if input.Status != "" && !isValidMobileFeedbackWorkItemStatus(input.Status) {
		return input, ErrMobileFeedbackInvalid.WithMetadata(map[string]string{"field": "work_item.status"})
	}
	if input.Priority != "" && !isValidMobileFeedbackWorkItemPriority(input.Priority) {
		return input, ErrMobileFeedbackInvalid.WithMetadata(map[string]string{"field": "work_item.priority"})
	}
	return input, nil
}

func isValidMobileFeedbackWorkItemStatus(status string) bool {
	switch status {
	case "backlog", "accepted", "in_progress", "testing", "released", "rejected":
		return true
	default:
		return false
	}
}

func isValidMobileFeedbackWorkItemPriority(priority string) bool {
	switch priority {
	case "p0", "p1", "p2", "p3":
		return true
	default:
		return false
	}
}

type mobileFeedbackWorkItemRepository interface {
	EnsureMobileFeedbackWorkItem(ctx context.Context, feedbackID int64, item MobileFeedbackWorkItem) (*MobileFeedbackWorkItem, error)
	ListMobileFeedbackWorkItems(ctx context.Context, feedbackID int64) ([]MobileFeedbackWorkItem, error)
	UpdateMobileFeedbackWorkItem(ctx context.Context, feedbackID int64, input MobileFeedbackWorkItemUpdate) (*MobileFeedbackWorkItem, error)
}

func (s *PlayService) attachMobileFeedbackWorkItems(ctx context.Context, record *MobileFeedbackRecord) {
	if record == nil || record.ID <= 0 {
		return
	}
	workRepo, ok := s.repo.(mobileFeedbackWorkItemRepository)
	if !ok {
		return
	}
	items, err := workRepo.ListMobileFeedbackWorkItems(ctx, record.ID)
	if err == nil {
		record.WorkItems = items
	}
}

func defaultMobileFeedbackWorkItem(record MobileFeedbackRecord) MobileFeedbackWorkItem {
	itemType := mobileFeedbackWorkItemType(record)
	return MobileFeedbackWorkItem{
		FeedbackID:         record.ID,
		Type:               itemType,
		Priority:           mobileFeedbackWorkItemPriority(record),
		Status:             "backlog",
		Title:              truncateMobileFeedbackText(record.Title, 120),
		Summary:            truncateMobileFeedbackText(mobileFeedbackWorkItemSummary(record), 1000),
		AcceptanceCriteria: truncateMobileFeedbackText(mobileFeedbackWorkItemAcceptance(record, itemType), 1000),
		Source:             "auto",
	}
}

func mobileFeedbackWorkItemType(record MobileFeedbackRecord) string {
	switch normalizeMobileFeedbackCategory(record.Category) {
	case "payment":
		return "payment"
	case "account":
		return "account"
	case "image":
		return "image"
	case "chat":
		return "chat"
	case "experience":
		return "ux"
	case "request":
		return "feature"
	case "bug":
		if strings.Contains(strings.ToLower(record.LastError), "网络") || strings.Contains(strings.ToLower(record.LastError), "timeout") {
			return "network"
		}
		return "bug"
	default:
		return "other"
	}
}

func mobileFeedbackWorkItemPriority(record MobileFeedbackRecord) string {
	text := strings.ToLower(strings.Join([]string{record.Category, record.Title, record.Content, record.LastError}, " "))
	switch {
	case strings.Contains(text, "支付") || strings.Contains(text, "payment") || strings.Contains(text, "登录") || strings.Contains(text, "login") || strings.Contains(text, "failed to fetch"):
		return "p1"
	case strings.Contains(text, "崩溃") || strings.Contains(text, "crash") || strings.Contains(text, "无法使用"):
		return "p1"
	case normalizeMobileFeedbackCategory(record.Category) == "request":
		return "p3"
	default:
		return "p2"
	}
}

func mobileFeedbackWorkItemSummary(record MobileFeedbackRecord) string {
	parts := []string{
		"来源：APP反馈",
		"类型：" + normalizeMobileFeedbackCategory(record.Category),
		"标题：" + strings.TrimSpace(record.Title),
	}
	if record.LastError != "" {
		parts = append(parts, "错误摘要："+record.LastError)
	}
	if record.AppVersion != "" {
		parts = append(parts, "版本："+record.AppVersion)
	}
	if record.DeviceModel != "" {
		parts = append(parts, "设备："+record.DeviceModel)
	}
	parts = append(parts, "内容："+strings.TrimSpace(record.Content))
	return strings.Join(parts, "\n")
}

func mobileFeedbackWorkItemAcceptance(record MobileFeedbackRecord, itemType string) string {
	switch itemType {
	case "payment":
		return "用户能在APP内成功发起支付；返回APP后可自动刷新订单状态；失败时展示明确中文原因。"
	case "account":
		return "用户登录后不会在弱网或切换网络时无故中断；认证恢复失败时给出明确处理方式。"
	case "network":
		return "弱网、Wi-Fi、移动网络切换场景下请求可自动重试或提示可恢复状态；诊断信息可定位失败路径。"
	case "image":
		return "生图相关操作成功、失败、取消、重试状态一致；历史记录可管理。"
	case "chat":
		return "聊天请求能及时发送并返回状态；失败时可重试且不丢失用户输入。"
	case "feature":
		return "需求被产品确认后给出范围、入口、消耗和验收标准。"
	default:
		return "复现用户反馈场景；确认问题原因；修复后在对应APP版本验证并记录处理结果。"
	}
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
	case MobileSupportStatusOpen:
		return "new"
	case MobileSupportStatusInProgress:
		return "viewed"
	case MobileSupportStatusWaiting:
		return "deferred"
	case MobileSupportStatusResolved:
		return "handled"
	case MobileSupportStatusClosed:
		return "ignored"
	default:
		return ""
	}
}

func mobileSupportStorageStatus(status string) (string, bool) {
	if strings.TrimSpace(status) == "" {
		return "", true
	}
	value := normalizeMobileFeedbackStatus(status)
	return value, value != ""
}

func mobileFeedbackUserFacingError(err error) error {
	if err == nil {
		return nil
	}
	for _, known := range []error{ErrMobileFeedbackNotFound, ErrMobileFeedbackInvalid, ErrMobileFeedbackClosed, ErrMobileFeedbackSensitive} {
		if errors.Is(err, known) {
			return err
		}
	}
	return ErrMobileFeedbackUnavailable.WithCause(err)
}

func mobileSupportPublicStatus(status string) string {
	switch normalizeMobileFeedbackStatus(status) {
	case "new":
		return MobileSupportStatusOpen
	case "viewed":
		return MobileSupportStatusInProgress
	case "deferred":
		return MobileSupportStatusWaiting
	case "handled":
		return MobileSupportStatusResolved
	case "ignored":
		return MobileSupportStatusClosed
	default:
		return MobileSupportStatusOpen
	}
}

func mobileSupportTicketFromRecord(record MobileFeedbackRecord, messages []MobileFeedbackMessage) MobileSupportTicket {
	return MobileSupportTicket{
		ID:          record.ID,
		Title:       record.Title,
		Category:    record.Category,
		Content:     record.Content,
		Status:      mobileSupportPublicStatus(record.Status),
		AppVersion:  record.AppVersion,
		Platform:    record.Platform,
		DeviceModel: record.DeviceModel,
		Screenshots: append([]MobileFeedbackScreenshot(nil), record.Screenshots...),
		Messages:    append([]MobileFeedbackMessage(nil), messages...),
		CreatedAt:   record.CreatedAt,
		UpdatedAt:   record.UpdatedAt,
	}
}

var mobileFeedbackDeviceInfoAllowlist = map[string]struct{}{
	"platform": {}, "manufacturer": {}, "brand": {}, "model": {}, "device": {}, "product": {},
	"androidversion": {}, "sdkint": {}, "appversionname": {}, "appversioncode": {},
	"useragent": {}, "language": {}, "screen": {}, "networktype": {}, "connectiontype": {},
	"httpstatus": {}, "errorcode": {}, "timeout": {}, "transport": {},
}

func normalizeMobileFeedbackDeviceInfo(input map[string]any) (map[string]any, error) {
	if err := validateMobileFeedbackDiagnosticKeys(input); err != nil {
		return nil, err
	}
	out := make(map[string]any)
	for key, value := range input {
		normalizedKey := normalizeMobileFeedbackDiagnosticKey(key)
		if _, ok := mobileFeedbackDeviceInfoAllowlist[normalizedKey]; !ok {
			continue
		}
		switch value.(type) {
		case string, bool, float64, float32, int, int32, int64, uint, uint32, uint64, nil:
			out[key] = value
		}
	}
	return out, nil
}

func validateMobileFeedbackDiagnosticKeys(value any) error {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if isSensitiveMobileFeedbackDiagnosticKey(normalizeMobileFeedbackDiagnosticKey(key)) {
				return ErrMobileFeedbackSensitive.WithMetadata(map[string]string{"field": key})
			}
			if err := validateMobileFeedbackDiagnosticKeys(child); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range typed {
			if err := validateMobileFeedbackDiagnosticKeys(child); err != nil {
				return err
			}
		}
	}
	return nil
}

func normalizeMobileFeedbackDiagnosticKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	replacer := strings.NewReplacer("_", "", "-", "", ".", "", " ", "")
	return replacer.Replace(key)
}

func isSensitiveMobileFeedbackDiagnosticKey(key string) bool {
	for _, forbidden := range []string{"token", "apikey", "authorization", "credential", "secret", "password", "prompt", "chat", "conversation", "messages"} {
		if strings.Contains(key, forbidden) {
			return true
		}
	}
	return false
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
