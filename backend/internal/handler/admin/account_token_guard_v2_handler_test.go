package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestResumeReloginBudgetRequiresConfirmation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/accounts/7/resume-relogin-budget", strings.NewReader(`{"confirm":false}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = gin.Params{{Key: "id", Value: "7"}}

	handler := NewAccountTokenGuardV2Handler(nil)
	handler.ResumeReloginBudget(ctx)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}
