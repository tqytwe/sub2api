//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type panickingModelAvailabilityRepo struct {
	AccountRepository
}

func (panickingModelAvailabilityRepo) ListModelAvailabilityCandidates(context.Context, *int64, []string, bool) ([]Account, error) {
	panic("typed-nil repository adapter")
}

func TestOpenAIGatewayServiceModelAvailabilityDiagnosticsFailOpenOnRepositoryPanic(t *testing.T) {
	groupID := int64(7)
	svc := NewOpenAIGatewayService(
		panickingModelAvailabilityRepo{}, nil, nil, nil, nil, nil, nil,
		&config.Config{RunMode: config.RunModeSimple}, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)

	got := svc.DiagnoseModelAvailabilityForPlatform(context.Background(), &groupID, "seedance", PlatformOpenAI)
	require.Equal(t, ModelAvailabilityDiagnosis{HasAccountsInPool: true, HasModelSupport: true}, got)
}
