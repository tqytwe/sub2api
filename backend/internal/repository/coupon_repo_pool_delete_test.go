package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestDeleteCouponRewardPoolDeletesOnlyDraftVersions(t *testing.T) {
	t.Run("deletes a draft pool and its entries", func(t *testing.T) {
		db, mock := newCouponRepositoryMock(t)
		mock.ExpectExec(`(?s)DELETE FROM coupon_reward_pool_versions.*WHERE id = \$1 AND status = 'draft'`).
			WithArgs(int64(71)).
			WillReturnResult(sqlmock.NewResult(0, 1))

		repo := &couponRepository{db: db}
		require.NoError(t, repo.DeleteCouponRewardPool(context.Background(), 71))
	})

	t.Run("does not delete published or retired history", func(t *testing.T) {
		db, mock := newCouponRepositoryMock(t)
		mock.ExpectExec(`(?s)DELETE FROM coupon_reward_pool_versions.*WHERE id = \$1 AND status = 'draft'`).
			WithArgs(int64(71)).
			WillReturnResult(sqlmock.NewResult(0, 0))

		repo := &couponRepository{db: db}
		err := repo.DeleteCouponRewardPool(context.Background(), 71)
		require.Equal(t, "COUPON_POOL_DELETE_REJECTED", infraerrors.Reason(err))
	})
}
