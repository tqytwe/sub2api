//go:build unit

package handler

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func (r openAIImagesFailoverAccountRepo) GetOpenAITurnAdmission(ctx context.Context, id int64) (*service.Account, *service.Account, error) {
	return readHandlerTestOpenAITurnAdmission(ctx, r, id)
}

// Do not inherit the embedded reader: this fixture deliberately overrides
// GetByID to test a mismatch between the selected and resolved account.
func (r grokMediaSlotRepo) GetOpenAITurnAdmission(ctx context.Context, id int64) (*service.Account, *service.Account, error) {
	return readHandlerTestOpenAITurnAdmission(ctx, r, id)
}

var (
	_ service.OpenAITurnAdmissionReader = openAIImagesFailoverAccountRepo{}
	_ service.OpenAITurnAdmissionReader = grokMediaSlotRepo{}
)

func TestHandlerTurnAdmissionAdapterPreservesOverriddenLookup(t *testing.T) {
	repo := grokMediaSlotRepo{
		openAIImagesFailoverAccountRepo: openAIImagesFailoverAccountRepo{accounts: []service.Account{{ID: 1}, {ID: 2}}},
		mismatch:                        true,
	}
	account, parent, err := repo.GetOpenAITurnAdmission(t.Context(), 1)
	require.NoError(t, err)
	require.Equal(t, int64(2), account.ID)
	require.Nil(t, parent)
	require.Empty(t, account.Status)
	require.False(t, account.Schedulable)
}
