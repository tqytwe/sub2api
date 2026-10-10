package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
)

type openAITurnAdmissionLogAtForwardBoundaryKey struct{}

// Log only local admission metadata. Never include err.Error(), account names,
// models, credentials or request payloads; a request ID may itself be user input.
func logOpenAITurnAdmissionDenial(ctx context.Context, account *Account, err error) {
	var denied *OpenAITurnAdmissionError
	if !errors.As(err, &denied) || denied == nil {
		return
	}
	var accountID int64
	if account != nil {
		accountID = account.ID
	}
	requestIDHash := ""
	if ctx != nil {
		if requestID, ok := ctx.Value(ctxkey.RequestID).(string); ok && requestID != "" {
			digest := sha256.Sum256([]byte(requestID))
			requestIDHash = hex.EncodeToString(digest[:16])
		}
	}
	slog.WarnContext(ctx, "openai_turn_admission_denied",
		"account_id", accountID, "request_id_hash", requestIDHash,
		"reason", openAITurnAdmissionReasonForLog(denied.Reason))
}

func openAITurnAdmissionReasonForLog(reason string) string {
	switch reason {
	case openAIResponsesToolsProtocolMismatch, "account_unavailable", "latest_state_unavailable", "account_binding_changed",
		"account_ineligible", "proxy_unavailable", "credential_parent_ineligible",
		"credential_parent_binding_changed", "group_membership_changed", "group_unavailable",
		"model_not_allowed_in_group", "account_runtime_blocked", "model_runtime_blocked",
		"model_rate_limited", "credential_snapshot_unavailable", "credential_refresh_unavailable",
		"credential_binding_changed", "credential_account_ineligible", "credential_generation_stale",
		"credential_generation_changed", "credential_token_unavailable", "credential_token_expired",
		"credential_state_unavailable":
		return reason
	default:
		return "unknown"
	}
}
