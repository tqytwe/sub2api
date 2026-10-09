package handler

import "github.com/Wei-Shaw/sub2api/internal/service"

func shouldReportOpenAIWSTurnForwardResult(account *service.Account, result *service.OpenAIForwardResult, turnErr error) bool {
	if account != nil && account.IsOpenAI() && service.IsOpenAITurnAdmissionError(turnErr) {
		return false
	}
	// Keep the pre-existing successful/image reporting path. Retaining partial
	// text for settlement must not turn its transport failure into account success.
	return result != nil && (turnErr == nil || result.ImageCount > 0)
}

func shouldRecordOpenAIWSTurnUsage(account *service.Account, result *service.OpenAIForwardResult, turnErr error, cyberPolicyRecorded bool) bool {
	if result == nil {
		return false
	}
	if turnErr == nil {
		return true
	}
	// Errored cyber-policy turns are settled by recordCyberPolicyIfMarked.
	if cyberPolicyRecorded {
		return false
	}
	if account != nil && !account.IsOpenAI() {
		return result.ImageCount > 0
	}
	// Only upstream-observed aggregate usage proves text work to settle.
	// Output deltas, timing and token-detail subsets must not become estimates.
	return result.HasObservedUsage()
}
