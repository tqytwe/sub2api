//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func (s *APIKeyRepoSuite) TestModelPolicyAuthRoundtripAndIndependentWriteMasks() {
	user := s.mustCreateUser("key-policy-auth@example.invalid")
	a, b := s.mustCreateGroup("key-policy-primary"), s.mustCreateGroup("key-policy-secondary")
	key := &service.APIKey{UserID: user.ID, Key: "sk-key-policy-auth", Name: "original", Status: service.StatusActive, GroupID: &a.ID, GroupIDs: []int64{a.ID, b.ID}, Quota: 100, RateLimit5h: 100}
	s.Require().NoError(s.repo.Create(s.ctx, key))
	s.Require().True(key.AllowLongContext, "new keys inherit the pre-upgrade unrestricted policy")
	s.Require().Zero(key.ContextTokenLimit)
	stale, err := s.repo.GetByID(s.ctx, key.ID)
	s.Require().NoError(err)
	_, err = s.repo.IncrementQuotaUsed(s.ctx, key.ID, 17)
	s.Require().NoError(err)
	s.Require().NoError(s.repo.IncrementRateLimitUsage(s.ctx, key.ID, 23))
	stale.ModelMapping = map[string]string{"public-model": "target-model"}
	stale.ReasoningEffort = "high"
	stale.AllowLongContext = false
	stale.ContextTokenLimit = 12345
	s.Require().NoError(s.repo.Update(s.ctx, stale, service.APIKeyUpdateFields{ModelMapping: true, ReasoningEffort: true, AllowLongContext: true, ContextTokenLimit: true}))
	got, err := s.repo.GetByKeyForAuth(s.ctx, key.Key)
	s.Require().NoError(err)
	s.Require().Equal(stale.ModelMapping, got.ModelMapping)
	s.Require().Equal("high", got.ReasoningEffort)
	s.Require().False(got.AllowLongContext)
	s.Require().Equal(12345, got.ContextTokenLimit)
	s.Require().Equal([]int64{a.ID, b.ID}, got.GroupIDs)
	s.Require().Equal(a.ID, *got.GroupID)
	s.Require().InDelta(17, got.QuotaUsed, 1e-9)
	current, err := s.repo.GetByID(s.ctx, key.ID)
	s.Require().NoError(err)
	s.Require().InDelta(23, current.Usage5h, 1e-9)
	// The ordinary user edit mask cannot write administrator fields even if its in-memory object is stale.
	key.Name = "renamed"
	key.ModelMapping = map[string]string{"public-model": "unauthorized-target"}
	key.ReasoningEffort = "ultra"
	s.Require().NoError(s.repo.Update(s.ctx, key, service.APIKeyUpdateFields{Name: true}))
	got, err = s.repo.GetByID(s.ctx, key.ID)
	s.Require().NoError(err)
	s.Require().Equal("renamed", got.Name)
	s.Require().Equal(stale.ModelMapping, got.ModelMapping)
	s.Require().Equal("high", got.ReasoningEffort)
	s.Require().False(got.AllowLongContext)
	s.Require().Equal(12345, got.ContextTokenLimit)
	// Each policy field has its own write mask: clearing one must not reset the others or the key's groups.
	got.ModelMapping = nil
	s.Require().NoError(s.repo.Update(s.ctx, got, service.APIKeyUpdateFields{ModelMapping: true}))
	got, err = s.repo.GetByKeyForAuth(s.ctx, key.Key)
	s.Require().NoError(err)
	s.Require().Empty(got.ModelMapping)
	s.Require().Equal("high", got.ReasoningEffort)
	s.Require().Equal(12345, got.ContextTokenLimit)
	s.Require().False(got.AllowLongContext)
	s.Require().Equal([]int64{a.ID, b.ID}, got.GroupIDs)
}

func TestMigration356APIKeyModelPolicyPreservesLegacyAndReplay(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)
	// A transaction-local legacy table avoids altering any existing fixture table or migration history.
	_, err := tx.ExecContext(ctx, `CREATE TEMP TABLE api_keys (id bigint PRIMARY KEY, name text NOT NULL) ON COMMIT DROP; INSERT INTO api_keys VALUES (1,'legacy')`)
	require.NoError(t, err)
	sql, err := dbmigrations.FS.ReadFile("356_api_key_model_policy.sql")
	require.NoError(t, err)
	for range 2 {
		_, err = tx.ExecContext(ctx, string(sql))
		require.NoError(t, err)
	}
	var mapping, effort, name string
	var allowed bool
	var limit int
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT name,model_mapping::text,reasoning_effort,allow_long_context,context_token_limit FROM api_keys WHERE id=1`).Scan(&name, &mapping, &effort, &allowed, &limit))
	require.Equal(t, "legacy", name)
	require.JSONEq(t, `{}`, mapping)
	require.Empty(t, effort)
	require.True(t, allowed)
	require.Zero(t, limit)
	_, err = tx.ExecContext(ctx, `UPDATE api_keys SET model_mapping='{"public":"target"}',reasoning_effort='high',allow_long_context=false,context_token_limit=1234 WHERE id=1`)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(sql))
	require.NoError(t, err)
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT model_mapping::text,reasoning_effort,allow_long_context,context_token_limit FROM api_keys WHERE id=1`).Scan(&mapping, &effort, &allowed, &limit))
	require.JSONEq(t, `{"public":"target"}`, mapping)
	require.Equal(t, "high", effort)
	require.False(t, allowed)
	require.Equal(t, 1234, limit)
	_, err = tx.ExecContext(ctx, `UPDATE api_keys SET context_token_limit=-1 WHERE id=1`)
	require.Error(t, err, "database prevents negative ceilings")
}
