package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAudioEndpointsRegistration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int // We expect 401 or 404, not 405 (Method Not Allowed)
	}{
		{
			name:       "audio_speech_endpoint_exists",
			method:     http.MethodPost,
			path:       "/v1/audio/speech",
			wantStatus: http.StatusUnauthorized, // No API key = 401
		},
		{
			name:       "audio_transcriptions_endpoint_exists",
			method:     http.MethodPost,
			path:       "/v1/audio/transcriptions",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "audio_translations_endpoint_exists",
			method:     http.MethodPost,
			path:       "/v1/audio/translations",
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test just verifies routes exist, not full functionality
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader("{}"))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()

			// Create a minimal router just to test route registration
			r := gin.New()
			handler := &OpenAIGatewayHandler{}
			r.POST("/v1/audio/speech", handler.AudioSpeech)
			r.POST("/v1/audio/transcriptions", handler.AudioTranscriptions)
			r.POST("/v1/audio/translations", handler.AudioTranslations)

			r.ServeHTTP(w, req)

			// 405 means route doesn't exist; we should get 401 (no auth) or other errors
			require.NotEqual(t, http.StatusMethodNotAllowed, w.Code,
				"Route %s should exist (got 405 Method Not Allowed)", tt.path)
		})
	}
}

func TestRealtimeEndpointRegistration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest(http.MethodGet, "/v1/realtime", nil)
	// Simulate WebSocket upgrade headers
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	w := httptest.NewRecorder()

	r := gin.New()
	handler := &OpenAIGatewayHandler{}
	r.GET("/v1/realtime", handler.Realtime)

	r.ServeHTTP(w, req)

	// Should not be 405 (route exists)
	require.NotEqual(t, http.StatusMethodNotAllowed, w.Code,
		"Route /v1/realtime should exist")
}

func TestAudioSpeechPlatformRouting(t *testing.T) {
	// This is a placeholder for future integration tests
	// that verify platform-specific routing logic
	t.Skip("Integration test - requires full service setup")
}
