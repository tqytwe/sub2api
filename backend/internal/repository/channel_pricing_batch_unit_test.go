//go:build unit

package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestChannelPricingBatchMissingTargetRollsBackBeforeReplacing(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM channels WHERE id = ANY($1) ORDER BY id FOR UPDATE")).WithArgs("{8,7}").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
	mock.ExpectRollback()
	err = (&channelRepository{db: db}).BatchReplaceModelPricing(context.Background(), []service.ChannelPricingBatchItem{{ChannelID: 8, Pricing: []service.ChannelModelPricing{}}, {ChannelID: 7, Pricing: []service.ChannelModelPricing{}}})
	require.ErrorIs(t, err, service.ErrChannelNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestChannelPricingBatchRollsBackEarlierReplacementOnLaterFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM channels WHERE id = ANY($1) ORDER BY id FOR UPDATE")).WithArgs("{7,8}").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7).AddRow(8))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM channel_model_pricing WHERE channel_id = $1")).WithArgs(int64(7)).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM channel_model_pricing WHERE channel_id = $1")).WithArgs(int64(8)).WillReturnError(errors.New("replacement failed"))
	mock.ExpectRollback()
	err = (&channelRepository{db: db}).BatchReplaceModelPricing(context.Background(), []service.ChannelPricingBatchItem{{ChannelID: 7, Pricing: []service.ChannelModelPricing{}}, {ChannelID: 8, Pricing: []service.ChannelModelPricing{}}})
	require.ErrorContains(t, err, "replacement failed")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestChannelPricingBatchClearsPricesAndUpdatesChannelTimestamp(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM channels WHERE id = ANY($1) ORDER BY id FOR UPDATE")).WithArgs("{7}").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM channel_model_pricing WHERE channel_id = $1")).WithArgs(int64(7)).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE channels SET updated_at = NOW() WHERE id = ANY($1)")).WithArgs("{7}").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	err = (&channelRepository{db: db}).BatchReplaceModelPricing(context.Background(), []service.ChannelPricingBatchItem{{ChannelID: 7, Pricing: []service.ChannelModelPricing{}}})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
