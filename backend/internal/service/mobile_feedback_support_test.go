//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type mobileFeedbackSupportRepoStub struct {
	PlayRepository
	records  map[int64]MobileFeedbackRecord
	messages map[int64][]MobileFeedbackMessage
	created  *MobileFeedbackRecord
}

type mobileFeedbackAdminRepoStub struct {
	PlayRepository
	record       MobileFeedbackRecord
	workItemErr  error
	statusUpdate MobileFeedbackStatusUpdate
}

func (r *mobileFeedbackAdminRepoStub) GetAdminMobileFeedback(_ context.Context, id int64) (*MobileFeedbackRecord, error) {
	if r.record.ID != id {
		return nil, ErrMobileFeedbackNotFound
	}
	record := r.record
	return &record, nil
}

func (r *mobileFeedbackAdminRepoStub) UpdateAdminMobileFeedback(_ context.Context, id int64, input MobileFeedbackStatusUpdate) (*MobileFeedbackRecord, error) {
	if r.record.ID != id {
		return nil, ErrMobileFeedbackNotFound
	}
	r.statusUpdate = input
	if input.ExpectedVersion != r.record.Version {
		return nil, ErrMobileFeedbackVersionConflict
	}
	r.record.Status = input.Status
	r.record.AdminNote = input.AdminNote
	r.record.Version++
	r.record.UpdatedBy = &input.ActorAdminID
	copy := r.record
	return &copy, nil
}

func (r *mobileFeedbackAdminRepoStub) UpdateMobileFeedbackWorkItem(_ context.Context, _ int64, _ MobileFeedbackWorkItemUpdate) (*MobileFeedbackWorkItem, error) {
	if r.workItemErr != nil {
		return nil, r.workItemErr
	}
	return &MobileFeedbackWorkItem{}, nil
}

func TestUpdateAdminMobileFeedbackKeepsStatusWhenWorkItemWriteFails(t *testing.T) {
	repo := &mobileFeedbackAdminRepoStub{
		record:      mobileFeedbackSupportRecord(77, 42, "viewed"),
		workItemErr: errors.New("work item store unavailable"),
	}
	repo.record.Version = 4
	svc := NewPlayService(repo, nil, nil, nil, nil, nil)

	updated, err := svc.UpdateAdminMobileFeedback(context.Background(), 77, MobileFeedbackUpdate{
		Status:          "handled",
		AdminNote:       mobileFeedbackStringPtr("已处理"),
		ExpectedVersion: 4,
		ActorAdminID:    99,
		WorkItem:        &MobileFeedbackWorkItemUpdate{ID: 7, Status: "released"},
	})

	require.NoError(t, err)
	require.Equal(t, "handled", updated.Status)
	require.Equal(t, int64(5), updated.Version)
	require.Equal(t, int64(99), *updated.UpdatedBy)
}

func TestUpdateAdminMobileFeedbackRejectsStaleVersion(t *testing.T) {
	repo := &mobileFeedbackAdminRepoStub{record: mobileFeedbackSupportRecord(77, 42, "viewed")}
	repo.record.Version = 5
	svc := NewPlayService(repo, nil, nil, nil, nil, nil)

	_, err := svc.UpdateAdminMobileFeedback(context.Background(), 77, MobileFeedbackUpdate{
		Status:          "handled",
		ExpectedVersion: 4,
		ActorAdminID:    99,
	})

	require.ErrorIs(t, err, ErrMobileFeedbackVersionConflict)
	require.Equal(t, "viewed", repo.record.Status)
	require.Equal(t, int64(5), repo.record.Version)
}

func (r *mobileFeedbackSupportRepoStub) CreateMobileFeedback(_ context.Context, record MobileFeedbackRecord) (*MobileFeedbackRecord, error) {
	record.ID = 100
	r.created = &record
	return &record, nil
}

func (r *mobileFeedbackSupportRepoStub) ListUserMobileFeedback(_ context.Context, userID int64, filter MobileFeedbackListFilter) ([]MobileFeedbackRecord, int64, error) {
	items := make([]MobileFeedbackRecord, 0)
	for _, record := range r.records {
		if record.UserID == userID && (filter.Status == "" || record.Status == filter.Status) {
			items = append(items, record)
		}
	}
	return items, int64(len(items)), nil
}

