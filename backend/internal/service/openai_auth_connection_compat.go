package service

import "github.com/Wei-Shaw/sub2api/internal/pkg/openai"

const OpenAISIWCEnabledKey = "openai_siwc"

func (a *Account) IsSIWCEnabled() bool {
	return a != nil && a.Platform == PlatformOpenAI && a.Type == AccountTypeOAuth && !a.IsCredentialShadow() && !a.IsOpenAIPersonalAccessToken() && !a.IsOpenAIAgentIdentity() && a.Extra[OpenAISIWCEnabledKey] == true
}

func (a *Account) IsExcelOAuth() bool {
	return a != nil && a.Platform == PlatformOpenAI && a.Type == AccountTypeOAuth && a.GetCredential("client_id") == openai.ExcelClientID
}
