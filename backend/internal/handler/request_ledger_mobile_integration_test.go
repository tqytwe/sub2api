//go:build integration

package handler

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/Wei-Shaw/sub2api/internal/requestledger/ledgertest"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRequestLedgerMobileWorkerStartsBeforeGatewayAndRefusesStorageFailure(t *testing.T) {
	db := ledgertest.New(t)
	l := requestledger.New(db)
	ctx := context.Background()
	job := mobileVideoGatewayTestJob(service.MobileVideoAdapterGrok)
	h, err := l.Begin(ctx, "/api/v1/mobile/video/jobs", "POST", "http")
	require.NoError(t, err)
	parent := requestledger.WithHandle(ctx, h)
	require.NoError(t, requestledger.BindTask(parent, "mobile_video", job.TaskID, job.UserID, job.ExecutionAPIKeyID))
	require.NoError(t, h.Finish(ctx, "succeeded", 202, ""))
	key := &mobileVideoExecutionKeyFake{key: mobileVideoGatewayTestKey(service.PlatformComposite)}
	gateway := &mobileVideoGatewayFake{}
	provider := newMobileVideoGatewayProviderWithDependencies(key, gateway, nil)
	provider.ledger = requestledger.New(db)
	_, err = provider.Create(ctx, job)
	require.NoError(t, err)
	require.Equal(t, 1, gateway.grokCreate)
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_requests WHERE parent_id=$1 AND kind='async_execution' AND user_id=$2 AND api_key_id=$3 AND execution_state='succeeded'`, h.ID, job.UserID, job.ExecutionAPIKeyID).Scan(&count))
	require.Equal(t, 1, count)
	_, err = db.Exec(`ALTER TABLE gateway_requests RENAME TO ledger_fixture_requests_offline`)
	require.NoError(t, err)
	_, err = provider.Create(ctx, job)
	require.ErrorIs(t, err, requestledger.ErrUnavailable)
	require.Equal(t, 1, gateway.grokCreate)
}
