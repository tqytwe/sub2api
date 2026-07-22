package service

import infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"

var ErrMarketplaceFeatureDisabled = infraerrors.NotFound(
	"MARKETPLACE_FEATURE_DISABLED",
	"marketplace feature is disabled",
)
