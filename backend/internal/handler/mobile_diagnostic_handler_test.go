package handler

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestNormalizeMobileDiagnosticKeepsOnlyPrivacySafeMetadata(t *testing.T) {
	now := time.Now().UTC()
	input := mobileDiagnosticInput{
		InstallationID: uuid.NewString(), Operation: "CHAT", Category: "network",
		Path: "/api/v1/nextchat/mobile/bootstrap", NetworkType: "WIFI", StatusCode: 503,
		DurationMS: 1500, RetryCount: 2, AppVersion: "2.0.36", OccurredAt: &now,
		Metadata: map[string]any{
			"native": true, "request_id": "request-1", "token": "secret",
			"prompt": "private prompt", "url": "https://example.com/private",
		},
	}

	require.True(t, normalizeMobileDiagnostic(&input))
	require.Equal(t, "chat", input.Operation)
	require.Equal(t, "wifi", input.NetworkType)
	require.Equal(t, map[string]any{"native": true}, input.Metadata)
}

func TestNormalizeMobileDiagnosticRejectsFullURLAndQuery(t *testing.T) {
	for _, path := range []string{
		"https://api.example.com/api/v1/user?token=secret",
		"/api/v1/user?access_token=secret",
	} {
		input := mobileDiagnosticInput{Operation: "sync", Category: "http", Path: path, NetworkType: "cellular"}
		require.False(t, normalizeMobileDiagnostic(&input), path)
	}
}
