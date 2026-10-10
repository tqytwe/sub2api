package repository

import (
	"context"
	"errors"
	"sort"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// UpdateAdminAccount makes the existing account and policy writers share a
// transaction. The scoped writer cannot publish a scheduler snapshot early.
func (r *accountRepository) UpdateAdminAccount(ctx context.Context, account *service.Account, options service.AccountAdminUpdateOptions) (*service.Account, error) {
	baseCtx := ctx
	client := r.client
	var tx *dbent.Tx
	if external := dbent.TxFromContext(ctx); external != nil {
		client = external.Client()
	} else {
		var err error
		tx, err = client.Tx(ctx)
		if err != nil && !errors.Is(err, dbent.ErrTxStarted) {
			return nil, err
		}
		if tx != nil {
			defer func() { _ = tx.Rollback() }()
			client = tx.Client()
			ctx = dbent.NewTxContext(ctx, tx)
		}
	}
	writer := newAccountRepositoryWithSQL(client, client, nil)
	if err := writer.UpdateWithAccountBillingSettings(ctx, account, options.ProbeEnabled, options.RateSyncEnabled, options.RateMultiplier); err != nil {
		return nil, err
	}
	changedIDs := []int64{account.ID}
	if options.PropagateProxy {
		shadows, err := writer.ListShadowsByParent(ctx, account.ID)
		if err != nil {
			return nil, err
		}
		sort.Slice(shadows, func(i, j int) bool { return shadows[i].ID < shadows[j].ID })
		for _, shadow := range shadows {
			shadow.ProxyID = account.ProxyID
			if err := writer.Update(ctx, shadow); err != nil {
				return nil, err
			}
			changedIDs = append(changedIDs, shadow.ID)
		}
	}
	if options.GroupIDs != nil {
		if err := writer.BindGroupsWithAllowedModels(ctx, account.ID, *options.GroupIDs, options.GroupAllowedModels); err != nil {
			return nil, err
		}
	} else if options.GroupAllowedModels != nil {
		if err := writer.SetGroupAllowedModels(ctx, account.ID, options.GroupAllowedModels); err != nil {
			return nil, err
		}
	}
	updated, err := writer.GetByID(ctx, account.ID)
	if err != nil {
		return nil, err
	}
	if tx != nil {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		for _, id := range changedIDs {
			r.syncSchedulerAccountSnapshot(baseCtx, id)
		}
	}
	// Caller-owned transactions publish through their transactional outbox.
	return updated, nil
}
