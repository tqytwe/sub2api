//go:build unit

package service

import (
	"net/http"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestMarketplaceFeatureDisabledError_StableReason(t *testing.T) {
	syntheticAdapter := func(runtime MarketplaceRuntime) error {
		if !runtime.Enabled {
			return ErrMarketplaceFeatureDisabled
		}
		return nil
	}

	err := syntheticAdapter(MarketplaceRuntime{})

	require.Equal(t, http.StatusNotFound, infraerrors.Code(err))
	require.Equal(t, "MARKETPLACE_FEATURE_DISABLED", infraerrors.Reason(err))
}
