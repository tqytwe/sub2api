package service

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
)

const OpenAISIWCEnabledKey = "openai_siwc"

func (a *Account) IsSIWCEnabled() bool {
	return a != nil && a.Platform == PlatformOpenAI && a.Type == AccountTypeOAuth && !a.IsCredentialShadow() && !a.IsOpenAIPersonalAccessToken() && !a.IsOpenAIAgentIdentity() && a.Extra[OpenAISIWCEnabledKey] == true
}

func (a *Account) IsExcelOAuth() bool {
	return a != nil && a.Platform == PlatformOpenAI && a.Type == AccountTypeOAuth && a.GetCredential("client_id") == openai.ExcelClientID
}

// isOpenAICodexTicketAccount is the local compatibility predicate used by
// SIWC routing. The full Codex ticket harvester is intentionally not part of
// this credential-management change, so keep SIWC and Excel/BPS accounts out
// of the legacy ticket path when that package is absent from the fork.
func isOpenAICodexTicketAccount(account *Account, upstreamModels ...string) bool {
	if account == nil || account.IsSIWCEnabled() || !account.IsOpenAIOAuthLike() || account.IsShadow() || account.IsExcelBPSEnabled() {
		return false
	}
	// The fork does not carry the upstream model-catalog gate. A non-empty
	// model argument is therefore accepted for the legacy compatibility path.
	return len(upstreamModels) == 0 || strings.TrimSpace(upstreamModels[0]) != ""
}