func (r *mobileFeedbackSupportRepoStub) GetUserMobileFeedback(_ context.Context, userID, id int64) (*MobileFeedbackRecord, error) {
	record, ok := r.records[id]
	if !ok || record.UserID != userID {
		return nil, ErrMobileFeedbackNotFound
	}
	copy := record
	return &copy, nil
}

func (r *mobileFeedbackSupportRepoStub) ListMobileFeedbackMessages(_ context.Context, userID, feedbackID int64) ([]MobileFeedbackMessage, error) {
	if _, err := r.GetUserMobileFeedback(context.Background(), userID, feedbackID); err != nil {
		return nil, err
	}
	return append([]MobileFeedbackMessage(nil), r.messages[feedbackID]...), nil
}

func (r *mobileFeedbackSupportRepoStub) CreateUserMobileFeedbackMessage(_ context.Context, userID, feedbackID int64, content string) (*MobileFeedbackMessage, error) {
	record, err := r.GetUserMobileFeedback(context.Background(), userID, feedbackID)
	if err != nil {
		return nil, err
	}
	if record.Status == "ignored" {
		return nil, ErrMobileFeedbackClosed
	}
	if record.Status == "handled" {
		record.Status = "new"
	} else if record.Status == "deferred" {
		record.Status = "viewed"
	}
	record.UpdatedAt = record.UpdatedAt.Add(time.Minute)
	r.records[feedbackID] = *record
	message := MobileFeedbackMessage{ID: int64(len(r.messages[feedbackID]) + 1), FeedbackID: feedbackID, SenderType: "user", Content: content, CreatedAt: record.UpdatedAt}
	r.messages[feedbackID] = append(r.messages[feedbackID], message)
	return &message, nil
}

func (r *mobileFeedbackSupportRepoStub) CloseUserMobileFeedback(_ context.Context, userID, id int64) (*MobileFeedbackRecord, error) {
	record, err := r.GetUserMobileFeedback(context.Background(), userID, id)
	if err != nil {
		return nil, err
	}
	record.Status = "ignored"
	r.records[id] = *record
	return record, nil
}

func TestListUserMobileFeedbackMapsPublicStatusesAndOwnership(t *testing.T) {
	repo := newMobileFeedbackSupportRepoStub()
	repo.records[1] = mobileFeedbackSupportRecord(1, 42, "new")
	repo.records[2] = mobileFeedbackSupportRecord(2, 42, "deferred")
	repo.records[3] = mobileFeedbackSupportRecord(3, 99, "new")
	svc := NewPlayService(repo, nil, nil, nil, nil, nil)

	result, err := svc.ListUserMobileFeedback(context.Background(), 42, MobileFeedbackListFilter{Status: MobileSupportStatusWaiting})
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	require.Equal(t, int64(2), result.Items[0].ID)
	require.Equal(t, MobileSupportStatusWaiting, result.Items[0].Status)
	require.Equal(t, int64(1), result.Total)

	_, err = svc.ListUserMobileFeedback(context.Background(), 42, MobileFeedbackListFilter{Status: "unknown"})
	require.ErrorIs(t, err, ErrMobileFeedbackInvalid)
}

func TestCreateMobileFeedbackRedactsAutomaticDiagnostics(t *testing.T) {
	repo := newMobileFeedbackSupportRepoStub()
	svc := NewPlayService(repo, nil, nil, nil, nil, nil)
	_, err := svc.CreateMobileFeedback(context.Background(), 42, MobileFeedbackInput{
		Title: "网络错误", Category: "bug", Content: "请协助排查网络问题",
		BackendURL: "https://api.example.com/private?token=secret",
		LastError:  "Authorization: Bearer secret prompt=private-chat",
		CrashLog:   "full chat content",
	})
	require.NoError(t, err)
	require.Equal(t, "https://api.example.com", repo.created.BackendURL)
	require.NotContains(t, repo.created.LastError, "secret")
	require.NotContains(t, repo.created.LastError, "private-chat")
	require.Empty(t, repo.created.CrashLog)
}

