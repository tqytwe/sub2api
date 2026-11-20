package service

import "context"

// PlatformAcceptsGeminiInboundBridge 报告该平台能否把 Gemini 原生入站
// （/v1beta/models）转成自己实际支持的上游协议。Gemini 与 Antigravity 走原生
// /v1beta/models，不经过这条桥。TypeSafe 只有 /v1/systemone，也不接受。
func PlatformAcceptsGeminiInboundBridge(platform string) bool {
	return UsesOpenAIGatewayForChat(platform) || platform == PlatformAnthropic
}

// UsesOpenAIGatewayForChat matches the platforms routed through the OpenAI
// Chat Completions handler, including when the incoming request is Gemini.
func UsesOpenAIGatewayForChat(platform string) bool {
	switch platform {
	case PlatformOpenAI, PlatformGrok, PlatformKimi, PlatformZhipu, PlatformDeepseek, PlatformMiniMax:
		return true
	default:
		return false
	}
}

// GeminiBridgeModelIDs 返回桥接分组可对外宣布的模型。账号目录缺失时回落平台默认列表。
func (s *GatewayService) GeminiBridgeModelIDs(ctx context.Context, groupID *int64, platform string) []string {
	if s == nil || s.accountRepo == nil {
		return nil
	}
	ids := s.GetAvailableModels(ctx, groupID, platform)
	if ids == nil {
		return DefaultModelIDsForPlatform(platform)
	}
	return ids
}
