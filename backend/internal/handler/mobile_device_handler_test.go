package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type mobileDeviceHandlerTestService struct {
	registerUserID int64
	registration   service.MobileDeviceRegistration
	registerResult *service.MobileDevice
	registerErr    error
	deleteUserID   int64
	deleteID       string
	deleteErr      error
}

func (s *mobileDeviceHandlerTestService) RegisterDevice(_ context.Context, userID int64, registration service.MobileDeviceRegistration) (*service.MobileDevice, error) {
	s.registerUserID = userID
	s.registration = registration
	return s.registerResult, s.registerErr
}

func (s *mobileDeviceHandlerTestService) DeleteDevice(_ context.Context, userID int64, installationID string) error {
	s.deleteUserID = userID
	s.deleteID = installationID
	return s.deleteErr
}

func TestMobileDeviceHandlerRequiresAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fake := &mobileDeviceHandlerTestService{}
	handler := NewMobileDeviceHandler(fake)
	recorder := performMobileDeviceRequest(handler.Register, http.MethodPut, "/mobile/devices/id", []byte(`{}`), 0, "id")

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Zero(t, fake.registerUserID)
}

func TestMobileDeviceHandlerRegistersWithoutReturningFullToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	installationID := uuid.NewString()
	token := "fcm-token-that-must-never-be-returned"
	now := time.Date(2026, time.July, 26, 12, 0, 0, 0, time.UTC)
	fake := &mobileDeviceHandlerTestService{registerResult: &service.MobileDevice{
		ID: uuid.NewString(), InstallationID: installationID, Platform: "android",
		PushProvider: "fcm", TokenFingerprint: "abcd1234abcd", AppVersion: "2.0.34",
		Locale: "zh-CN", Enabled: true, LastSeenAt: now, CreatedAt: now, UpdatedAt: now,
	}}
	handler := NewMobileDeviceHandler(fake)
	body, err := json.Marshal(map[string]any{"fcm_token": token, "platform": "android", "app_version": "2.0.34", "locale": "zh-CN"})
	require.NoError(t, err)

	recorder := performMobileDeviceRequest(handler.Register, http.MethodPut, "/mobile/devices/"+installationID, body, 51, installationID)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, int64(51), fake.registerUserID)
	require.Equal(t, installationID, fake.registration.InstallationID)
	require.Equal(t, token, fake.registration.FCMToken)
	require.NotContains(t, recorder.Body.String(), token)
	require.Contains(t, recorder.Body.String(), "abcd1234abcd")
	require.Equal(t, "private, no-store", recorder.Header().Get("Cache-Control"))
}

func TestMobileDeviceHandlerValidatesInputBeforeService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fake := &mobileDeviceHandlerTestService{}
	handler := NewMobileDeviceHandler(fake)
	recorder := performMobileDeviceRequest(handler.Register, http.MethodPut, "/mobile/devices/id", []byte(`{"platform":"android"}`), 7, "id")

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Zero(t, fake.registerUserID)
}

func TestMobileDeviceHandlerDeleteUsesAuthenticatedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	installationID := uuid.NewString()
	fake := &mobileDeviceHandlerTestService{}
	handler := NewMobileDeviceHandler(fake)

	recorder := performMobileDeviceRequest(handler.Delete, http.MethodDelete, "/mobile/devices/"+installationID, nil, 63, installationID)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, int64(63), fake.deleteUserID)
	require.Equal(t, installationID, fake.deleteID)
	require.NotContains(t, recorder.Body.String(), "token")
}

func TestMobileDeviceHandlerDeleteDoesNotRevealForeignOwnership(t *testing.T) {
	gin.SetMode(gin.TestMode)
	installationID := uuid.NewString()
	fake := &mobileDeviceHandlerTestService{deleteErr: service.ErrMobileDeviceNotFound}
	handler := NewMobileDeviceHandler(fake)

	recorder := performMobileDeviceRequest(handler.Delete, http.MethodDelete, "/mobile/devices/"+installationID, nil, 71, installationID)

	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Equal(t, int64(71), fake.deleteUserID)
	require.NotContains(t, recorder.Body.String(), "owner")
	require.NotContains(t, recorder.Body.String(), "user_id")
}

func performMobileDeviceRequest(method gin.HandlerFunc, httpMethod, target string, body []byte, userID int64, installationID string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(httpMethod, target, bytes.NewReader(body))
	if len(body) > 0 {
		request.Header.Set("Content-Type", "application/json")
	}
	context, _ := gin.CreateTestContext(recorder)
	context.Request = request
	context.Params = gin.Params{{Key: "installation_id", Value: installationID}}
	if userID > 0 {
		context.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: userID})
	}
	method(context)
	return recorder
}
