//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type mobileSupportHandlerServiceStub struct {
	listFilter service.MobileFeedbackListFilter
	userID     int64
	ticketID   int64
	content    string
}

func (s *mobileSupportHandlerServiceStub) CreateMobileFeedback(_ context.Context, userID int64, input service.MobileFeedbackInput) (*service.MobileFeedbackRecord, error) {
	s.userID, s.content = userID, input.Content
	return &service.MobileFeedbackRecord{ID: 1, UserID: userID, Title: input.Title, Content: input.Content}, nil
}

func (s *mobileSupportHandlerServiceStub) ListUserMobileFeedback(_ context.Context, userID int64, filter service.MobileFeedbackListFilter) (*service.MobileSupportTicketList, error) {
	s.userID, s.listFilter = userID, filter
	return &service.MobileSupportTicketList{Items: []service.MobileSupportTicket{}, Page: filter.Page, PageSize: filter.PageSize}, nil
}

func (s *mobileSupportHandlerServiceStub) GetUserMobileFeedback(_ context.Context, userID, id int64) (*service.MobileSupportTicket, error) {
	s.userID, s.ticketID = userID, id
	return &service.MobileSupportTicket{ID: id, Status: service.MobileSupportStatusOpen}, nil
}

func (s *mobileSupportHandlerServiceStub) AddUserMobileFeedbackMessage(_ context.Context, userID, id int64, content string) (*service.MobileSupportTicket, error) {
	s.userID, s.ticketID, s.content = userID, id, content
	return &service.MobileSupportTicket{ID: id, Status: service.MobileSupportStatusInProgress}, nil
}

func (s *mobileSupportHandlerServiceStub) CloseUserMobileFeedback(_ context.Context, userID, id int64) (*service.MobileSupportTicket, error) {
	s.userID, s.ticketID = userID, id
	return &service.MobileSupportTicket{ID: id, Status: service.MobileSupportStatusClosed}, nil
}

func TestMobileSupportHandlerListUsesAuthenticatedUserAndPagination(t *testing.T) {
	svc := &mobileSupportHandlerServiceStub{}
	router := newMobileSupportHandlerTestRouter(svc)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/support?page=2&page_size=15&status=waiting_user", nil)
	req.Header.Set("Authorization", "Bearer valid")
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, int64(42), svc.userID)
	require.Equal(t, 2, svc.listFilter.Page)
	require.Equal(t, 15, svc.listFilter.PageSize)
	require.Equal(t, service.MobileSupportStatusWaiting, svc.listFilter.Status)
}

func TestMobileSupportHandlerDetailAndCloseUsePathTicketID(t *testing.T) {
	svc := &mobileSupportHandlerServiceStub{}
	router := newMobileSupportHandlerTestRouter(svc)

	detail := httptest.NewRecorder()
	detailReq := httptest.NewRequest(http.MethodGet, "/support/77", nil)
	detailReq.Header.Set("Authorization", "Bearer valid")
	router.ServeHTTP(detail, detailReq)
	require.Equal(t, http.StatusOK, detail.Code)
	require.Equal(t, int64(77), svc.ticketID)

	closeRecorder := httptest.NewRecorder()
	closeReq := httptest.NewRequest(http.MethodPost, "/support/77/close", nil)
	closeReq.Header.Set("Authorization", "Bearer valid")
	router.ServeHTTP(closeRecorder, closeReq)
	require.Equal(t, http.StatusOK, closeRecorder.Code)
	var envelope struct {
		Data service.MobileSupportTicket `json:"data"`
	}
	require.NoError(t, json.Unmarshal(closeRecorder.Body.Bytes(), &envelope))
	require.Equal(t, service.MobileSupportStatusClosed, envelope.Data.Status)
}

func TestMobileSupportHandlerAddMessageValidatesChineseErrors(t *testing.T) {
	svc := &mobileSupportHandlerServiceStub{}
	router := newMobileSupportHandlerTestRouter(svc)

	empty := httptest.NewRecorder()
	emptyReq := httptest.NewRequest(http.MethodPost, "/support/8/messages", strings.NewReader(`{"content":" "}`))
	emptyReq.Header.Set("Authorization", "Bearer valid")
	emptyReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(empty, emptyReq)
	require.Equal(t, http.StatusBadRequest, empty.Code)
	require.Contains(t, empty.Body.String(), "回复内容不能为空")

	valid := httptest.NewRecorder()
	validReq := httptest.NewRequest(http.MethodPost, "/support/8/messages", strings.NewReader(`{"content":"补充网络截图"}`))
	validReq.Header.Set("Authorization", "Bearer valid")
	validReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(valid, validReq)
	require.Equal(t, http.StatusOK, valid.Code)
	require.Equal(t, int64(42), svc.userID)
	require.Equal(t, int64(8), svc.ticketID)
	require.Equal(t, "补充网络截图", svc.content)
}

func TestMobileSupportHandlerRequiresAuthenticationAndValidID(t *testing.T) {
	svc := &mobileSupportHandlerServiceStub{}
	router := newMobileSupportHandlerTestRouter(svc)

	unauthorized := httptest.NewRecorder()
	router.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/support", nil))
	require.Equal(t, http.StatusUnauthorized, unauthorized.Code)
	require.Contains(t, unauthorized.Body.String(), "请先登录")

	badID := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/support/not-a-number", nil)
	req.Header.Set("Authorization", "Bearer valid")
	router.ServeHTTP(badID, req)
	require.Equal(t, http.StatusBadRequest, badID.Code)
	require.Contains(t, badID.Body.String(), "工单编号不正确")
}

func newMobileSupportHandlerTestRouter(svc mobileSupportService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if c.GetHeader("Authorization") == "Bearer valid" {
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
		}
		c.Next()
	})
	h := NewMobileSupportHandler(svc)
	router.GET("/support", h.List)
	router.GET("/support/:id", h.Detail)
	router.POST("/support/:id/messages", h.AddMessage)
	router.POST("/support/:id/close", h.Close)
	return router
}
