package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
)

// OpenAITurnAdmissionReader reads an account, its routing/group state and, for
// a shadow, its credential parent from one primary-database snapshot. Admission
// must not fall back to the scheduler's eventually consistent cache.
// The reader supplies state only; callers are responsible for eligibility checks.
type OpenAITurnAdmissionReader interface {
	GetOpenAITurnAdmission(context.Context, int64) (*Account, *Account, error)
}

// OpenAITurnAdmissionError is a local pre-send rejection, never an upstream
// failure. It must not cause account/proxy health penalties or replay a turn.
type OpenAITurnAdmissionError struct{ Reason string }

func (e *OpenAITurnAdmissionError) Error() string { return "request admission denied: " + e.Reason }
func IsOpenAITurnAdmissionError(err error) bool {
	var denied *OpenAITurnAdmissionError
	return errors.As(err, &denied)
}
func denyOpenAITurn(reason string) error { return &OpenAITurnAdmissionError{Reason: reason} }

// AdmitOpenAITurn always requires authoritative OpenAI state, including on zero-value services.
// HTTP forwarding uses the same predicate with the production constructor's
// invariant; only legacy direct-struct unit fixtures retain supplied-state mode.
func (s *OpenAIGatewayService) AdmitOpenAITurn(ctx context.Context, c *gin.Context, selected *Account, model string) (*Account, error) {
	return s.admitOpenAITurnModels(ctx, c, selected, openAITurnRequestModel(ctx, model), model, model, true)
}

// Channel/composite routing can replace the body model before forwarding.
// Read the immutable original model for this fork's public group allowlist;
// model-specific account limits still use the actual outbound model separately.
func openAITurnRequestModel(ctx context.Context, fallback string) string {
	if model, ok := RequestedPublicModelFromContext(ctx); ok {
		return model
	}
	if ctx != nil {
		if model, ok := ctx.Value(ctxkey.Model).(string); ok && strings.TrimSpace(model) != "" {
			return strings.TrimSpace(model)
		}
	}
	return fallback
}

func (s *OpenAIGatewayService) admitOpenAITurnForRequest(ctx context.Context, c *gin.Context, selected *Account, requestModel, outboundModel string) (*Account, error) {
	return s.admitOpenAITurnModels(ctx, c, selected, requestModel, outboundModel, outboundModel, s == nil || s.requireLatestTurnAdmission)
}

