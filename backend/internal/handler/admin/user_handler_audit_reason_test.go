package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type updateUserAuditCaptureService struct {
	service.AdminService
	input *service.UpdateUserInput
	calls int
}

func (s *updateUserAuditCaptureService) UpdateUser(_ context.Context, userID int64, input *service.UpdateUserInput) (*service.User, error) {
	s.calls++
	s.input = input
	return &service.User{ID: userID, Email: input.Email, Status: service.StatusActive}, nil
}

type updateUserAuditCaptureRepository struct {
	mu   sync.Mutex
	logs []*service.AuditLog
}

func (r *updateUserAuditCaptureRepository) BatchInsert(_ context.Context, logs []*service.AuditLog) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logs = append(r.logs, logs...)
	return int64(len(logs)), nil
}

func (r *updateUserAuditCaptureRepository) Insert(_ context.Context, log *service.AuditLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logs = append(r.logs, log)
	return nil
}

func (*updateUserAuditCaptureRepository) List(context.Context, *service.AuditLogFilter) (*service.AuditLogList, error) {
	return &service.AuditLogList{}, nil
}

func (*updateUserAuditCaptureRepository) GetByID(context.Context, int64) (*service.AuditLog, error) {
	return nil, service.ErrAuditLogNotFound
}

func (*updateUserAuditCaptureRepository) Count(context.Context) (int64, error) { return 0, nil }
func (*updateUserAuditCaptureRepository) TruncateAll(context.Context) error    { return nil }
func (*updateUserAuditCaptureRepository) DeleteBefore(context.Context, time.Time, int) (int64, error) {
	return 0, nil
}

func TestUpdateUserAuditReasonIsAcceptedAndOnlyRecordedForAudit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminService := &updateUserAuditCaptureService{}
	auditRepository := &updateUserAuditCaptureRepository{}
	auditService := service.NewAuditLogService(auditRepository, nil)
	auditService.Start()
	defer auditService.Stop()

	router := gin.New()
	router.Use(gin.HandlerFunc(middleware.NewAuditLogMiddleware(auditService)))
	router.PUT("/api/v1/admin/users/:id", NewUserHandler(adminService, nil, nil, nil, nil, nil, nil).Update)

	reason := strings.Repeat("界", 500)
	body, err := json.Marshal(map[string]string{
		"reason":   reason,
		"email":    "updated@example.com",
		"username": "updated-user",
		"password": "valid-password",
	})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/v1/admin/users/42", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, 1, adminService.calls)
	require.Equal(t, "updated@example.com", adminService.input.Email)
	require.Equal(t, "updated-user", *adminService.input.Username)
	require.Equal(t, "valid-password", adminService.input.Password)

	auditService.Stop()
	auditRepository.mu.Lock()
	logs := append([]*service.AuditLog(nil), auditRepository.logs...)
	auditRepository.mu.Unlock()
	require.Len(t, logs, 1)

	var auditedBody map[string]any
	require.NoError(t, json.Unmarshal([]byte(logs[0].RequestBody), &auditedBody))
	require.Equal(t, reason, auditedBody["reason"])
	require.Equal(t, "updated@example.com", auditedBody["email"])
	require.Equal(t, "updated-user", auditedBody["username"])
	require.Equal(t, "***", auditedBody["password"])
}

func TestUpdateUserAuditReasonRejectsMoreThan500Characters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminService := &updateUserAuditCaptureService{}
	router := gin.New()
	router.PUT("/api/v1/admin/users/:id", NewUserHandler(adminService, nil, nil, nil, nil, nil, nil).Update)

	body, err := json.Marshal(map[string]string{"reason": strings.Repeat("a", 501)})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/v1/admin/users/42", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Zero(t, adminService.calls)
}
