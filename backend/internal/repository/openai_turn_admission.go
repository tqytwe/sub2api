package repository

import (
	"context"
	"database/sql"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

var _ service.OpenAITurnAdmissionReader = (*accountRepository)(nil)

// GetOpenAITurnAdmission obtains routing state and shadow credentials from one
// read-only primary-database snapshot. Never use scheduler cache fallback here:
// a partially hydrated account cannot authorize another upstream turn.
//
// Reuse the full GetByID projection inside the transaction so both this fork's
// public Group allowlist and the source membership AllowedModels restrictions,
// along with proxy-fallback fields, are preserved.
// This reader does not itself authorize a request or replace auth-time billing
// group/pricing snapshots; the caller must apply the admission predicate.
func (r *accountRepository) GetOpenAITurnAdmission(ctx context.Context, id int64) (*service.Account, *service.Account, error) {
	tx, err := r.client.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback() }()
	reader := &accountRepository{client: tx.Client()}
	account, err := reader.GetByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	// A missing/deleted configured proxy must not silently turn into direct egress.
	if account.ProxyID != nil && account.Proxy == nil {
		return nil, nil, service.ErrProxyNotFound
	}
	var parent *service.Account
	if account.IsShadow() {
		parent, err = reader.GetByID(ctx, *account.ParentAccountID)
		if err != nil {
			return nil, nil, err
		}
		if parent.ProxyID != nil && parent.Proxy == nil {
			return nil, nil, service.ErrProxyNotFound
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}
	return account, parent, nil
}