func (s *OpenAIGatewayService) admitOpenAITurnModels(ctx context.Context, c *gin.Context, selected *Account, requestModel, routedModel, outboundModel string, requireLatest bool) (admittedAccount *Account, admissionErr error) {
	defer func() {
		if atForward, _ := ctx.Value(openAITurnAdmissionLogAtForwardBoundaryKey{}).(bool); !atForward {
			logOpenAITurnAdmissionDenial(ctx, selected, admissionErr)
		}
	}()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if selected == nil {
		return nil, denyOpenAITurn("account_unavailable")
	}
	if !selected.IsOpenAI() {
		// Source parity: compatible providers share the membership guard, but
		// retain their supplied snapshot rather than using the OpenAI DB reader.
		if c != nil && getAPIKeyFromContext(c) != nil {
			groupID := getOpenAIGroupIDFromContext(c)
			if !selected.IsModelAllowedInGroup(&groupID, outboundModel) {
				return nil, denyOpenAITurn("model_not_allowed_in_group")
			}
		}
		return selected, nil
	}
	latest := selected
	var parent *Account
	authoritative := false
	if s != nil && s.accountRepo != nil {
		if reader, ok := s.accountRepo.(OpenAITurnAdmissionReader); ok {
			readCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			var err error
			latest, parent, err = reader.GetOpenAITurnAdmission(readCtx, selected.ID)
			readContextErr := readCtx.Err()
			cancel()
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if readContextErr != nil || err != nil || latest == nil || latest.ID != selected.ID {
				return nil, denyOpenAITurn("latest_state_unavailable")
			}
			authoritative = true
		} else if requireLatest {
			return nil, denyOpenAITurn("latest_state_unavailable")
		}
	} else if requireLatest {
		return nil, denyOpenAITurn("latest_state_unavailable")
	}
	if latest.Platform != selected.Platform || latest.Type != selected.Type {
		return nil, denyOpenAITurn("account_binding_changed")
	}
	if authoritative {
		if !latest.IsSchedulable() {
			return nil, denyOpenAITurn("account_ineligible")
		}
		if latest.ProxyID != nil && latest.Proxy == nil {
			return nil, denyOpenAITurn("proxy_unavailable")
		}
		if latest.IsShadow() && (parent == nil || parent.ID != *latest.ParentAccountID || parent.IsShadow() || !parent.IsOpenAIOAuth() || !parent.IsCredentialUsableForShadow() || (parent.ProxyID != nil && parent.Proxy == nil)) {
			return nil, denyOpenAITurn("credential_parent_ineligible")
		}
	}
	if authoritative {
		if err := validateOpenAITurnCredentialParentBinding(selected, parent); err != nil {
			return nil, err
		}
	}
	// The key's current routed group is the scheduling scope. Never replace its
	// auth-time parent billing group or the frozen request pricing timestamp.
	hasGroupContext := c != nil && getAPIKeyFromContext(c) != nil
	enforceGroup := hasGroupContext && (s == nil || s.cfg == nil || s.cfg.RunMode != config.RunModeSimple)
	if enforceGroup {
		groupID := getOpenAIGroupIDFromContext(c)
		if (groupID != 0 && !slices.Contains(latest.GroupIDs, groupID)) || (groupID == 0 && len(latest.GroupIDs) != 0) {
			return nil, denyOpenAITurn("group_membership_changed")
		}
		if authoritative && groupID != 0 {
			var group *Group
			for _, candidate := range latest.Groups {
				if candidate != nil && candidate.ID == groupID {
					group = candidate
					break
				}
			}
			if group == nil || !group.IsActive() {
				return nil, denyOpenAITurn("group_unavailable")
			}
			// This fork's group-wide allowlist is expressed in client/public model IDs,
			// unlike the upstream-mapped per-membership policy introduced separately.
			if !group.ModelAllowlist.Allows(requestModel) {
				return nil, denyOpenAITurn("model_not_allowed_in_group")
			}
		}
	}
	// Simple mode skips group binding, but the source still applies a present
	// membership model restriction. This uses the outbound name, independently
	// of the immutable public name used by the group's allowlist above.
	if hasGroupContext {
		groupID := getOpenAIGroupIDFromContext(c)
		// The initial HTTP check still protects the routed alias independently
		// of the mapped target. Only model cooldowns use the outbound name alone.
		if !latest.IsModelAllowedInGroup(&groupID, routedModel) ||
			(routedModel != outboundModel && !latest.IsModelAllowedInGroup(&groupID, outboundModel)) {
			return nil, denyOpenAITurn("model_not_allowed_in_group")
		}
	}
	if openAITurnRouteFingerprint(latest) != openAITurnRouteFingerprint(selected) {
		return nil, denyOpenAITurn("account_binding_changed")
	}
	if !s.openAIResponsesToolsProtocolCompatible(ctx, latest, routedModel, false) {
		return nil, denyOpenAITurn(openAIResponsesToolsProtocolMismatch)
	}
	if s != nil {
		if raw, ok := s.openaiAccountRuntimeBlockUntil.Load(latest.ID); ok {
			if until, valid := raw.(time.Time); valid && time.Now().Before(until) {
				return nil, denyOpenAITurn("account_runtime_blocked")
			}
		}
		if s.getOpenAIAccountModelTransientState().isBlocked(latest.ID, openAIAccountModelTransientModel(outboundModel), time.Now()) {
			return nil, denyOpenAITurn("model_runtime_blocked")
		}
	}
	if openAIAccountOutboundModelRateLimited(ctx, latest, requestModel, outboundModel) {
		return nil, denyOpenAITurn("model_rate_limited")
	}
	if authoritative {
		admitted := bindOpenAITurnCredentialParent(latest, parent)
		if admitted == nil {
			return nil, denyOpenAITurn("credential_snapshot_unavailable")
		}
		return admitted, nil
	}
	return latest, nil
}

// Routing identity deliberately excludes refreshable authentication secrets.
// The digest is only compared in memory and is never logged.
func openAITurnRouteFingerprint(a *Account) [32]byte {
	if a == nil {
		return [32]byte{}
	}
	routeExtra := make(map[string]any)
	for _, key := range []string{
		codexFingerprintSeedExtraKey, codexFingerprintModeExtraKey,
		"openai_passthrough", "openai_oauth_passthrough", "openai_excel_bps", "openai_excel_bps_models", "openai_excel_bps_mihomo",
		"openai_prism_browser", "openai_prism_browser_models",
		"openai_oauth_responses_websockets_v2_mode", "openai_apikey_responses_websockets_v2_mode",
		"openai_oauth_responses_websockets_v2_enabled", "openai_apikey_responses_websockets_v2_enabled",
		"responses_websockets_v2_enabled", "openai_ws_enabled", "openai_ws_force_http",
		"openai_compact_mode", "openai_compact_supported",
		"openai_responses_mode", "openai_responses_supported",
		"openai_responses_flatten_namespaces", "enable_tls_fingerprint", "tls_fingerprint_profile_id",
		"codex_cli_only", "codex_cli_only_allow_app_server",
	} {
		if value, ok := a.Extra[key]; ok {
			routeExtra[key] = value
		}
	}
	proxyURL := ""
	if a.Proxy != nil {
		proxyURL = a.Proxy.URL()
	}
	routeCredentials := make(map[string]any)
	for key, value := range a.Credentials {
		switch key {
		case "access_token", "refresh_token", "id_token", "_token_version",
			"expires_at", "expires_in", "token_type", "scope":
			continue
		}
		if IsSensitiveCredentialKey(key) || strings.HasPrefix(key, "_token_") {
			continue
		}
		routeCredentials[key] = value
	}
	b, _ := json.Marshal(struct {
		Platform, Type string
		Parent, Proxy  *int64
		Credentials    map[string]any
		RouteExtra     map[string]any
		ProxyURL       string
	}{a.Platform, a.Type, a.ParentAccountID, a.ProxyID, routeCredentials, routeExtra, proxyURL})
	return sha256.Sum256(b)
}