func TestCreateMobileFeedbackStoresOnlyClassifiedErrorSummary(t *testing.T) {
	repo := newMobileFeedbackSupportRepoStub()
	svc := NewPlayService(repo, nil, nil, nil, nil, nil)
	_, err := svc.CreateMobileFeedback(context.Background(), 42, MobileFeedbackInput{
		Title: "网络错误", Category: "bug", Content: "无线网络无法同步",
		LastError: "Failed to fetch https://private.example/v1?token=secret Bearer eyJhbGciOiJIUzI1NiJ9.payload.signature sk-live-private HTTP 401",
	})
	require.NoError(t, err)
	require.Equal(t, "HTTP 401；网络连接失败", repo.created.LastError)
}

func TestGetUserMobileFeedbackUsesAdminNoteAsSupportReplyAndRedactsDiagnostics(t *testing.T) {
	repo := newMobileFeedbackSupportRepoStub()
	record := mobileFeedbackSupportRecord(1, 42, "viewed")
	record.AdminNote = "客服已收到，正在处理"
	record.LastError = "Authorization: Bearer secret"
	record.CrashLog = "private crash details"
	record.DeviceInfo = map[string]any{"api_key": "secret"}
	repo.records[1] = record
	repo.messages[1] = []MobileFeedbackMessage{{ID: 3, FeedbackID: 1, SenderType: "user", Content: "补充说明", CreatedAt: record.CreatedAt.Add(time.Minute)}}
	svc := NewPlayService(repo, nil, nil, nil, nil, nil)

	ticket, err := svc.GetUserMobileFeedback(context.Background(), 42, 1)
	require.NoError(t, err)
	require.Equal(t, MobileSupportStatusInProgress, ticket.Status)
	require.Len(t, ticket.Messages, 2)
	require.Equal(t, "support", ticket.Messages[0].SenderType)
	require.Equal(t, "客服已收到，正在处理", ticket.Messages[0].Content)
	require.Equal(t, "user", ticket.Messages[1].SenderType)
	raw, err := json.Marshal(ticket)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "Authorization")
	require.NotContains(t, string(raw), "private crash")
	require.NotContains(t, string(raw), "api_key")

	_, err = svc.GetUserMobileFeedback(context.Background(), 99, 1)
	require.ErrorIs(t, err, ErrMobileFeedbackNotFound)
}

func TestAddUserMobileFeedbackMessageReopensResolvedAndRejectsClosed(t *testing.T) {
	repo := newMobileFeedbackSupportRepoStub()
	repo.records[1] = mobileFeedbackSupportRecord(1, 42, "handled")
	repo.records[2] = mobileFeedbackSupportRecord(2, 42, "ignored")
	svc := NewPlayService(repo, nil, nil, nil, nil, nil)

	ticket, err := svc.AddUserMobileFeedbackMessage(context.Background(), 42, 1, "问题仍然存在")
	require.NoError(t, err)
	require.Equal(t, MobileSupportStatusOpen, ticket.Status)
	require.Len(t, ticket.Messages, 1)
	require.Equal(t, "问题仍然存在", ticket.Messages[0].Content)

	_, err = svc.AddUserMobileFeedbackMessage(context.Background(), 42, 2, "再次回复")
	require.ErrorIs(t, err, ErrMobileFeedbackClosed)
	_, err = svc.AddUserMobileFeedbackMessage(context.Background(), 42, 1, "   ")
	require.ErrorIs(t, err, ErrMobileFeedbackInvalid)
}

func TestCloseUserMobileFeedbackIsIdempotent(t *testing.T) {
	repo := newMobileFeedbackSupportRepoStub()
	repo.records[1] = mobileFeedbackSupportRecord(1, 42, "new")
	svc := NewPlayService(repo, nil, nil, nil, nil, nil)

	first, err := svc.CloseUserMobileFeedback(context.Background(), 42, 1)
	require.NoError(t, err)
	require.Equal(t, MobileSupportStatusClosed, first.Status)
	second, err := svc.CloseUserMobileFeedback(context.Background(), 42, 1)
	require.NoError(t, err)
	require.Equal(t, MobileSupportStatusClosed, second.Status)
}

