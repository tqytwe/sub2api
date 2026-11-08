package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

type atomicAffiliateSettingsStub struct {
	AffiliateRepository
	calls int
	input AffiliateSettingsChange
}

func (r *atomicAffiliateSettingsStub) UpdateAffiliateSettings(_ context.Context, _ int64, input AffiliateSettingsChange) error {
	r.calls++
	r.input = input
	return nil
}

func TestAffiliateAtomicSettingsValidateAllFieldsBeforeWrite(t *testing.T) {
	r := &atomicAffiliateSettingsStub{}
	svc := NewAffiliateService(r, nil, nil, nil)
	code, badRate := "VIP2026", 1000.0
	err := svc.AdminUpdateSettings(context.Background(), 42, AffiliateSettingsChange{Code: &code, Rate: &badRate})
	require.Error(t, err)
	require.Zero(t, r.calls)
	rate := 20.0
	require.NoError(t, svc.AdminUpdateSettings(context.Background(), 42, AffiliateSettingsChange{Code: &code, Rate: &rate, ActorID: 3}))
	require.Equal(t, 1, r.calls)
	require.Equal(t, "VIP2026", *r.input.Code)
}

func TestOfflineWithdrawalEvidenceValidation(t *testing.T) {
	for _, input := range []AffiliateWithdrawalEvidence{
		{}, {ExternalReference: "ref", Note: "done"},
		{ExternalReference: "ref", PaidAt: "not-date", Note: "done"},
		{ExternalReference: "ref", PaidAt: "2999-01-01T00:00:00Z", Note: "done"},
	} {
		require.Error(t, input.Validate())
	}
	require.NoError(t, (AffiliateWithdrawalEvidence{ExternalReference: "bank-ref-1", PaidAt: "2026-01-01T00:00:00+08:00", Note: "Paid outside site"}).Validate())
}
