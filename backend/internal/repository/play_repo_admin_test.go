package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestCountTeamRewardSettlementsNeedingAttentionUsesSettlementTable(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := &playRepository{sql: db}
	mock.ExpectQuery(`(?is)FROM play_team_settlements\s+WHERE status IN \('pending', 'processing', 'partial', 'failed'\)`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))

	got, err := repo.CountTeamRewardSettlementsNeedingAttention(context.Background())

	require.NoError(t, err)
	require.Equal(t, 3, got)
	require.NoError(t, mock.ExpectationsWereMet())
}
