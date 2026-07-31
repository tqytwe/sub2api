package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMobileAttributionMigrationHasPrivacyAndIdempotencyGuards(t *testing.T) {
	raw, err := FS.ReadFile("237_mobile_attribution.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(raw))

	require.Contains(t, sql, "create table if not exists mobile_installations")
	require.Contains(t, sql, "create table if not exists mobile_attribution_events")
	require.Contains(t, sql, "idempotency_key_digest bytea")
	require.Contains(t, sql, "attribution_digest bytea")
	require.Contains(t, sql, "verified boolean not null default false")
	require.Contains(t, sql, "unique (idempotency_key_digest)")
	require.NotContains(t, sql, "attribution_token")
	require.NotContains(t, sql, "idempotency_key varchar")
	for _, eventType := range []string{"download", "click", "open", "register", "login", "active", "share"} {
		require.Contains(t, sql, "'"+eventType+"'")
	}
}

func TestMembershipContributionMigrationProtectsFinancialHistory(t *testing.T) {
	raw, err := FS.ReadFile("234_play_membership_accounting_and_campaign_audience.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(raw))

	require.Contains(t, sql, "references payment_orders(id) on delete restrict")
	require.NotContains(t, sql, "references payment_orders(id) on delete cascade")
}

func TestMobileAttributionKeepsReferralAndPlayCampaignForeignKeysSeparate(t *testing.T) {
	raw, err := FS.ReadFile("240_mobile_attribution_referral_campaign_fk.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(raw))

	require.Contains(t, sql, "referral_campaign_id bigint null references referral_campaigns(id)")
	require.Contains(t, sql, "mobile_installations(referral_campaign_id")
	require.Contains(t, sql, "mobile_attribution_events(referral_campaign_id")
	require.NotContains(t, sql, "referral_campaign_id bigint null references play_campaigns(id)")
}
