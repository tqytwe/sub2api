package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type complianceHandlerSettingRepo struct {
	mu       sync.Mutex
	values   map[string]string
	setCalls int
}

func newComplianceHandlerSettingRepo() *complianceHandlerSettingRepo {
	return &complianceHandlerSettingRepo{values: map[string]string{}}
}

func (r *complianceHandlerSettingRepo) Get(ctx context.Context, key string) (*service.Setting, error) {
	value, err := r.GetValue(ctx, key)
	if err != nil {
		return nil, err
	}
	return &service.Setting{Key: key, Value: value}, nil
}

func (r *complianceHandlerSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, ok := r.values[key]
	if !ok {
		return "", service.ErrSettingNotFound
	}
	return value, nil
}

func (r *complianceHandlerSettingRepo) Set(_ context.Context, key, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.values[key] = value
	r.setCalls++
	return nil
}

func (r *complianceHandlerSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	values := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			values[key] = value
		}
	}
	return values, nil
}

func (r *complianceHandlerSettingRepo) SetMultiple(_ context.Context, values map[string]string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key, value := range values {
		r.values[key] = value
	}
	return nil
}

func (r *complianceHandlerSettingRepo) GetAll(context.Context) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	values := make(map[string]string, len(r.values))
	for key, value := range r.values {
		values[key] = value
	}
	return values, nil
}

func (r *complianceHandlerSettingRepo) Delete(_ context.Context, key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.values, key)
	return nil
}

func (r *complianceHandlerSettingRepo) writes() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.setCalls
}

type complianceHandlerAuditRepo struct {
	mu   sync.Mutex
	logs []*service.AuditLog
}

func (r *complianceHandlerAuditRepo) BatchInsert(_ context.Context, logs []*service.AuditLog) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logs = append(r.logs, logs...)
	return int64(len(logs)), nil
}

func (r *complianceHandlerAuditRepo) Insert(_ context.Context, log *service.AuditLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logs = append(r.logs, log)
	return nil
}

func (*complianceHandlerAuditRepo) List(context.Context, *service.AuditLogFilter) (*service.AuditLogList, error) {
	return &service.AuditLogList{}, nil
}

func (*complianceHandlerAuditRepo) GetByID(context.Context, int64) (*service.AuditLog, error) {
	return nil, service.ErrAuditLogNotFound
}

func (*complianceHandlerAuditRepo) Count(context.Context) (int64, error) { return 0, nil }
func (*complianceHandlerAuditRepo) TruncateAll(context.Context) error    { return nil }
func (*complianceHandlerAuditRepo) DeleteBefore(context.Context, time.Time, int) (int64, error) {
	return 0, nil
}

func (r *complianceHandlerAuditRepo) entries() []*service.AuditLog {
	r.mu.Lock()
	defer r.mu.Unlock()
	entries := make([]*service.AuditLog, len(r.logs))
	copy(entries, r.logs)
	return entries
}

func newComplianceHandlerRouter(handler *ComplianceHandler, auditService *service.AuditLogService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.RequestLogger())
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
		c.Set(string(middleware.ContextKeyUserRole), "admin")
		c.Next()
	})
	if auditService != nil {
		router.Use(gin.HandlerFunc(middleware.NewAuditLogMiddleware(auditService)))
	}
	router.POST("/api/v1/admin/compliance/accept", handler.Accept)
	return router
}

type complianceAcceptResponse struct {
	Code   int                           `json:"code"`
	Data   service.AdminComplianceStatus `json:"data"`
	Reason string                        `json:"reason"`
}

