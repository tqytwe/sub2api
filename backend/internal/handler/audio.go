package handler

import (
	"net/http"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// AudioSpeech handles /v1/audio/speech (TTS - Text to Speech)
// Converts text to spoken audio using the provider's TTS models.
func (h *OpenAIGatewayHandler) AudioSpeech(c *gin.Context) {
	if c == nil {
		return
	}
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey.Group == nil {
		h.errorResponse(c, http.StatusUnauthorized, "invalid_request_error", "API key is required")
		return
	}

	// Route to platform-specific implementation
	platform := apiKey.Group.Platform
	switch platform {
	case service.PlatformGrok:
		// Forward to Grok TTS endpoint
		h.GrokVoice(c, "tts")
	case service.PlatformOpenAI:
		h.openAIAudio(c, service.OpenAIAudioOperationSpeech)
	default:
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Audio speech API is not supported for this platform")
	}
}

// AudioTranscriptions handles /v1/audio/transcriptions (STT - Speech to Text)
// Transcribes audio into the input language using Whisper or equivalent models.
func (h *OpenAIGatewayHandler) AudioTranscriptions(c *gin.Context) {
	if c == nil {
		return
	}
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey.Group == nil {
		h.errorResponse(c, http.StatusUnauthorized, "invalid_request_error", "API key is required")
		return
	}

	platform := apiKey.Group.Platform
	switch platform {
	case service.PlatformGrok:
		// Forward to Grok STT endpoint
		h.GrokVoice(c, "stt")
	case service.PlatformOpenAI:
		h.openAIAudio(c, service.OpenAIAudioOperationTranscriptions)
	default:
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Audio transcriptions API is not supported for this platform")
	}
}

// AudioTranslations handles /v1/audio/translations
// Transcribes audio into English regardless of the input language.
func (h *OpenAIGatewayHandler) AudioTranslations(c *gin.Context) {
	if c == nil {
		return
	}
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey.Group == nil {
		h.errorResponse(c, http.StatusUnauthorized, "invalid_request_error", "API key is required")
		return
	}

	platform := apiKey.Group.Platform
	switch platform {
	case service.PlatformGrok:
		// Grok may not support translations endpoint, return not found
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Audio translations API is not supported for Grok platform")
	case service.PlatformOpenAI:
		h.openAIAudio(c, service.OpenAIAudioOperationTranslations)
	default:
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Audio translations API is not supported for this platform")
	}
}

// Realtime handles /v1/realtime WebSocket connections (OpenAI Realtime API compatible)
// Provides a standard WebSocket endpoint for real-time audio streaming.
func (h *OpenAIGatewayHandler) Realtime(c *gin.Context) {
	if c == nil || c.Request == nil {
		return
	}
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey.Group == nil {
		h.errorResponse(c, http.StatusUnauthorized, "invalid_request_error", "API key is required")
		return
	}

	// Check if this is a WebSocket upgrade request
	if !isOpenAIWSUpgradeRequest(c.Request) {
		h.errorResponse(c, http.StatusUpgradeRequired, "invalid_request_error", "WebSocket upgrade required (Upgrade: websocket)")
		return
	}

	platform := apiKey.Group.Platform
	switch platform {
	case service.PlatformGrok:
		// Forward to Grok Realtime WebSocket
		h.GrokRealtime(c)
	case service.PlatformOpenAI:
		h.openAIRealtime(c)
	default:
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Realtime API is not supported for this platform")
	}
}
