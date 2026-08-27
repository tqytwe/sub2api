package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestResolveAgnesVideoStatusEndpointUsesExplicitVideoIDForOpaqueValues(t *testing.T) {
	endpoint, videoID := resolveAgnesVideoStatusEndpoint("opaque-task-id", true, "", "")
	require.Equal(t, service.AgnesVideoEndpointStatusVideo, endpoint)
	require.Equal(t, "opaque-task-id", videoID)
}

func TestResolveAgnesVideoStatusEndpointKeepsLegacyPathCompatibility(t *testing.T) {
	endpoint, videoID := resolveAgnesVideoStatusEndpoint("", false, "legacy-task", "")
	require.Equal(t, service.AgnesVideoEndpointStatusLegacy, endpoint)
	require.Equal(t, "legacy-task", videoID)

	endpoint, videoID = resolveAgnesVideoStatusEndpoint("", false, "video_123", "")
	require.Equal(t, service.AgnesVideoEndpointStatusVideo, endpoint)
	require.Equal(t, "video_123", videoID)
}

func TestResolveAgnesVideoStatusEndpointRejectsEmptyExplicitQueryAtVideoEndpoint(t *testing.T) {
	endpoint, videoID := resolveAgnesVideoStatusEndpoint("", true, "legacy-task", "")
	require.Equal(t, service.AgnesVideoEndpointStatusVideo, endpoint)
	require.Empty(t, videoID)
}
