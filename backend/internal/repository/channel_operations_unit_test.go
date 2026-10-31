//go:build unit

package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestChannelOperationsCatalogReturnsEligibleAndRetiredMembership(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT operations_revision FROM channels WHERE id = $1`)).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"operations_revision"}).AddRow(int64(11)))
	mock.ExpectQuery(`SELECT count\(\*\) FROM channel_operation_upstreams`).
		WithArgs(int64(7), "").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))
	mock.ExpectQuery(`SELECT u\.id, u\.name, \(SELECT count\(\*\) FROM channel_operation_members`).
		WithArgs(int64(7), "", 20, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "members"}).AddRow(int64(9), "provider", int64(1)))
	mock.ExpectQuery(`SELECT count\(\*\).*channel_operation_members`).
		WithArgs(int64(7), "", "all", int64(0)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(2)))
	mock.ExpectQuery(`SELECT a\.id, a\.name, a\.platform, a\.status`).
		WithArgs(int64(7), "", "all", int64(0), 20, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "platform", "status", "schedulable", "concurrency", "upstream_id", "eligible"}).
			AddRow(int64(12), "active", "openai", "active", true, 3, int64(9), true).
			AddRow(int64(13), "retired", "openai", "disabled", false, 0, int64(9), false))
	mock.ExpectCommit()

	repo := &channelRepository{db: db}
	got, err := repo.ChannelOperationsCatalog(context.Background(), 7, service.ChannelOperationsQuery{
		Page: 1, PageSize: 20, UpstreamPage: 1, Membership: "all",
	})
	require.NoError(t, err)
	require.Equal(t, int64(11), got.Revision)
	require.Equal(t, int64(1), got.UpstreamTotal)
	require.Equal(t, []service.ChannelOperationUpstream{{ID: 9, Name: "provider", Members: 1}}, got.Upstreams)
	require.Equal(t, int64(2), got.Total)
	require.Equal(t, int64(12), got.Accounts[0].ID)
	require.True(t, got.Accounts[0].Eligible)
	require.Equal(t, int64(13), got.Accounts[1].ID)
	require.False(t, got.Accounts[1].Eligible)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReadChannelOperationsUsesPersistedQualityHistory(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	from := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
	until := from.Add(time.Hour)
	mock.ExpectQuery(`(?s)WITH base AS .*usage_logs u.*first_token_ms.*model_mapping_chain.*`+regexp.QuoteMeta(`'error_rate',NULL`)).
		WithArgs(from, until, int64(7), int64(0), int64(0), int64(0), "", "public", 20, 0).
		WillReturnRows(sqlmock.NewRows([]string{"jsonb_build_object"}).AddRow([]byte(`{"source":"usage_logs.channel_id / persisted first_token_ms","summary":{}}`)))
	mock.ExpectQuery(`(?s)WITH scoped AS .*scheduled_test_results.*quality_judgment.*round_integrity.*'has_more'`).
		WithArgs(from, until, int64(7), int64(0), int64(0), int64(0), "", int64(0), 20).
		WillReturnRows(sqlmock.NewRows([]string{"jsonb_build_object"}).AddRow([]byte(`{"source":"scheduled_test_results: frozen config / judgment / plan+round+model","summary":{"rounds":0}}`)))
	mock.ExpectQuery(`(?s)WITH scoped AS .*FROM ops_error_logs e.*is_count_tokens.*'error_rate',NULL`).
		WithArgs(from, until, int64(7), int64(0), int64(0), int64(0), "", "public").
		WillReturnRows(sqlmock.NewRows([]string{"jsonb_build_object"}).AddRow([]byte(`{"source":"ops_error_logs","error_rate":null}`)))
	mock.ExpectQuery(`(?s)SELECT jsonb_build_object\('source','accounts current configuration'.*configured_concurrency.*FROM accounts a`).
		WithArgs(int64(7), int64(0), int64(0), int64(0)).
		WillReturnRows(sqlmock.NewRows([]string{"jsonb_build_object"}).AddRow([]byte(`{"source":"accounts current configuration","configured_concurrency":0}`)))

	repo := &channelRepository{db: db}
	got := repo.ReadChannelOperations(context.Background(), service.ChannelOperationsFilter{
		ChannelID: 7, From: from, Until: until, ModelScope: "public", Dimension: "channel", Page: 1, PageSize: 20,
	})
	require.Equal(t, "ready", got.Usage.Status)
	require.Contains(t, string(got.Usage.Data), "first_token_ms")
	require.Equal(t, "ready", got.Quality.Status)
	require.Contains(t, string(got.Quality.Data), "scheduled_test_results")
	require.Equal(t, "ready", got.Errors.Status)
	require.Contains(t, string(got.Errors.Data), `"error_rate":null`)
	require.Equal(t, "ready", got.Capacity.Status)
	require.Contains(t, string(got.Capacity.Data), "configured_concurrency")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReadChannelOperationsRejectsUnsupportedQualityModelScope(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	from := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
	until := from.Add(time.Hour)
	mock.ExpectQuery(`(?s)WITH base AS .*usage_logs u.*first_token_ms.*model_mapping_chain.*`+regexp.QuoteMeta(`'error_rate',NULL`)).
		WithArgs(from, until, int64(7), int64(0), int64(0), int64(0), "public-model", "public", 20, 0).
		WillReturnRows(sqlmock.NewRows([]string{"jsonb_build_object"}).AddRow([]byte(`{"summary":{}}`)))
	mock.ExpectQuery(`(?s)WITH scoped AS .*FROM ops_error_logs e.*is_count_tokens.*'error_rate',NULL`).
		WithArgs(from, until, int64(7), int64(0), int64(0), int64(0), "public-model", "public").
		WillReturnRows(sqlmock.NewRows([]string{"jsonb_build_object"}).AddRow([]byte(`{"error_rate":null}`)))
	mock.ExpectQuery(`(?s)SELECT jsonb_build_object\('source','accounts current configuration'.*configured_concurrency.*FROM accounts a`).
		WithArgs(int64(7), int64(0), int64(0), int64(0)).
		WillReturnRows(sqlmock.NewRows([]string{"jsonb_build_object"}).AddRow([]byte(`{"configured_concurrency":0}`)))

	got := (&channelRepository{db: db}).ReadChannelOperations(context.Background(), service.ChannelOperationsFilter{
		ChannelID: 7, From: from, Until: until, Model: "public-model", ModelScope: "public", Dimension: "channel", Page: 1, PageSize: 20,
	})
	require.Equal(t, "unsupported_scope", got.Quality.Status)
	require.JSONEq(t, "null", string(got.Quality.Data))
	require.Contains(t, got.Quality.Error, "不能推断公开或已记录模型别名")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMutateChannelOperationsRejectsStaleRevisionBeforeMutation(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT operations_revision FROM channels WHERE id = $1 FOR UPDATE`)).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"operations_revision"}).AddRow(int64(4)))
	mock.ExpectRollback()

	revision := int64(3)
	repo := &channelRepository{db: db}
	_, err = repo.MutateChannelOperations(context.Background(), 7, service.ChannelOperationsMutation{
		Action: "create_upstream", Name: "provider", Revision: &revision,
	})
	require.ErrorIs(t, err, service.ErrChannelOperationsConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMutateChannelOperationsRejectsIneligibleAccount(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT operations_revision FROM channels WHERE id = $1 FOR UPDATE`)).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"operations_revision"}).AddRow(int64(4)))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM channel_operation_upstreams WHERE id = $1 AND channel_id = $2`)).
		WithArgs(int64(9), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(9)))
	mock.ExpectQuery(`SELECT \(a\.deleted_at IS NULL AND EXISTS`).
		WithArgs(int64(7), int64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"eligible"}).AddRow(false))
	mock.ExpectRollback()

	revision := int64(4)
	repo := &channelRepository{db: db}
	_, err = repo.MutateChannelOperations(context.Background(), 7, service.ChannelOperationsMutation{
		Action: "assign", Revision: &revision, UpstreamID: 9, AccountID: 12,
	})
	require.ErrorIs(t, err, service.ErrChannelOperationsValidation)
	require.NoError(t, mock.ExpectationsWereMet())
}
