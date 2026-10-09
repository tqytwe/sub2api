package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpenAITurnAdmissionReadTransaction(t *testing.T) {
	for _, failure := range []string{"", "begin", "account", "membership", "commit"} {
		t.Run(failure, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()
			client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
			defer func() { _ = client.Close() }()
			repo := &accountRepository{client: client}
			begin := mock.ExpectBegin()
			if failure == "begin" {
				begin.WillReturnError(errors.New("begin failed"))
			} else {
				query := mock.ExpectQuery(`SELECT .* FROM "accounts"`)
				if failure == "account" {
					query.WillReturnError(errors.New("account read failed"))
					mock.ExpectRollback()
				} else {
					query.WillReturnRows(sqlmock.NewRows([]string{"id", "platform", "type", "status", "schedulable"}).AddRow(7, "openai", "oauth", "active", true))
					groups := mock.ExpectQuery(`SELECT .* FROM "account_groups"`)
					if failure == "membership" {
						groups.WillReturnError(errors.New("membership read failed"))
						mock.ExpectRollback()
					} else {
						groups.WillReturnRows(sqlmock.NewRows([]string{"account_id", "group_id"}))
						commit := mock.ExpectCommit()
						if failure == "commit" {
							commit.WillReturnError(errors.New("commit failed"))
						}
					}
				}
			}
			account, parent, err := repo.GetOpenAITurnAdmission(context.Background(), 7)
			if failure == "" {
				require.NoError(t, err)
				require.EqualValues(t, 7, account.ID)
				require.True(t, account.IsSchedulable())
			} else {
				require.Error(t, err)
				require.Nil(t, account, "never return an incomplete or cached snapshot")
			}
			require.Nil(t, parent)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestOpenAITurnAdmissionPreservesTargetGroupRestrictions(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	defer func() { _ = client.Close() }()
	repo := &accountRepository{client: client}
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "accounts"`).WillReturnRows(sqlmock.NewRows([]string{"id", "platform", "type", "status", "schedulable"}).AddRow(7, "openai", "oauth", "active", true))
	mock.ExpectQuery(`SELECT .* FROM "account_groups"`).WillReturnRows(sqlmock.NewRows([]string{"account_id", "group_id", "priority"}).AddRow(7, 9, 3))
	mock.ExpectQuery(`SELECT .* FROM "groups"`).WillReturnRows(sqlmock.NewRows([]string{"id", "status", "model_allowlist"}).AddRow(9, "active", []byte(`{"enabled":true,"models":["gpt-6-astra"]}`)))
	mock.ExpectCommit()
	account, parent, err := repo.GetOpenAITurnAdmission(context.Background(), 7)
	require.NoError(t, err)
	require.Nil(t, parent)
	require.Equal(t, []int64{9}, account.GroupIDs)
	require.Len(t, account.Groups, 1)
	require.True(t, account.Groups[0].ModelAllowlist.Enabled)
	require.Equal(t, []string{"gpt-6-astra"}, account.Groups[0].ModelAllowlist.Models)
	require.Len(t, account.AccountGroups, 1)
	require.Same(t, account.Groups[0], account.AccountGroups[0].Group)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOpenAITurnAdmissionShadowParentSnapshot(t *testing.T) {
	for _, failure := range []string{"", "parent", "parent membership", "proxy", "group"} {
		t.Run(failure, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()
			client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
			defer func() { _ = client.Close() }()
			repo := &accountRepository{client: client}
			mock.ExpectBegin()
			mock.ExpectQuery(`SELECT .* FROM "accounts"`).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"id", "platform", "type", "parent_account_id"}).AddRow(7, "openai", "oauth", 8))
			mock.ExpectQuery(`SELECT .* FROM "account_groups"`).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"account_id", "group_id"}))
			parentQuery := mock.ExpectQuery(`SELECT .* FROM "accounts"`).WithArgs(int64(8))
			if failure == "parent" {
				parentQuery.WillReturnError(errors.New("parent unavailable"))
				mock.ExpectRollback()
			} else {
				parentQuery.WillReturnRows(sqlmock.NewRows([]string{"id", "platform", "type", "proxy_id"}).AddRow(8, "openai", "oauth", 11))
				proxyQuery := mock.ExpectQuery(`SELECT .* FROM "proxies"`).WithArgs(int64(11))
				if failure == "proxy" {
					proxyQuery.WillReturnError(errors.New("proxy unavailable"))
					mock.ExpectRollback()
				} else {
					proxyQuery.WillReturnRows(sqlmock.NewRows([]string{"id", "host", "port", "protocol"}).AddRow(11, "proxy.example", 8080, "http"))
					membership := mock.ExpectQuery(`SELECT .* FROM "account_groups"`).WithArgs(int64(8))
					if failure == "parent membership" {
						membership.WillReturnError(errors.New("parent membership unavailable"))
						mock.ExpectRollback()
					} else {
						membership.WillReturnRows(sqlmock.NewRows([]string{"account_id", "group_id"}).AddRow(8, 9))
						group := mock.ExpectQuery(`SELECT .* FROM "groups"`).WithArgs(int64(9))
						if failure == "group" {
							group.WillReturnError(errors.New("group unavailable"))
							mock.ExpectRollback()
						} else {
							group.WillReturnRows(sqlmock.NewRows([]string{"id", "status"}).AddRow(9, "active"))
							mock.ExpectCommit()
						}
					}
				}
			}
			account, parent, err := repo.GetOpenAITurnAdmission(context.Background(), 7)
			if failure != "" {
				require.Error(t, err)
				require.Nil(t, account)
				require.Nil(t, parent)
			} else {
				require.NoError(t, err)
				require.EqualValues(t, 7, account.ID)
				require.EqualValues(t, 8, parent.ID)
				require.NotNil(t, parent.Proxy)
				require.Equal(t, "proxy.example", parent.Proxy.Host)
				require.Equal(t, []int64{9}, parent.GroupIDs)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// Observe the actual Ent transaction options; sqlmock alone does not inspect them.
type admissionTransactionDriver struct {
	*entsql.Driver
	options *sql.TxOptions
}

func (d *admissionTransactionDriver) BeginTx(ctx context.Context, options *sql.TxOptions) (dialect.Tx, error) {
	copy := *options
	d.options = &copy
	return d.Driver.BeginTx(ctx, options)
}
func TestOpenAITurnAdmissionUsesReadOnlyRepeatableRead(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	driver := &admissionTransactionDriver{Driver: entsql.OpenDB(dialect.Postgres, db)}
	client := dbent.NewClient(dbent.Driver(driver))
	defer func() { _ = client.Close() }()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "accounts"`).WillReturnError(errors.New("stop snapshot"))
	mock.ExpectRollback()
	_, _, err = (&accountRepository{client: client}).GetOpenAITurnAdmission(context.Background(), 7)
	require.Error(t, err)
	require.Equal(t, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead}, driver.options)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOpenAITurnAdmissionMissingAccountAndCancellation(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing account", true: "cancelled"}[cancelled], func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()
			client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
			defer func() { _ = client.Close() }()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if cancelled {
				cancel()
			} else {
				mock.ExpectBegin()
				// The existing soft-delete interceptor must remain active in the transaction.
				mock.ExpectQuery(`SELECT .* FROM "accounts" WHERE .*"deleted_at" IS NULL`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
				mock.ExpectRollback()
			}
			account, parent, err := (&accountRepository{client: client}).GetOpenAITurnAdmission(ctx, 7)
			if cancelled {
				require.ErrorIs(t, err, context.Canceled)
			} else {
				require.ErrorIs(t, err, service.ErrAccountNotFound)
			}
			require.Nil(t, account)
			require.Nil(t, parent)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestOpenAITurnAdmissionRejectsUnresolvedConfiguredProxy(t *testing.T) {
	for _, shadow := range []bool{false, true} {
		t.Run(map[bool]string{false: "selected account", true: "credential parent"}[shadow], func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()
			client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
			defer func() { _ = client.Close() }()
			mock.ExpectBegin()
			id := int64(7)
			if shadow {
				mock.ExpectQuery(`SELECT .* FROM "accounts"`).WithArgs(id).WillReturnRows(sqlmock.NewRows([]string{"id", "parent_account_id"}).AddRow(7, 8))
				mock.ExpectQuery(`SELECT .* FROM "account_groups"`).WithArgs(id).WillReturnRows(sqlmock.NewRows([]string{"account_id", "group_id"}))
				id = 8
			}
			mock.ExpectQuery(`SELECT .* FROM "accounts"`).WithArgs(id).WillReturnRows(sqlmock.NewRows([]string{"id", "proxy_id"}).AddRow(id, 11))
			// Missing and soft-deleted proxy rows are both absent from this projection.
			mock.ExpectQuery(`SELECT .* FROM "proxies" WHERE .*"deleted_at" IS NULL`).WithArgs(int64(11)).WillReturnRows(sqlmock.NewRows([]string{"id"}))
			mock.ExpectQuery(`SELECT .* FROM "account_groups"`).WithArgs(id).WillReturnRows(sqlmock.NewRows([]string{"account_id", "group_id"}))
			mock.ExpectRollback()
			account, parent, err := (&accountRepository{client: client}).GetOpenAITurnAdmission(context.Background(), 7)
			require.ErrorIs(t, err, service.ErrProxyNotFound)
			require.Nil(t, account)
			require.Nil(t, parent)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestOpenAITurnAdmissionKeepsMissingGroupMetadataDistinct(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	defer func() { _ = client.Close() }()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "accounts"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
	mock.ExpectQuery(`SELECT .* FROM "account_groups"`).WillReturnRows(sqlmock.NewRows([]string{"account_id", "group_id"}).AddRow(7, 9))
	mock.ExpectQuery(`SELECT .* FROM "groups"`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectCommit()
	account, parent, err := (&accountRepository{client: client}).GetOpenAITurnAdmission(context.Background(), 7)
	require.NoError(t, err)
	require.Nil(t, parent)
	require.Equal(t, []int64{9}, account.GroupIDs)
	require.Empty(t, account.Groups)
	require.Len(t, account.AccountGroups, 1)
	require.EqualValues(t, 9, account.AccountGroups[0].GroupID)
	require.Nil(t, account.AccountGroups[0].Group)
	require.NoError(t, mock.ExpectationsWereMet())
}
