package service

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/requestledger"
)

func routingAccountID(account *Account) int64 {
	if account == nil {
		return 0
	}
	return account.ID
}
func recordOpenAIRoutingVeto(ctx context.Context, account *Account, reason string) {
	requestledger.RoutingVeto(ctx, "", routingAccountID(account), reason)
}

// The predicate has already executed at its original short-circuit position.
// This helper never evaluates policy or reads account/configuration storage.
func observeOpenAIRoutingVeto(ctx context.Context, account *Account, reason string, veto bool) bool {
	if veto {
		recordOpenAIRoutingVeto(ctx, account, reason)
	}
	return veto
}
func observeOpenAIRoutingGroupAfterCompatibility(ctx context.Context, account *Account, compatible, groupCompatible bool) bool {
	if compatible && !groupCompatible {
		recordOpenAIRoutingVeto(ctx, account, "group_mismatch")
	}
	return !groupCompatible
}
func (s *defaultOpenAIAccountScheduler) isAccountTransportCompatibleForRouting(ctx context.Context, account *Account, transport OpenAIUpstreamTransport) bool {
	compatible := s.isAccountTransportCompatible(account, transport)
	if !compatible {
		recordOpenAIRoutingVeto(ctx, account, "transport_incompatible")
	}
	return compatible
}

func routingQuotaReason(decision openAIQuotaAutoPauseDecision) string {
	if decision.reason != "" {
		return decision.reason
	}
	if decision.window != "" {
		return "quota_auto_pause_" + decision.window
	}
	return "quota_auto_pause"
}