func TestNormalizeMobileFeedbackDeviceInfoUsesAllowlistAndRejectsSensitiveKeys(t *testing.T) {
	got, err := normalizeMobileFeedbackDeviceInfo(map[string]any{
		"manufacturer": "Xiaomi",
		"model":        "Redmi K30",
		"sdkInt":       float64(33),
		"diagnostics":  "unstructured text is intentionally discarded",
		"random":       "discarded",
	})
	require.NoError(t, err)
	require.Equal(t, map[string]any{"manufacturer": "Xiaomi", "model": "Redmi K30", "sdkInt": float64(33)}, got)

	for _, key := range []string{"access_token", "api_key", "Authorization", "chat_messages", "imagePrompt", "password"} {
		t.Run(key, func(t *testing.T) {
			_, err := normalizeMobileFeedbackDeviceInfo(map[string]any{key: "secret"})
			require.ErrorIs(t, err, ErrMobileFeedbackSensitive)
		})
	}
	_, err = normalizeMobileFeedbackDeviceInfo(map[string]any{
		"diagnostics": map[string]any{"request": map[string]any{"prompt": "完整聊天内容"}},
	})
	require.ErrorIs(t, err, ErrMobileFeedbackSensitive)
}

func TestCreateMobileFeedbackPersistsOnlyWhitelistedDiagnostics(t *testing.T) {
	repo := newMobileFeedbackSupportRepoStub()
	svc := NewPlayService(repo, nil, nil, nil, nil, nil)

	created, err := svc.CreateMobileFeedback(context.Background(), 42, MobileFeedbackInput{
		Title: "网络问题", Content: "Wi-Fi 下无法同步账户", DeviceInfo: map[string]any{
			"manufacturer": "Xiaomi",
			"model":        "Redmi K30",
			"sdkInt":       float64(33),
			"diagnostics":  "legacy unstructured diagnostic text",
		},
	})
	require.NoError(t, err)
	require.Equal(t, int64(100), created.ID)
	require.Equal(t, map[string]any{
		"manufacturer": "Xiaomi", "model": "Redmi K30", "sdkInt": float64(33),
	}, repo.created.DeviceInfo)

	_, err = svc.CreateMobileFeedback(context.Background(), 42, MobileFeedbackInput{
		Title: "网络问题", Content: "Wi-Fi 下无法同步账户", DeviceInfo: map[string]any{"api_key": "secret"},
	})
	require.ErrorIs(t, err, ErrMobileFeedbackSensitive)
}

func TestCreateMobileFeedbackNormalizesSafeInstallationContext(t *testing.T) {
	repo := newMobileFeedbackSupportRepoStub()
	svc := NewPlayService(repo, nil, nil, nil, nil, nil)

	created, err := svc.CreateMobileFeedback(context.Background(), 42, MobileFeedbackInput{
		Title: "安装上下文", Content: "记录安装来源和渠道信息",
		InstallationID: " 550e8400-e29b-41d4-a716-446655440000 ", Channel: "google-play", Referrer: "utm_source=partner&utm_campaign=august",
	})

	require.NoError(t, err)
	require.Equal(t, "550e8400-e29b-41d4-a716-446655440000", created.InstallationID)
	require.Equal(t, "google-play", created.Channel)
	require.Equal(t, "utm_source=partner&utm_campaign=august", created.Referrer)

	_, err = svc.CreateMobileFeedback(context.Background(), 42, MobileFeedbackInput{
		Title: "非法安装上下文", Content: "不应接受敏感来源字段", InstallationID: "550e8400-e29b-41d4-a716-446655440000", Referrer: "token=secret",
	})
	require.ErrorIs(t, err, ErrMobileFeedbackSensitive)
}

func newMobileFeedbackSupportRepoStub() *mobileFeedbackSupportRepoStub {
	return &mobileFeedbackSupportRepoStub{records: make(map[int64]MobileFeedbackRecord), messages: make(map[int64][]MobileFeedbackMessage)}
}

func mobileFeedbackSupportRecord(id, userID int64, status string) MobileFeedbackRecord {
	createdAt := time.Date(2026, 7, 26, 10, 0, 0, 0, time.UTC)
	return MobileFeedbackRecord{
		ID: id, UserID: userID, Title: "网络问题", Category: "bug", Content: "无法同步账户信息",
		Status: status, Platform: "android", Screenshots: []MobileFeedbackScreenshot{},
		CreatedAt: createdAt, UpdatedAt: createdAt,
	}
}

func mobileFeedbackStringPtr(value string) *string { return &value }
