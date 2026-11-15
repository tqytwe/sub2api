package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newPelicanTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := &AccountHandler{}
	router.GET("/accounts/pelican-question", handler.PreparePelicanQuestion)
	router.POST("/accounts/:id/pelican-test", handler.PelicanTest)
	return router
}

func TestPreparePelicanQuestionIssuesManualRoundSnapshot(t *testing.T) {
	router := newPelicanTestRouter()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/accounts/pelican-question", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var envelope struct {
		Data service.PelicanTestConfig `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, service.PelicanPromptModeRandom, envelope.Data.PromptMode)
	require.NotNil(t, envelope.Data.PelicanQuestion)
	require.NotEmpty(t, envelope.Data.ActualPrompt)
	require.NotEmpty(t, envelope.Data.PelicanQuestion.Criteria)
	require.Contains(t, envelope.Data.PelicanQuestion.RoundID, "manual:")
}

func TestPelicanTestRejectsInvalidQuestionIdentityAndMode(t *testing.T) {
	router := newPelicanTestRouter()
	tests := []struct {
		name string
		body string
	}{
		{
			name: "client cannot relabel a random task",
			body: `{"prompt_mode":"random","pelican_question":{"id":"forged","version":"svg-motion-v1","round_id":"manual:0123456789abcdef0123456789abcdef"}}`,
		},
		{
			name: "unknown prompt mode is rejected",
			body: `{"prompt_mode":"random_reasoning","prompt":"x"}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/accounts/42/pelican-test", bytes.NewBufferString(test.body))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, request)
			require.Equal(t, http.StatusBadRequest, recorder.Code)
		})
	}
}
