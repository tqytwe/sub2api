package service

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var balanceLedgerDirectWritePatterns = []*regexp.Regexp{
	regexp.MustCompile(`\.(SetBalance|AddBalance|SetFrozenBalance|AddFrozenBalance)\s*\(`),
	regexp.MustCompile(`\bSET\s+balance\s*=`),
	regexp.MustCompile(`\bbalance\s*=\s*balance\b`),
	regexp.MustCompile(`\bfrozen_balance\s*=`),
	regexp.MustCompile(`\.(UpdateBalance|DeductBalance|ApplyRedeemBalanceAdjustment)\s*\(`),
}

var balanceLedgerAllowedDirectWriteFiles = map[string]string{
	"repository/affiliate_repo.go":     "affiliate quota transfer keeps a nil-ledger legacy fallback",
	"repository/play_repo_extended.go": "play balance grant fallback remains for nil-ledger tests and compatibility",
	"repository/usage_billing_repo.go": "ledger-aware production path migrated; direct SQL remains only as nil-ledger legacy fallback",
	"repository/user_repo.go":          "legacy user balance repository methods and create/update snapshots remain",
	"service/admin_user.go":            "admin balance adjustment keeps a nil-ledger legacy fallback",
	"service/auth_oauth_first_bind.go": "OAuth first-bind default grant keeps a nil-ledger legacy fallback",
	"service/balance_ledger.go":        "the unified ledger service is the only intended direct user-balance writer",
	"service/gateway_usage_billing.go": "gateway post-usage direct deduction remains only as nil-repository degraded fallback",
	"service/payment_refund.go":        "refund deduction and rollback keep nil-ledger legacy fallbacks",
	"service/promo_service.go":         "promo service keeps a nil-ledger legacy fallback",
	"service/redeem_service.go":        "redeem service keeps a nil-ledger legacy fallback",
	"service/usage_service.go":         "legacy usage service charge keeps a nil-ledger fallback",
	"service/user_service.go":          "legacy public balance wrapper remains while callers migrate",
}

func TestBalanceLedgerDirectWritersStayAudited(t *testing.T) {
	t.Parallel()

	internalRoot := filepath.Clean("..")
	var unexpected []string
	err := filepath.WalkDir(internalRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		rel, err := filepath.Rel(internalRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !containsBalanceLedgerDirectWrite(string(data)) {
			return nil
		}
		if _, ok := balanceLedgerAllowedDirectWriteFiles[rel]; !ok {
			unexpected = append(unexpected, rel)
		}
		return nil
	})
	require.NoError(t, err)
	require.Empty(t, unexpected, "new direct users.balance/frozen_balance writers must use BalanceLedgerService.ApplyDelta or be explicitly audited")
}

func TestBalanceLedgerDirectWriteAllowlistMentionsExistingFiles(t *testing.T) {
	t.Parallel()

	internalRoot := filepath.Clean("..")
	for rel := range balanceLedgerAllowedDirectWriteFiles {
		_, err := os.Stat(filepath.Join(internalRoot, filepath.FromSlash(rel)))
		require.NoError(t, err, "remove stale direct-write allowlist entry %s", rel)
	}
}

func containsBalanceLedgerDirectWrite(text string) bool {
	for _, pattern := range balanceLedgerDirectWritePatterns {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}
