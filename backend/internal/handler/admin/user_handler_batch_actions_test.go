package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUserHandlerPreviewBatchActionReturnsImpact(t *testing.T) {
	gin.SetMode(gin.TestMode)
	serviceStub := newStubAdminService()
	serviceStub.userBatchPreviewResult = &service.UserBatchActionPreview{
		Action:         service.UserBatchActionDelete,
		RequestedCount: 3,
		EligibleUsers: []service.UserBatchActionTarget{
			{ID: 1, Email: "one@example.test", APIKeyCount: 2},
		},
		ProtectedAdministrators: []service.UserBatchActionTarget{
			{ID: 2, Email: "admin@example.test", Role: service.RoleAdmin},
		},
		MissingUserIDs:    []int64{404},
		AffectedAPIKeys:   2,
		RequiresStepUp:    true,
		ConfirmationToken: "preview-token",
		ExpiresAt:         time.Date(2026, 7, 24, 10, 5, 0, 0, time.UTC),
	}
	handler := NewUserHandler(serviceStub, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.POST("/api/v1/admin/users/batch-actions/preview", handler.PreviewBatchAction)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/users/batch-actions/preview",
		bytes.NewBufferString(`{"action":"delete","user_ids":[1,2,404],"reason":"confirmed abuse"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, service.UserBatchActionDelete, serviceStub.lastUserBatchPreviewInput.Action)
	require.Equal(t, []int64{1, 2, 404}, serviceStub.lastUserBatchPreviewInput.UserIDs)
	require.Equal(t, "confirmed abuse", serviceStub.lastUserBatchPreviewInput.Reason)
	var envelope struct {
		Data service.UserBatchActionPreview `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, 2, envelope.Data.AffectedAPIKeys)
	require.Equal(t, "preview-token", envelope.Data.ConfirmationToken)
}

func TestUserHandlerPreviewBatchActionRejectsMalformedRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	serviceStub := newStubAdminService()
	handler := NewUserHandler(serviceStub, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.POST("/api/v1/admin/users/batch-actions/preview", handler.PreviewBatchAction)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/users/batch-actions/preview",
		bytes.NewBufferString(`{"action":"disable","user_ids":[],"reason":""}`),
	)
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Empty(t, serviceStub.lastUserBatchPreviewInput.UserIDs)
}

func TestUserHandlerExecuteBatchActionRequiresStepUp(t *testing.T) {
	gin.SetMode(gin.TestMode)
	serviceStub := newStubAdminService()
	handler := NewUserHandler(serviceStub, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.POST("/api/v1/admin/users/batch-actions", handler.ExecuteBatchAction)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/users/batch-actions",
		bytes.NewBufferString(`{"action":"disable","user_ids":[1],"reason":"confirmed abuse","confirmation_token":"preview-token"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Empty(t, serviceStub.lastUserBatchExecuteInput.UserIDs)
}

func TestUserHandlerExecuteBatchActionRejectsMissingPreviewToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	serviceStub := newStubAdminService()
	handler := NewUserHandler(serviceStub, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.POST("/api/v1/admin/users/batch-actions", handler.ExecuteBatchAction)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/users/batch-actions",
		bytes.NewBufferString(`{"action":"disable","user_ids":[1],"reason":"confirmed abuse"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "USER_BATCH_ACTION_PREVIEW_INVALID")
	require.Empty(t, serviceStub.lastUserBatchExecuteInput.UserIDs)
}
