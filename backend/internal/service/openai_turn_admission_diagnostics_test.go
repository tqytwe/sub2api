package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAITurnAdmissionDiagnosticIsSanitized(t *testing.T) {
	var output bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })
	a := Account{ID: 71, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: "disabled", Name: "private-account-name", Credentials: map[string]any{"api_key": "private-api-key"}}
	svc := &OpenAIGatewayService{accountRepo: compactAdmissionRoutingRepo{schedulerTestOpenAIAccountRepo{accounts: []Account{a}}}}
	ctx := context.WithValue(context.Background(), ctxkey.RequestID, "private-request-id\nsecret")
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/responses/compact", strings.NewReader("private-body"))
	_, err := svc.AdmitOpenAITurn(ctx, c, &a, "private-model")
	require.True(t, IsOpenAITurnAdmissionError(err))
	var event map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(output.Bytes()), &event))
	require.Equal(t, "openai_turn_admission_denied", event["msg"])
	require.Equal(t, float64(71), event["account_id"])
	require.Equal(t, "account_ineligible", event["reason"])
	digest := sha256.Sum256([]byte("private-request-id\nsecret"))
	require.Equal(t, hex.EncodeToString(digest[:16]), event["request_id_hash"])
	for _, secret := range []string{"private-request-id", "private-api-key", "private-account-name", "private-body", "private-model"} {
		require.NotContains(t, output.String(), secret)
	}
}

func TestOpenAITurnAdmissionDiagnosticReasonAllowlist(t *testing.T) {
	for _, reason := range []string{"model_runtime_blocked", "model_rate_limited", "account_runtime_blocked", "group_membership_changed",
		"credential_refresh_unavailable", "credential_binding_changed", "credential_account_ineligible", "credential_generation_stale",
		"credential_generation_changed", "credential_token_unavailable", "credential_token_expired", "credential_state_unavailable"} {
		require.Equal(t, reason, openAITurnAdmissionReasonForLog(reason))
	}
	require.Equal(t, "unknown", openAITurnAdmissionReasonForLog("private-error-body\napi-key"))
}
