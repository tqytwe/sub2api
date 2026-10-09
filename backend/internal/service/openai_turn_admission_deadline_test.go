//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type lateTurnAdmissionRepo struct {
	AccountRepository
	account      *Account
	cancelParent context.CancelFunc
}

func (r *lateTurnAdmissionRepo) GetOpenAITurnAdmission(ctx context.Context, _ int64) (*Account, *Account, error) {
	if r.cancelParent != nil {
		r.cancelParent()
	}
	<-ctx.Done()
	// A driver can deliver a valid row after its deadline without returning
	// the context error. Admission must independently reject that late row.
	return r.account, nil, nil
}

func TestOpenAITurnAdmissionInternalDeadlineRejectsLateSnapshot(t *testing.T) {
	for _, cancelParent := range []bool{false, true} {
		name := "internal deadline"
		if cancelParent {
			name = "parent cancellation keeps precedence"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			selected := turnAdmissionAccount()
			repo := &lateTurnAdmissionRepo{account: selected}
			if cancelParent {
				repo.cancelParent = cancel
			}
			svc := newTurnAdmissionGateway(repo, false)
			got, err := svc.AdmitOpenAITurn(ctx, nil, selected, "gpt-5.4")
			require.Nil(t, got)
			if cancelParent {
				require.ErrorIs(t, err, context.Canceled)
			} else {
				var denied *OpenAITurnAdmissionError
				require.ErrorAs(t, err, &denied)
				require.Equal(t, "latest_state_unavailable", denied.Reason)
			}
		})
	}
}
