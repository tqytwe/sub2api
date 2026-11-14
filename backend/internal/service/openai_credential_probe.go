package service

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/requestledger"
)

type openAICredentialProbeContextKey struct{}

func isFreshOpenAICredentialProbe(ctx context.Context) bool {
	fresh, _ := ctx.Value(openAICredentialProbeContextKey{}).(bool)
	return fresh
}

// FetchOpenAIModelsListFresh verifies an OAuth parent account against the native
// Codex endpoint without reusing or publishing shared model-catalog evidence.
func (s *OpenAIGatewayService) FetchOpenAIModelsListFresh(ctx context.Context, account *Account) (result *OpenAIModelsResponse, probeErr error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || account == nil || !account.IsOpenAIOAuth() || account.IsCredentialShadow() || account.IsOpenAIPersonalAccessToken() || account.IsOpenAIAgentIdentity() {
		return nil, infraerrors.BadRequest("OPENAI_CREDENTIAL_PROBE_ACCOUNT_INVALID", "credential inspection requires an OpenAI OAuth parent account")
	}
	// Bypassing the shared refresh must retain its independent, non-billing
	// execution receipt. The upstream request itself remains caller-cancellable.
	ctx = requestledger.CurrentContext(ctx)
	receipt, err := requestledger.BeginQueryExecution(ctx)
	if err != nil {
		return nil, infraerrors.New(http.StatusServiceUnavailable, "REQUEST_LEDGER_UNAVAILABLE", "Request audit storage is temporarily unavailable")
	}
	if receipt != nil {
		ctx = requestledger.WithHandle(ctx, receipt)
		defer func() {
			if recovered := recover(); recovered != nil {
				if err := receipt.Finish(ctx, "failed", 0, "internal_error"); err != nil {
					slog.Error("request_ledger_credential_probe_finish_failed")
				}
				panic(recovered)
			}
			cause := probeErr
			if ctx.Err() != nil {
				cause = ctx.Err()
			}
			if err := receipt.Finish(ctx, requestledger.Outcome(0, cause), 0, requestledger.ErrorCode(cause)); err != nil {
				slog.Error("request_ledger_credential_probe_finish_failed")
			}
		}()
	}
	clientVersion := CodexCanonicalClientVersion()
	if s.settingService != nil {
		clientVersion = s.settingService.GetOpenAICodexClientVersion(ctx)
	}
	probeCtx := context.WithValue(ctx, openAICredentialProbeContextKey{}, true)
	response, err := s.fetchNativeCodexModelsManifest(probeCtx, account, clientVersion, "")
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, err
	}
	if response == nil || response.NotModified {
		return nil, invalidOpenAIModelsList(errors.New("credential inspection requires a current model response"))
	}
	body, err := standardOpenAIModelsBody(response.Body, true)
	if err != nil {
		return nil, invalidOpenAIModelsList(err)
	}
	return &OpenAIModelsResponse{Body: body, ETag: codexModelsManifestBodyETag(body)}, nil
}
