package admin

import (
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
