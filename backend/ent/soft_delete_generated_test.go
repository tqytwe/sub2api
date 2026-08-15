package ent_test

import (
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/stretchr/testify/require"
)

func TestSoftDeleteSchemasRegisterHooksAndInterceptors(t *testing.T) {
	client := dbent.NewClient()
	tests := []struct {
		name       string
		hookCount  func() int
		interCount func() int
	}{
		{"APIKey", func() int { return len(client.APIKey.Hooks()) }, func() int { return len(client.APIKey.Interceptors()) }},
		{"Account", func() int { return len(client.Account.Hooks()) }, func() int { return len(client.Account.Interceptors()) }},
		{"CompositeModelRoute", func() int { return len(client.CompositeModelRoute.Hooks()) }, func() int { return len(client.CompositeModelRoute.Interceptors()) }},
		{"Group", func() int { return len(client.Group.Hooks()) }, func() int { return len(client.Group.Interceptors()) }},
		{"MobileAsset", func() int { return len(client.MobileAsset.Hooks()) }, func() int { return len(client.MobileAsset.Interceptors()) }},
		{"Proxy", func() int { return len(client.Proxy.Hooks()) }, func() int { return len(client.Proxy.Interceptors()) }},
		{"User", func() int { return len(client.User.Hooks()) }, func() int { return len(client.User.Interceptors()) }},
		{"UserAttributeDefinition", func() int { return len(client.UserAttributeDefinition.Hooks()) }, func() int { return len(client.UserAttributeDefinition.Interceptors()) }},
		{"UserPlatformQuota", func() int { return len(client.UserPlatformQuota.Hooks()) }, func() int { return len(client.UserPlatformQuota.Interceptors()) }},
		{"UserSubscription", func() int { return len(client.UserSubscription.Hooks()) }, func() int { return len(client.UserSubscription.Interceptors()) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Positive(t, tt.hookCount(), "soft-delete hook is missing from generated client")
			require.Positive(t, tt.interCount(), "soft-delete query interceptor is missing from generated client")
		})
	}
}
