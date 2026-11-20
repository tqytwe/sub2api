//go:build integration

package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/authguard"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestRegistrationIPQuotaRollsBackWithAccountCreation(t *testing.T) {
	base := context.Background()
	client := testEntClient(t)
	repo := newUserRepositoryWithSQL(client, integrationDB)
	const address = "192.0.2.173"
	key := authguard.IPKey(address)
	ctx := authguard.WithRegistrationIP(base, address)
	newUser := func(n int) *service.User {
		return &service.User{Email: fmt.Sprintf("signup-quota-%d@example.test", n), PasswordHash: "fixture-hash", Role: service.RoleUser, Status: service.StatusActive}
	}
	readCount := func() int {
		var count int
		require.NoError(t, integrationDB.QueryRowContext(base, "SELECT account_count FROM registration_ip_quotas WHERE ip_hash = $1", key).Scan(&count))
		return count
	}
	require.NoError(t, repo.Create(ctx, newUser(1)))
	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	require.NoError(t, repo.Create(dbent.NewTxContext(ctx, tx), newUser(2)))
	require.NoError(t, tx.Rollback())
	require.Equal(t, 1, readCount())

	tx, err = client.Tx(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	require.NoError(t, repo.ClaimPendingOAuthRegistrationIP(dbent.NewTxContext(ctx, tx)))
	require.NoError(t, tx.Rollback())
	require.Equal(t, 1, readCount())

	results := make(chan error, 2)
	for i := 3; i <= 4; i++ {
		go func(user *service.User) { results <- repo.Create(ctx, user) }(newUser(i))
	}
	var accepted, limited int
	for i := 0; i < 2; i++ {
		switch err := <-results; {
		case err == nil:
			accepted++
		case errors.Is(err, service.ErrRegistrationIPLimit):
			limited++
		default:
			t.Fatalf("unexpected signup result: %v", err)
		}
	}
	require.Equal(t, 1, accepted)
	require.Equal(t, 1, limited)
	require.Equal(t, 2, readCount())
	require.NoError(t, repo.Create(base, newUser(5)), "an admin-created account does not consume public signup quota")
}
