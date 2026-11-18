package handler

import (
	"errors"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIRPMRetryAfterOnlyForExhaustion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	admission := openAIRPMAdmission{resetAt: time.Now().Add(15 * time.Second)}
	admission.retryAfter(c, service.ErrOpenAIRPMUnavailable)
	require.Empty(t, recorder.Header().Get("Retry-After"))
	admission.retryAfter(c, service.ErrOpenAIRPMExhausted)
	seconds, err := strconv.Atoi(recorder.Header().Get("Retry-After"))
	require.NoError(t, err)
	require.GreaterOrEqual(t, seconds, 1)
	require.LessOrEqual(t, seconds, 15)
	require.ErrorIs(t, admission.selectionError(errors.Join(service.ErrNoAvailableAccounts, service.ErrOpenAIRPMExhausted)), service.ErrOpenAIRPMExhausted)
}
