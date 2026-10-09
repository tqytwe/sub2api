package service

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"
)

const openAIAdmittedRefreshTimeout = 8 * time.Second

// An admitted snapshot has already crossed the primary-state boundary. The
// account-ID-only cache cannot prove it belongs to that credential generation.
// Keep ordinary provider caching unchanged and never publish this snapshot over
// a concurrent rotation. Refreshing still uses the shared locks and persistence.
func (p *OpenAITokenProvider) getAdmittedAccessToken(ctx context.Context, account *Account) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := validateOpenAIAdmittedCredentialState(account, account); err != nil {
		return "", err
	}
	expiresAt := account.GetCredentialAsTime("expires_at")
	needsRefresh := !account.IsOpenAIPersonalAccessToken() && (expiresAt == nil || time.Until(*expiresAt) <= openAITokenRefreshSkew)
	if !needsRefresh || strings.TrimSpace(account.GetOpenAIRefreshToken()) == "" {
		return admittedOpenAIAccessToken(account)
	}
	if p.refreshAPI == nil || p.executor == nil {
		// Legacy callers may have no refresh implementation. A still-valid
		// admitted token remains usable; an expired token must never escape.
		return admittedOpenAIAccessToken(account)
	}

	refreshCtx, cancel := context.WithTimeout(ctx, openAIAdmittedRefreshTimeout)
	defer cancel()
	executor := &admittedOpenAIRefreshExecutor{OAuthRefreshExecutor: p.executor, admitted: account}
	p.metrics.refreshRequests.Add(1)
	p.metrics.touchNow()
	result, err := p.refreshAPI.RefreshIfNeeded(withOAuthRefreshRequestPath(refreshCtx), account, executor, openAITokenRefreshSkew)
	if refreshCtx.Err() != nil {
		return "", denyOpenAITurn("credential_refresh_unavailable")
	}
	if executor.stateErr != nil {
		return "", executor.stateErr
	}
	if err != nil {
		p.metrics.refreshFailure.Add(1)
		// Only an attempted, revalidated credential can be the normal
		// near-expiry fallback. Read/lock/state failures have no such snapshot.
		if result != nil && result.Account != nil {
			if stateErr := validateOpenAIAdmittedCredentialState(account, result.Account); stateErr != nil {
				return "", stateErr
			}
			if p.refreshPolicy.OnRefreshError == ProviderRefreshErrorUseExistingToken && refreshCtx.Err() == nil {
				return admittedOpenAIAccessToken(result.Account)
			}
		}
		return "", denyOpenAITurn("credential_refresh_unavailable")
	}
	if result == nil {
		return "", denyOpenAITurn("credential_refresh_unavailable")
	}
	if result.LockHeld {
		p.metrics.lockContention.Add(1)
		p.metrics.touchNow()
		return p.waitForAdmittedCredentialRefresh(refreshCtx, account)
	}
	if stateErr := validateOpenAIAdmittedCredentialState(account, result.Account); stateErr != nil {
		return "", stateErr
	}
	if result.Refreshed {
		p.metrics.refreshSuccess.Add(1)
		// A rotation can revoke the old bearer. Invalidation cannot restore an
		// older value, unlike writing a request-local snapshot into this cache.
		if p.tokenCache != nil {
			if cacheErr := p.tokenCache.DeleteAccessToken(refreshCtx, OpenAITokenCacheKey(account)); cacheErr != nil {
				slog.Warn("openai_admitted_token_cache_invalidation_failed", "account_id", account.ID, "error", cacheErr)
			}
		}
	}
	if refreshCtx.Err() != nil {
		return "", denyOpenAITurn("credential_refresh_unavailable")
	}
	return admittedOpenAIAccessToken(result.Account)
}

// CanRefresh is called after the refresh API acquires its locks and rereads the
// durable row, before the executor can use a different/older refresh credential.
// Keep this guard request-local so other providers and background refreshes keep
// their existing refresh policy and lock protocol.
type admittedOpenAIRefreshExecutor struct {
	OAuthRefreshExecutor
	admitted *Account
	stateErr error
}

