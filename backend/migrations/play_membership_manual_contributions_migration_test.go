package migrations

import (
	"strings"
	"testing"
)

func TestPlayMembershipManualContributionsMigrationContract(t *testing.T) {
	raw, err := FS.ReadFile("269_play_membership_manual_contributions.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(raw))
	for _, want := range []string{
		"create table if not exists play_membership_manual_contributions",
		"balance_transaction_id bigint not null unique references balance_transactions",
		"constraint uq_play_membership_manual_source_ref unique (source_type, external_ref)",
		"constraint chk_play_membership_manual_amounts check",
		"qualification_state in ('pending_review', 'verified', 'rejected')",
		"create or replace view play_membership_verified_contributions",
		"union all",
		"offline_recharge_backfill",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("migration missing contract %q", want)
		}
	}
	if strings.Contains(sql, "references payment_orders") {
		t.Error("manual contributions must not reference payment_orders")
	}
}