func postComplianceAccept(
	t *testing.T,
	router *gin.Engine,
	idempotencyKey string,
	requestID string,
) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(AcceptAdminComplianceRequest{
		Phrase:   service.AdminComplianceAckPhraseEN,
		Language: "en",
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/compliance/accept", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	if requestID != "" {
		req.Header.Set("X-Request-ID", requestID)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestComplianceAcceptIdempotencyReplaysOriginalAcknowledgement(t *testing.T) {
	repo := newComplianceHandlerSettingRepo()
	handler := NewComplianceHandler(service.NewSettingService(repo, &config.Config{}))
	idempotencyRepo := newMemoryIdempotencyRepoStub()
	idempotencyConfig := service.DefaultIdempotencyConfig()
	idempotencyConfig.ObserveOnly = false
	service.SetDefaultIdempotencyCoordinator(service.NewIdempotencyCoordinator(idempotencyRepo, idempotencyConfig))
	t.Cleanup(func() { service.SetDefaultIdempotencyCoordinator(nil) })

	router := newComplianceHandlerRouter(handler, nil)
	first := postComplianceAccept(t, router, "admin-compliance-accept-42", "rid-compliance-accept-first")
	second := postComplianceAccept(t, router, "admin-compliance-accept-42", "rid-compliance-accept-replay")

	require.Equal(t, http.StatusOK, first.Code)
	require.Equal(t, http.StatusOK, second.Code)
	require.Equal(t, "true", second.Header().Get("X-Idempotency-Replayed"))
	require.Equal(t, 1, repo.writes(), "a replay must not rewrite the acknowledgement")

	var firstResponse, secondResponse complianceAcceptResponse
	require.NoError(t, json.Unmarshal(first.Body.Bytes(), &firstResponse))
	require.NoError(t, json.Unmarshal(second.Body.Bytes(), &secondResponse))
	require.Equal(t, 0, firstResponse.Code)
	require.Equal(t, 0, secondResponse.Code)
	require.False(t, firstResponse.Data.Required)
	require.False(t, secondResponse.Data.Required)
	require.NotNil(t, firstResponse.Data.Acknowledgement)
	require.NotNil(t, secondResponse.Data.Acknowledgement)
	require.Equal(t, firstResponse.Data.Acknowledgement.AcceptedAt, secondResponse.Data.Acknowledgement.AcceptedAt)

	idempotencyRepo.mu.Lock()
	records := make([]*service.IdempotencyRecord, 0, len(idempotencyRepo.data))
	for _, record := range idempotencyRepo.data {
		records = append(records, idempotencyRepo.clone(record))
	}
	idempotencyRepo.mu.Unlock()
	require.Len(t, records, 1)
	require.Equal(t, adminComplianceAcceptOperation, records[0].Scope)
}

func TestComplianceAcceptRequiresIdempotencyKeyWhenEnforced(t *testing.T) {
	repo := newComplianceHandlerSettingRepo()
	handler := NewComplianceHandler(service.NewSettingService(repo, &config.Config{}))
	idempotencyConfig := service.DefaultIdempotencyConfig()
	idempotencyConfig.ObserveOnly = false
	service.SetDefaultIdempotencyCoordinator(service.NewIdempotencyCoordinator(newMemoryIdempotencyRepoStub(), idempotencyConfig))
	t.Cleanup(func() { service.SetDefaultIdempotencyCoordinator(nil) })

	response := postComplianceAccept(t, newComplianceHandlerRouter(handler, nil), "", "rid-compliance-missing-key")
	require.Equal(t, http.StatusBadRequest, response.Code)
	require.Equal(t, 0, repo.writes())

	var body complianceAcceptResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	require.Equal(t, "IDEMPOTENCY_KEY_REQUIRED", body.Reason)
}

func TestComplianceAcceptAuditCorrelatesIncomingRequestID(t *testing.T) {
	repo := newComplianceHandlerSettingRepo()
	handler := NewComplianceHandler(service.NewSettingService(repo, &config.Config{}))
	auditRepo := &complianceHandlerAuditRepo{}
	auditService := service.NewAuditLogService(auditRepo, nil)
	auditService.Start()

	router := newComplianceHandlerRouter(handler, auditService)
	requestID := "rid-admin-compliance-accept"
	response := postComplianceAccept(t, router, "", requestID)
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, requestID, response.Header().Get("X-Request-ID"))

	auditService.Stop()
	entries := auditRepo.entries()
	require.Len(t, entries, 1)
	require.Equal(t, adminComplianceAcceptOperation, entries[0].Action)
	require.Equal(t, requestID, entries[0].RequestID)
	require.Equal(t, http.MethodPost, entries[0].Method)
	require.Equal(t, "/api/v1/admin/compliance/accept", entries[0].Path)
	require.Equal(t, http.StatusOK, entries[0].StatusCode)
	require.NotNil(t, entries[0].ActorUserID)
	require.EqualValues(t, 42, *entries[0].ActorUserID)
	require.Equal(t, "admin", entries[0].ActorRole)
}