func (e *admittedOpenAIRefreshExecutor) CanRefresh(account *Account) bool {
	e.stateErr = validateOpenAIAdmittedCredentialState(e.admitted, account)
	return e.stateErr == nil && e.OAuthRefreshExecutor.CanRefresh(account)
}

func validateOpenAIAdmittedCredentialState(admitted, current *Account) error {
	if admitted == nil || current == nil || current.ID != admitted.ID || !current.IsOpenAIOAuth() || current.IsShadow() ||
		openAITurnRouteFingerprint(current) != openAITurnRouteFingerprint(admitted) {
		return denyOpenAITurn("credential_binding_changed")
	}
	eligible := current.IsSchedulable()
	if admitted.openAITurnCredentialForShadow {
		eligible = current.IsCredentialUsableForShadow()
	}
	if !eligible || (current.ProxyID != nil && current.Proxy == nil) {
		return denyOpenAITurn("credential_account_ineligible")
	}
	previousVersion := admitted.GetCredentialAsInt64("_token_version")
	currentVersion := current.GetCredentialAsInt64("_token_version")
	if currentVersion < previousVersion {
		return denyOpenAITurn("credential_generation_stale")
	}
	if currentVersion == previousVersion {
		// Equal versions cannot establish freshness for different secrets,
		// including imported credentials that have never been versioned.
		previous, previousErr := json.Marshal(admitted.Credentials)
		latest, latestErr := json.Marshal(current.Credentials)
		if previousErr != nil || latestErr != nil || !bytes.Equal(previous, latest) {
			return denyOpenAITurn("credential_generation_changed")
		}
	}
	return nil
}

func admittedOpenAIAccessToken(account *Account) (string, error) {
	if account == nil || strings.TrimSpace(account.GetOpenAIAccessToken()) == "" {
		return "", denyOpenAITurn("credential_token_unavailable")
	}
	expiresAt := account.GetCredentialAsTime("expires_at")
	// Imported/manual OAuth bearers may intentionally have no expiry metadata.
	// Keep that existing contract; only an explicit expired timestamp is fatal.
	if expiresAt != nil && !time.Now().Before(*expiresAt) {
		return "", denyOpenAITurn("credential_token_expired")
	}
	return account.GetOpenAIAccessToken(), nil
}

func (p *OpenAITokenProvider) waitForAdmittedCredentialRefresh(ctx context.Context, admitted *Account) (string, error) {
	if p.accountRepo == nil {
		return "", denyOpenAITurn("credential_state_unavailable")
	}
	waitCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	wait := openAILockInitialWait
	var latest *Account
	for attempt := 0; attempt < openAILockMaxAttempts; attempt++ {
		var err error
		latest, err = p.accountRepo.GetByID(waitCtx, admitted.ID)
		if waitCtx.Err() != nil {
			return "", denyOpenAITurn("credential_refresh_unavailable")
		}
		if err != nil {
			return "", denyOpenAITurn("credential_state_unavailable")
		}
		if err := validateOpenAIAdmittedCredentialState(admitted, latest); err != nil {
			return "", err
		}
		if latest.GetCredentialAsInt64("_token_version") > admitted.GetCredentialAsInt64("_token_version") {
			if token, tokenErr := admittedOpenAIAccessToken(latest); tokenErr == nil {
				p.metrics.lockWaitHit.Add(1)
				return token, nil
			}
		}
		if p.refreshPolicy.OnLockHeld != ProviderLockHeldWaitForCache || attempt == openAILockMaxAttempts-1 {
			break
		}
		actualWait := jitterLockWait(wait)
		timer := time.NewTimer(actualWait)
		select {
		case <-waitCtx.Done():
			timer.Stop()
			return "", denyOpenAITurn("credential_refresh_unavailable")
		case <-timer.C:
		}
		p.metrics.lockWaitSamples.Add(1)
		p.metrics.lockWaitTotalMs.Add(actualWait.Milliseconds())
		wait = min(wait*2, openAILockMaxWait)
	}
	p.metrics.lockWaitMiss.Add(1)
	// Retain the near-expiry policy only if the durable token is still valid.
	return admittedOpenAIAccessToken(latest)
}
