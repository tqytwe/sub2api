package service

import "github.com/Wei-Shaw/sub2api/internal/config"

// Keep historical compatibility overlays with an older Config buildable. An
// absent accessor preserves the default-disabled legacy behavior.
func openAILegacyCompatibilityEnabled(cfg *config.Config) bool {
	policy, ok := any(cfg).(interface{ OpenAILegacyCompatibilityEnabled() bool })
	return ok && policy.OpenAILegacyCompatibilityEnabled()
}

func (s *OpenAIGatewayService) openAILegacyCompatibility() bool {
	return s != nil && openAILegacyCompatibilityEnabled(s.cfg)
}
