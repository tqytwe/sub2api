package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type mobileAttributionHandlerStub struct {
	userID int64
	input  service.MobileAttributionEventInput
	result service.MobileAttributionRecordResult
	err    error
}

func (s *mobileAttributionHandlerStub) RecordEvent(_ context.Context, userID int64, input service.MobileAttributionEventInput) (service.MobileAttributionRecordResult, error) {
	s.userID, s.input = userID, input
	return s.result, s.err
}

func TestMobileAttributionHandlerAllowsAnonymousEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &mobileAttributionHandlerStub{result: service.MobileAttributionRecordResult{Created: true}}
	h := NewMobileAttributionHandler(stub)
	r := gin.New()
	r.POST("/events", h.Event)
	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(`{"installation_id":"`+uuid.NewString()+`","event_type":"open","idempotency_key":"open-event-1","platform":"android"}`))
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	require.Equal(t, http.StatusOK, res.Code)
	require.Zero(t, stub.userID)
	require.Equal(t, "open", stub.input.EventType)
}

func TestMobileAttributionHandlerPassesAuthenticatedUserForRegister(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &mobileAttributionHandlerStub{result: service.MobileAttributionRecordResult{Created: true}}
	h := NewMobileAttributionHandler(stub)
	r := gin.New()
	r.POST("/events", func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 19})
		h.Event(c)
	})
	req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(`{"installation_id":"`+uuid.NewString()+`","event_type":"register","idempotency_key":"register-event-1","platform":"ios"}`))
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	require.Equal(t, http.StatusOK, res.Code)
	require.Equal(t, int64(19), stub.userID)
}
