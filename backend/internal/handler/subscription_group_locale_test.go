package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestLocalizeSubscriptionProgressUsesRequestLocaleWithoutLeakingTranslations(t *testing.T) {
	progress := &service.SubscriptionProgress{
		GroupName: "标准",
		I18n:      service.GroupI18n{"en": {Name: "Standard"}},
	}

	got := localizeSubscriptionProgress(progress, "en-US,en;q=0.9")
	require.Same(t, progress, got)
	require.Equal(t, "Standard", got.GroupName)
	require.Nil(t, got.I18n)
}
