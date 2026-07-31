package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMaskReferralEmailPreservesDomainAndMasksLocalPart(t *testing.T) {
	require.Equal(t, "a***e@example.com", maskReferralEmail("alice@example.com"))
	require.Equal(t, "用*@example.com", maskReferralEmail("用户@example.com"))
	require.Equal(t, "***", maskReferralEmail("invalid"))
}
