package service

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

type authFirstBindSettingRepoStub struct {
	values map[string]string
}

func (r *authFirstBindSettingRepoStub) Get(_ context.Context, key string) (*Setting, error) {
	if value, ok := r.values[key]; ok {
		return &Setting{Key: key, Value: value}, nil
	}
	return nil, ErrSettingNotFound
}

func (r *authFirstBindSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if value, ok := r.values[key]; ok {
		return value, nil
	}
	return "", ErrSettingNotFound
}

func (r *authFirstBindSettingRepoStub) Set(context.Context, string, string) error { return nil }

func (r *authFirstBindSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (r *authFirstBindSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	return nil
}

func (r *authFirstBindSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	out := make(map[string]string, len(r.values))
	for key, value := range r.values {
		out[key] = value
	}
	return out, nil
}

func (r *authFirstBindSettingRepoStub) Delete(context.Context, string) error { return nil }

func TestApplyProviderDefaultSettingsOnFirstBindWritesBalanceLedger(t *testing.T) {
	t.Parallel()

	db, mock := newBalanceLedgerSQLMock(t)
	defer func() { _ = db.Close() }()
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	defer func() { _ = client.Close() }()

	cfg := &config.Config{}
	settingSvc := NewSettingService(&authFirstBindSettingRepoStub{values: map[string]string{
		SettingKeyDefaultBalance:                            "0",
		SettingKeyDefaultConcurrency:                        "0",
		SettingKeyDefaultSubscriptions:                      "[]",
		SettingKeyAuthSourceDefaultEmailBalance:             "2.5",
		SettingKeyAuthSourceDefaultEmailConcurrency:         "0",
		SettingKeyAuthSourceDefaultEmailSubscriptions:       "[]",
		SettingKeyAuthSourceDefaultEmailGrantOnFirstBind:    "true",
		SettingKeyAuthSourceDefaultEmailGrantOnSignup:       "false",
		SettingKeyAuthSourceDefaultLinuxDoGrantOnFirstBind:  "false",
		SettingKeyAuthSourceDefaultOIDCGrantOnFirstBind:     "false",
		SettingKeyAuthSourceDefaultWeChatGrantOnFirstBind:   "false",
		SettingKeyAuthSourceDefaultGitHubGrantOnFirstBind:   "false",
		SettingKeyAuthSourceDefaultGoogleGrantOnFirstBind:   "false",
		SettingKeyAuthSourceDefaultDingTalkGrantOnFirstBind: "false",
	}}, cfg)
	createdAt := time.Date(2026, 7, 20, 8, 0, 0, 0, time.UTC)
	ledger := &BalanceLedgerService{db: db, now: func() time.Time { return createdAt }}
	svc := NewAuthService(client, nil, nil, nil, cfg, settingSvc, nil, nil, nil, nil, nil, nil, nil, ledger)

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO user_provider_default_grants")).
		WithArgs(int64(42), "email", "first_bind").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(balanceLedgerSelectByKeyPattern()).
		WithArgs(int64(42), "auth_first_bind:42:email").
		WillReturnRows(balanceTransactionRows())
	mock.ExpectQuery("(?s)FROM users\\s+WHERE id = \\$1 AND deleted_at IS NULL\\s+FOR UPDATE").
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "frozen_balance"}).AddRow(1.0, 0.0))
	mock.ExpectExec("(?s)UPDATE users\\s+SET balance = \\$1, frozen_balance = \\$2").
		WithArgs(3.5, 0.0, int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("(?s)INSERT INTO balance_transactions").
		WithArgs(
			int64(42),
			2.5,
			1.0,
			3.5,
			0.0,
			0.0,
			0.0,
			"auth_first_bind_grant",
			"email",
			"auth_first_bind:42:email",
			"system",
			nil,
			"认证源首绑赠送",
			sqlmock.AnyArg(),
			false,
			"high",
			createdAt,
		).
		WillReturnRows(balanceTransactionRows().AddRow(
			int64(7001), int64(42), 2.5, 1.0, 3.5, 0.0, 0.0, 0.0,
			"auth_first_bind_grant", "email", "auth_first_bind:42:email", "system", nil,
			"认证源首绑赠送", `{"provider_type":"email","grant_reason":"first_bind","balance":2.5}`, false, "high", createdAt,
		))
	mock.ExpectCommit()

	require.NoError(t, svc.ApplyProviderDefaultSettingsOnFirstBind(context.Background(), 42, "email"))
	require.NoError(t, mock.ExpectationsWereMet())
}
