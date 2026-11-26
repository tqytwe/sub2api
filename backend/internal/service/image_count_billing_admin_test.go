//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdminGroupImageCountBillingCreateAndUpdate(t *testing.T) {
	t.Run("create enables only when image generation is enabled on supported platforms", func(t *testing.T) {
		for _, tc := range []struct {
			platform string
			allow    bool
			want     bool
		}{
			{platform: PlatformOpenAI, allow: true, want: true},
			{platform: PlatformComposite, allow: true, want: true},
			{platform: PlatformOpenAI, allow: false},
			{platform: PlatformGemini, allow: true},
		} {
			repo := &groupRepoStubForAdmin{createID: 77}
			svc := &adminServiceImpl{groupRepo: repo}
			_, err := svc.CreateGroup(context.Background(), &CreateGroupInput{
				Name: "images", Platform: tc.platform, RateMultiplier: 1,
				SubscriptionType:     SubscriptionTypeStandard,
				AllowImageGeneration: tc.allow, ImageCountBilling: true,
			})
			require.NoError(t, err)
			require.Equal(t, tc.want, repo.created.ImageCountBilling, tc.platform)
		}
	})

	t.Run("update disables switch when image generation is turned off", func(t *testing.T) {
		existing := &Group{ID: 9, Name: "images", Platform: PlatformComposite, AllowImageGeneration: true, ImageCountBilling: true}
		repo := &groupRepoStubForAdmin{getByID: existing}
		svc := &adminServiceImpl{groupRepo: repo}
		allow := false
		updated, err := svc.UpdateGroup(context.Background(), existing.ID, &UpdateGroupInput{AllowImageGeneration: &allow})
		require.NoError(t, err)
		require.False(t, updated.ImageCountBilling)
		require.False(t, repo.updated.ImageCountBilling)
	})
}
