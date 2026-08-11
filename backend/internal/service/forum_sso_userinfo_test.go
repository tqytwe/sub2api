//go:build unit

package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// Fix #7: userinfo language is a deployment-wide config, not a hardcoded value.
// The platform has no per-user locale, so an operator running a non-Chinese
// community must be able to set it without recompiling, while an unset value
// keeps the historical "zh-CN" behaviour byte-for-byte.

func TestForumSSOUserLanguageDefaultsToZhCN(t *testing.T) {
	t.Parallel()

	// Unset config → the pre-config hardcoded default.
	svc := &ForumSSOService{cfg: &config.Config{}}
	require.Equal(t, "zh-CN", svc.userLanguage())
}

func TestForumSSOUserLanguageHonorsConfig(t *testing.T) {
	t.Parallel()

	svc := &ForumSSOService{cfg: &config.Config{ForumSSO: config.ForumSSOConfig{UserLanguage: "en-US"}}}
	require.Equal(t, "en-US", svc.userLanguage())
}

func TestForumSSOUserLanguageBlankFallsBack(t *testing.T) {
	t.Parallel()

	// Whitespace-only is treated as unset so a stray space in an env var cannot
	// ship an empty locale to the forum.
	svc := &ForumSSOService{cfg: &config.Config{ForumSSO: config.ForumSSOConfig{UserLanguage: "   "}}}
	require.Equal(t, "zh-CN", svc.userLanguage())
}

func TestForumSSOUserLanguageNilServiceIsSafe(t *testing.T) {
	t.Parallel()

	// conf() guards a nil service / nil cfg; userLanguage must inherit that.
	var svc *ForumSSOService
	require.NotPanics(t, func() {
		require.Equal(t, "zh-CN", svc.userLanguage())
	})
}
