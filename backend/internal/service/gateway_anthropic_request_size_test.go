package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type requestSizeRejectedBody struct{ closed bool }

func (*requestSizeRejectedBody) Read([]byte) (int, error) {
	return 0, errors.New("upstream rejection body must not be read")
}

func (b *requestSizeRejectedBody) Close() error {
	b.closed = true
	return nil
}

func TestAnthropicRequestSizeAppliesOnlyToDirectMessagesAccounts(t *testing.T) {
	resp := &http.Response{StatusCode: http.StatusRequestEntityTooLarge}
	for _, accountType := range []string{AccountTypeAPIKey, AccountTypeOAuth, AccountTypeSetupToken} {
		require.True(t, isAnthropicRequestTooLarge(&Account{Platform: PlatformAnthropic, Type: accountType}, resp))
	}
	require.False(t, isAnthropicRequestTooLarge(&Account{Platform: PlatformAnthropic, Type: AccountTypeBedrock}, resp))
	require.False(t, isAnthropicRequestTooLarge(&Account{Platform: PlatformGemini, Type: AccountTypeAPIKey}, resp))
	require.False(t, isAnthropicRequestTooLarge(nil, resp))
	require.False(t, isAnthropicRequestTooLarge(&Account{Platform: PlatformAnthropic, Type: AccountTypeAPIKey}, nil))
}

func TestAnthropicRequestSizeClosesBodyWithoutReadingOrWritingAfterCancellation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(ctx)
	body := &requestSizeRejectedBody{}
	_, err := handleAnthropicRequestTooLarge(c, &http.Response{StatusCode: http.StatusRequestEntityTooLarge, Body: body})
	require.ErrorIs(t, err, context.Canceled)
	require.True(t, body.closed)
	require.False(t, c.Writer.Written())
	require.Empty(t, rec.Body.String())
	require.False(t, IsOpsRequestScopedError(c))
}

func TestAnthropicRequestSizePreservesCommittedNonSSEBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Data(http.StatusOK, "application/json", []byte(`{"existing":true}`))
	body := &requestSizeRejectedBody{}
	_, err := handleAnthropicRequestTooLarge(c, &http.Response{StatusCode: http.StatusRequestEntityTooLarge, Body: body})
	require.ErrorIs(t, err, errAnthropicRequestTooLarge)
	require.True(t, body.closed)
	require.Equal(t, `{"existing":true}`, rec.Body.String())
	require.NotContains(t, rec.Body.String(), "event: error")
	marked, ok := GetOpsStreamError(c)
	require.True(t, ok)
	require.True(t, marked.RequestScoped)
	require.Equal(t, http.StatusRequestEntityTooLarge, marked.IntendedStatus)
	require.True(t, strings.Contains(marked.Message, "32 MiB"))
}
