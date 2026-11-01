package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIAdminAuthURLRejectsUnscopedExcelFlow(t *testing.T) {
	svc := service.NewOpenAIOAuthService(nil, nil)
	defer svc.Stop()
	handler := &OpenAIOAuthHandler{openaiOAuthService: svc}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/generate-auth-url", strings.NewReader(`{"oauth_client":"excel"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	handler.GenerateAuthURL(c)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.NotContains(t, w.Body.String(), openai.ExcelClientID)
}
