package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type affiliateTransferLedgerRepoStub struct {
	AffiliateRepository
	ledgerUsed   bool
	fallbackUsed bool
}

func (r *affiliateTransferLedgerRepoStub) TransferQuotaToBalanceWithLedger(_ context.Context, userID int64, ledger BalanceLedgerApplier) (float64, float64, error) {
	r.ledgerUsed = true
	if userID <= 0 || ledger == nil {
		return 0, 0, ErrAffiliateQuotaEmpty
	}
	return 1.25, 4.5, nil
}

func (r *affiliateTransferLedgerRepoStub) TransferQuotaToBalance(context.Context, int64) (float64, float64, error) {
	r.fallbackUsed = true
	return 0.75, 4.0, nil
}

func TestAffiliateServiceTransferQuotaUsesBalanceLedgerRepositoryWhenAvailable(t *testing.T) {
	t.Parallel()

	repo := &affiliateTransferLedgerRepoStub{}
	svc := NewAffiliateService(repo, nil, nil, nil, &BalanceLedgerService{})

	transferred, balance, err := svc.TransferAffiliateQuota(context.Background(), 42)
	require.NoError(t, err)
	require.Equal(t, 1.25, transferred)
	require.Equal(t, 4.5, balance)
	require.True(t, repo.ledgerUsed)
	require.False(t, repo.fallbackUsed)
}

func TestAffiliateServiceTransferQuotaFallsBackWithoutBalanceLedger(t *testing.T) {
	t.Parallel()

	repo := &affiliateTransferLedgerRepoStub{}
	svc := NewAffiliateService(repo, nil, nil, nil)

	transferred, balance, err := svc.TransferAffiliateQuota(context.Background(), 42)
	require.NoError(t, err)
	require.Equal(t, 0.75, transferred)
	require.Equal(t, 4.0, balance)
	require.False(t, repo.ledgerUsed)
	require.True(t, repo.fallbackUsed)
}
