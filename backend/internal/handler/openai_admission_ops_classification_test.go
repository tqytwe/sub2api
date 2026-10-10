package handler

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIAdmissionOpsClassificationIsPreserved(t *testing.T) {
	errType := normalizeOpsErrorType("admission_unavailable", "")
	require.Equal(t, "api_error", errType)
	require.Equal(t, "internal", classifyOpsPhase(errType, "Account eligibility changed; please retry with complete context", ""))
}
