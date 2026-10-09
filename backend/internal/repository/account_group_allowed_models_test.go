package repository

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/stretchr/testify/require"
)

type accountGroupModelsWriter interface {
	SetGroupAllowedModels(context.Context, int64, map[int64][]string) error
}
type accountModelsJSONArg []string

func (want accountModelsJSONArg) Match(value driver.Value) bool {
	var data []byte
	switch v := value.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		return false
	}
	var got []string
	if json.Unmarshal(data, &got) != nil || len(got) != len(want) {
		return false
	}
	for i := range want {
		if want[i] != got[i] {
			return false
		}
	}
	return true
}

func TestAccountGroupAllowedModelsRepositoryTransactions(t *testing.T) {
	for _, scenario := range []string{"normalize", "clear", "unchanged", "rollback", "outbox_failure"} {
		t.Run(scenario, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()
			client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
			repo := newAccountRepositoryWithSQL(client, db, nil)
			writer, ok := any(repo).(accountGroupModelsWriter)
			require.True(t, ok, "account repository must persist membership model restrictions")
			current := `["old-model"]`
			allowed := map[int64][]string{2: {" gpt-5.5 ", "gpt-5.5"}, 999: {"unbound-ignored"}}
			if scenario == "clear" {
				allowed = map[int64][]string{}
			}
			if scenario == "unchanged" {
				current = `["gpt-5.5"]`
			}
			mock.ExpectBegin()
			mock.ExpectQuery(`SELECT id FROM accounts WHERE id = \$1 AND deleted_at IS NULL FOR NO KEY UPDATE`).WithArgs(int64(17)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(17))
			mock.ExpectQuery(`SELECT .* FROM "account_groups" WHERE`).WithArgs(int64(17)).WillReturnRows(
				sqlmock.NewRows([]string{"account_id", "group_id", "priority", "allowed_models", "created_at"}).AddRow(17, 2, 5, current, time.Now()))
			if scenario != "unchanged" {
				query := `UPDATE "account_groups" SET "allowed_models" = \$1 WHERE`
				if scenario == "clear" {
					query = `UPDATE "account_groups" SET "allowed_models" = NULL WHERE`
				}
				expectation := mock.ExpectExec(query)
				if scenario == "clear" {
					expectation.WithArgs(int64(17), int64(2))
				} else {
					expectation.WithArgs(accountModelsJSONArg{"gpt-5.5"}, int64(17), int64(2))
				}
				if scenario == "rollback" {
					expectation.WillReturnError(errors.New("write failed"))
				} else {
					expectation.WillReturnResult(sqlmock.NewResult(0, 1))
				}
			}
			if scenario == "rollback" {
				mock.ExpectRollback()
			} else {
				if scenario != "unchanged" {
					outbox := mock.ExpectExec(`INSERT INTO scheduler_outbox`)
					if scenario == "outbox_failure" {
						outbox.WillReturnError(errors.New("outbox failed"))
					} else {
						outbox.WillReturnResult(sqlmock.NewResult(1, 1))
					}
				}
				if scenario == "outbox_failure" {
					mock.ExpectRollback()
				} else {
					mock.ExpectCommit()
				}
			}

			err = writer.SetGroupAllowedModels(context.Background(), 17, allowed)
			if scenario == "rollback" || scenario == "outbox_failure" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestAccountGroupAllowedModelsHydrationPreservesGroupPolicy(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	repo := newAccountRepositoryWithSQL(client, db, nil)
	mock.ExpectQuery(`SELECT .* FROM "account_groups" WHERE`).WithArgs(int64(17)).WillReturnRows(sqlmock.NewRows([]string{"account_id", "group_id", "priority", "allowed_models", "created_at"}).AddRow(17, 2, 37, `["gpt-5.5"]`, time.Now()))
	mock.ExpectQuery(`SELECT .* FROM "groups" WHERE`).WithArgs(int64(2)).WillReturnRows(sqlmock.NewRows([]string{"id", "name", "rate_multiplier", "model_allowlist"}).AddRow(2, "custom-group", 1.75, `{"enabled":true,"models":["public-alias"]}`))
	groups, ids, memberships, err := repo.loadAccountGroups(context.Background(), []int64{17})
	require.NoError(t, err)
	require.Equal(t, []int64{2}, ids[17])
	require.Equal(t, 37, memberships[17][0].Priority)
	require.Equal(t, []string{"gpt-5.5"}, memberships[17][0].AllowedModels)
	require.Equal(t, "custom-group", groups[17][0].Name)
	require.Equal(t, 1.75, groups[17][0].RateMultiplier)
	require.True(t, groups[17][0].ModelAllowlist.Allows("public-alias"))
	require.False(t, groups[17][0].ModelAllowlist.Allows("gpt-5.5"), "membership restriction must not overwrite global public-model policy")
	require.NoError(t, mock.ExpectationsWereMet())
}
