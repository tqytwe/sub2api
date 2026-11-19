package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAnnouncementUserTargetValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		ids  []int64
	}{
		{name: "empty"},
		{name: "zero", ids: []int64{0}},
		{name: "negative", ids: []int64{-1}},
		{name: "duplicates", ids: []int64{2, 3, 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := AnnouncementTargeting{AnyOf: []AnnouncementConditionGroup{{AllOf: []AnnouncementCondition{{
				Type: "user", Operator: "in", UserIDs: tc.ids,
			}}}}}
			normalized, err := target.NormalizeAndValidate()
			if tc.name == "duplicates" {
				require.NoError(t, err)
				require.Equal(t, []int64{2, 3}, normalized.AnyOf[0].AllOf[0].UserIDs)
				return
			}
			require.ErrorIs(t, err, ErrAnnouncementInvalidTarget)
		})
	}
}
