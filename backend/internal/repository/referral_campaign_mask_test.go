package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMaskReferralEmailMasksBothLocalPartAndDomain(t *testing.T) {
	require.Equal(t, "al***@e***.com", maskReferralEmail("alice@example.com"))
	require.Equal(t, "用***@e***.com", maskReferralEmail("用户@example.com"))
	require.Equal(t, "", maskReferralEmail("invalid"))
}
