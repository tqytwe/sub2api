//go:build unit

package service

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

type revokePackageQuotaUserSubRepoStub struct {
	userSubRepoNoop
	sub     *UserSubscription
	deleted bool
}

func (r *revokePackageQuotaUserSubRepoStub) GetByID(_ context.Context, id int64) (*UserSubscription, error) {
	if r.sub == nil || r.sub.ID != id || r.deleted {
		return nil, ErrSubscriptionNotFound
	}
	cp := *r.sub
	return &cp, nil
}

func (r *revokePackageQuotaUserSubRepoStub) GetByIDForUpdate(ctx context.Context, id int64) (*UserSubscription, error) {
	return r.GetByID(ctx, id)
}

func (r *revokePackageQuotaUserSubRepoStub) Delete(_ context.Context, id int64) error {
	if r.sub == nil || r.sub.ID != id || r.deleted {
		return ErrSubscriptionNotFound
	}
	r.deleted = true
	return nil
}

func TestRevokeSubscriptionRevokesPackageEntitlementsAtomically(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE subscription_package_entitlements
				SET status = 'revoked', exhausted_reason = NULL, updated_at = NOW()
				WHERE user_id = $1 AND group_id = $2 AND status <> 'revoked'`)).
		WithArgs(int64(451), int64(62)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := &revokePackageQuotaUserSubRepoStub{sub: &UserSubscription{
		ID:        42,
		UserID:    451,
		GroupID:   62,
		Status:    SubscriptionStatusActive,
		ExpiresAt: time.Now().Add(time.Hour),
	}}
	svc := NewSubscriptionService(groupRepoNoop{}, repo, nil, client, nil)
	t.Cleanup(svc.Stop)

	require.NoError(t, svc.RevokeSubscription(context.Background(), 42))
	require.True(t, repo.deleted)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRevokeSubscriptionRollsBackWhenPackageEntitlementRevocationFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE subscription_package_entitlements
				SET status = 'revoked', exhausted_reason = NULL, updated_at = NOW()
				WHERE user_id = $1 AND group_id = $2 AND status <> 'revoked'`)).
		WithArgs(int64(451), int64(62)).
		WillReturnError(errors.New("package entitlement store unavailable"))
	mock.ExpectRollback()

	repo := &revokePackageQuotaUserSubRepoStub{sub: &UserSubscription{
		ID:        42,
		UserID:    451,
		GroupID:   62,
		Status:    SubscriptionStatusActive,
		ExpiresAt: time.Now().Add(time.Hour),
	}}
	svc := NewSubscriptionService(groupRepoNoop{}, repo, nil, client, nil)
	t.Cleanup(svc.Stop)

	err = svc.RevokeSubscription(context.Background(), 42)
	require.ErrorContains(t, err, "revoke package entitlements")
	require.NoError(t, mock.ExpectationsWereMet())
}
