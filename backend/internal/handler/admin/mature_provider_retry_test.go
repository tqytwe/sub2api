package admin

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMatureProviderRetryBoundsRateLimits(t *testing.T) {
	attempts := 0
	err := matureProviderRetry(context.Background(), func() error {
		attempts++
		return &providerRemoteError{operation: "wallet", status: http.StatusTooManyRequests}
	})
	require.Error(t, err)
	require.Equal(t, 3, attempts)
}

func TestMatureProviderRetryDoesNotRetryOtherFailures(t *testing.T) {
	attempts := 0
	err := matureProviderRetry(context.Background(), func() error {
		attempts++
		return &providerRemoteError{operation: "wallet", status: http.StatusBadGateway}
	})
	require.Error(t, err)
	require.Equal(t, 1, attempts)
}

func TestMatureProviderRetryHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	attempts := 0
	err := matureProviderRetry(ctx, func() error {
		attempts++
		return nil
	})
	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, attempts)
}
