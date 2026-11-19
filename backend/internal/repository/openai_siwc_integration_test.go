//go:build integration

package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/google/uuid"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSIWCGrantCASAndWorkerOwnershipPreservePrimaryCredentials(t *testing.T) {
	ctx, repo, cfg := totpRotationFixture(t)
	accounts := NewAccountRepository(integrationEntClient, integrationDB, nil)
	before, err := accounts.GetByID(ctx, cfg.AccountID)
	require.NoError(t, err)
	host, previous, err := repo.SIWCRegistration(ctx, cfg.AccountID, "urn:uuid:first")
	require.NoError(t, err)
	require.Empty(t, previous)
	repeated, _, err := repo.SIWCRegistration(ctx, cfg.AccountID, "urn:uuid:ignored")
	require.NoError(t, err)
	require.Equal(t, host, repeated)
	task, err := repo.CreateSIWCTask(ctx, cfg.AccountID, "snapshot")
	require.NoError(t, err)
	require.Equal(t, "siwc", task.OAuthProfile)
	_, err = repo.CreateTask(ctx, cfg.AccountID, "snapshot")
	require.Error(t, err)
	claimed, err := repo.ClaimNextTask(ctx, "local", time.Minute)
	require.NoError(t, err)
	require.Equal(t, task.ID, claimed.ID)
	require.Equal(t, "siwc", claimed.OAuthProfile)
	require.NoError(t, repo.SetSession(ctx, task.ID, "local", "server-session"))
	claimed, started, err := repo.BeginCallback(ctx, task.ID, "local")
	require.NoError(t, err)
	require.True(t, started)
	changed, err := repo.SaveSIWCCredentials(ctx, cfg.AccountID, map[string]any{"email": "stale@example.test"}, "", "encrypted:first", true, claimed)
	require.NoError(t, err)
	require.False(t, changed)
	changed, err = repo.SaveSIWCCredentials(ctx, cfg.AccountID, before.Credentials, "wrong-snapshot", "encrypted:first", true, claimed)
	require.NoError(t, err)
	require.False(t, changed)
	changed, err = repo.SaveSIWCCredentials(ctx, cfg.AccountID, before.Credentials, "", "encrypted:first", true, claimed)
	require.NoError(t, err)
	require.True(t, changed)
	completed, err := repo.GetTask(ctx, task.ID)
	require.NoError(t, err)
	require.Equal(t, "succeeded", completed.Status)
	changed, err = repo.SaveSIWCCredentials(ctx, cfg.AccountID, before.Credentials, "", "encrypted:stale", false, nil)
	require.NoError(t, err)
	require.False(t, changed)
	saved, err := repo.GetSIWCCredentials(ctx, cfg.AccountID)
	require.NoError(t, err)
	require.Equal(t, "encrypted:first", saved)
	after, err := accounts.GetByID(ctx, cfg.AccountID)
	require.NoError(t, err)
	require.Equal(t, before.Credentials, after.Credentials)
	require.Equal(t, before.Status, after.Status)
	require.Equal(t, before.Schedulable, after.Schedulable)
	// Independent refresh must tolerate rotation of the primary Codex bearer.
	_, err = integrationDB.ExecContext(ctx, `UPDATE accounts SET credentials=jsonb_set(credentials,'{access_token}','"rotated-codex"'::jsonb) WHERE id=$1`, cfg.AccountID)
	require.NoError(t, err)
	changed, err = repo.SaveSIWCCredentials(ctx, cfg.AccountID, before.Credentials, "encrypted:first", "encrypted:refreshed", false, nil)
	require.NoError(t, err)
	require.True(t, changed)
	after, err = accounts.GetByID(ctx, cfg.AccountID)
	require.NoError(t, err)
	require.Equal(t, "rotated-codex", after.GetCredential("access_token"))
}

func TestSIWCRevocationStopsOnlySIWCTasksAndPreservesPrimaryCredentials(t *testing.T) {
	for _, profile := range []string{"siwc", "codex"} {
		t.Run(profile, func(t *testing.T) {
			ctx, repo, cfg := totpRotationFixture(t)
			_, err := integrationDB.ExecContext(ctx, `UPDATE accounts SET extra='{"openai_siwc":true,"custom_label":"preserved"}'::jsonb WHERE id=$1`, cfg.AccountID)
			require.NoError(t, err)
			accounts := NewAccountRepository(integrationEntClient, integrationDB, nil)
			before, err := accounts.GetByID(ctx, cfg.AccountID)
			require.NoError(t, err)
			_, _, err = repo.SIWCRegistration(ctx, cfg.AccountID, "urn:uuid:test")
			require.NoError(t, err)
			changed, err := repo.SaveSIWCCredentials(ctx, cfg.AccountID, before.Credentials, "", "encrypted:grant", true, nil)
			require.NoError(t, err)
			require.True(t, changed)
			task, err := repo.CreateTask(ctx, cfg.AccountID, "snapshot")
			if profile == "siwc" {
				require.NoError(t, err)
				_, err = integrationDB.ExecContext(ctx, `UPDATE openai_oauth_reauth_tasks SET oauth_profile='siwc' WHERE id=$1`, task.ID)
			}
			require.NoError(t, err)
			require.NoError(t, repo.RevokeSIWCCredentials(ctx, cfg.AccountID))
			saved, err := repo.GetSIWCCredentials(ctx, cfg.AccountID)
			require.NoError(t, err)
			require.Empty(t, saved)
			after, err := accounts.GetByID(ctx, cfg.AccountID)
			require.NoError(t, err)
			require.Equal(t, before.Credentials, after.Credentials)
			require.Equal(t, "preserved", after.Extra["custom_label"])
			require.Equal(t, false, after.Extra["openai_siwc"])
			final, err := repo.GetTask(ctx, task.ID)
			require.NoError(t, err)
			if profile == "siwc" {
				require.Equal(t, "failed", final.Status)
			} else {
				require.Equal(t, "queued", final.Status)
			}
		})
	}
}

func TestSIWCMigrationAddsProfileToLegacyTaskSchema(t *testing.T) {
	ctx := context.Background()
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	schema := "siwc_upgrade_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = tx.ExecContext(ctx, `CREATE SCHEMA `+schema+`; SET LOCAL search_path TO `+schema)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `CREATE TABLE accounts(id BIGINT PRIMARY KEY); INSERT INTO accounts VALUES(1);
 CREATE TABLE openai_oauth_reauth_tasks(id BIGINT PRIMARY KEY,account_id BIGINT NOT NULL); INSERT INTO openai_oauth_reauth_tasks VALUES(1,1)`)
	require.NoError(t, err)
	migration, err := migrations.FS.ReadFile("375_openai_siwc.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(migration))
	require.NoError(t, err)
	var profile string
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT oauth_profile FROM openai_oauth_reauth_tasks WHERE id=1`).Scan(&profile))
	require.Equal(t, "codex", profile, "old tasks retain the existing protocol")
	_, err = tx.ExecContext(ctx, `SAVEPOINT invalid_profile`)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `UPDATE openai_oauth_reauth_tasks SET oauth_profile='unrecognized' WHERE id=1`)
	require.Error(t, err)
	_, err = tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT invalid_profile`)
	require.NoError(t, err)
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT oauth_profile FROM openai_oauth_reauth_tasks WHERE id=1`).Scan(&profile))
	require.Equal(t, "codex", profile)
	_, err = tx.ExecContext(ctx, `UPDATE openai_oauth_reauth_tasks SET oauth_profile='siwc' WHERE id=1`)
	require.NoError(t, err)
}
