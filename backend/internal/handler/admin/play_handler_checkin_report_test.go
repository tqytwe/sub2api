package admin

import (
	"context"
	"encoding/json"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCheckinReportRejectsMalformedPaginationBeforeStorage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := &AdminPlayHandler{}
	router.GET("/records", h.ListCheckinRecords)
	for _, query := range []string{"user_id=no", "user_id=-1", "user_id=0", "page=no", "page=-1", "page_size=0"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/records?"+query, nil))
		require.Equal(t, http.StatusBadRequest, response.Code, query)
	}
}

func TestCheckinRepairRequiresAuthenticatedActorAndRejectsMalformedBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := &AdminPlayHandler{}
	router.POST("/repair", h.RepairCheckins)
	for _, test := range []struct {
		body   string
		status int
	}{{"invalid", 400}, {`{"actor_user_id":1,"reason":"forged actor"}`, 401}} {
		request := httptest.NewRequest(http.MethodPost, "/repair", strings.NewReader(test.body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		require.Equal(t, test.status, response.Code)
	}
}

type checkinAccountReportRepo struct {
	service.PlayRepository
	filter service.PlayAdminCheckinFilter
}

func (r *checkinAccountReportRepo) ListAdminCheckins(_ context.Context, f service.PlayAdminCheckinFilter) (*service.PlayAdminCheckinPage, error) {
	r.filter = f
	return &service.PlayAdminCheckinPage{Items: []service.PlayAdminCheckinRecord{{Email: "example@example.test", Username: "account55", StreakAtCheckin: 3, ObservedStreak: 3, StreakStartDate: "2026-11-01"}}}, nil
}
func TestCheckinReportUsesAccountQueryWithoutReinterpretingItAsUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &checkinAccountReportRepo{}
	h := &AdminPlayHandler{playService: service.NewPlayService(repo, nil, nil, nil, nil, nil)}
	router := gin.New()
	router.GET("/records", h.ListCheckinRecords)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/records?account=%2055%20", nil))
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "55", repo.filter.Account)
	require.Zero(t, repo.filter.UserID)
	var body struct {
		Data service.PlayAdminCheckinPage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	require.Equal(t, "example@example.test", body.Data.Items[0].Email)
	require.Equal(t, "2026-11-01", body.Data.Items[0].StreakStartDate)
}
