//go:build integration

package repository

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAdminCheckinRepairUsesFrozenEvidenceAndAuditsOnce(t *testing.T) {
	seedCheckinCashPool(t)
	id := newCheckinMilestoneUser(t, true)
	today := time.Now().UTC()
	_, err := integrationDB.Exec(`DELETE FROM play_checkins WHERE user_id=$1 AND checkin_date=$2::date`, id, today.AddDate(0, 0, -7).Format("2006-01-02"))
	require.NoError(t, err)
	server := newCheckinContractServer(t, true, id)
	status, _, err := checkinContractRequest(server, id, http.MethodPost, "")
	require.NoError(t, err)
	require.Equal(t, 200, status)
	repo := &playRepository{client: integrationEntClient, sql: integrationDB}
	require.NoError(t, repo.InsertCheckin(context.Background(), id, today.AddDate(0, 0, -1), 0, 6))
	// A different current configuration must not change today's frozen $1 milestone.
	settings := service.NewSettingService(&blindboxIntegrationSettingRepo{values: map[string]string{service.SettingKeyPlayCheckinStreakMilestones: `[{"days":7,"bonus":99}]`}}, nil)
	svc := service.ProvidePlayService(repo, NewUserRepository(integrationEntClient, integrationDB), nil, settings, nil, integrationEntClient, service.NewBalanceLedgerService(integrationDB, nil, nil), nil, service.NewCouponService(NewCouponRepository(integrationDB)), nil)
	target := service.PlayCheckinRepairTarget{UserID: id, Date: today.Format("2006-01-02")}
	preview, err := svc.PreviewAdminCheckinRepair(context.Background(), target)
	require.NoError(t, err)
	require.True(t, preview.CanRepair)
	require.Equal(t, 1.0, preview.Bonus)
	actor := newCheckinExplorer(t, true, 30)
	input := service.PlayCheckinRepairInput{Items: []service.PlayCheckinRepairConfirmation{{Target: target, Digest: preview.Digest}}, ActorUserID: actor, Reason: "Verified original eligibility and milestone snapshot"}
	// A later invalid item must roll back the first item's money and audit too.
	invalidUser := newCheckinExplorer(t, true, 30)
	badBatch := input
	badBatch.Items = append(append([]service.PlayCheckinRepairConfirmation{}, input.Items...), service.PlayCheckinRepairConfirmation{Target: service.PlayCheckinRepairTarget{UserID: invalidUser, Date: target.Date}, Digest: strings.Repeat("0", 64)})
	_, err = svc.RepairAdminCheckins(context.Background(), badBatch)
	require.Error(t, err)
	var rolledBack int
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM play_reward_ledger WHERE user_id=$1 AND source='checkin_milestone'`, id).Scan(&rolledBack))
	require.Zero(t, rolledBack)
	var results [2][]service.PlayCheckinRepairPreview
	var errors [2]error
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errors[i] = svc.RepairAdminCheckins(context.Background(), input)
		}(i)
	}
	wg.Wait()
	winner := 0
	if errors[0] != nil {
		winner = 1
	}
	require.Error(t, errors[1-winner])
	result, err := results[winner], errors[winner]
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, 1.0, result[0].Bonus)
	_, err = svc.RepairAdminCheckins(context.Background(), input)
	require.Error(t, err)
	var count int
	var amount float64
	var storedActor int64
	var reason string
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*),COALESCE(SUM(amount),0) FROM play_reward_ledger WHERE user_id=$1 AND source='checkin_milestone'`, id).Scan(&count, &amount))
	require.Equal(t, 1, count)
	require.Equal(t, 1.0, amount)
	require.NoError(t, integrationDB.QueryRow(`SELECT a.actor_user_id,a.reason FROM play_checkin_reconciliations a JOIN play_checkins c ON c.id=a.checkin_id WHERE c.user_id=$1`, id).Scan(&storedActor, &reason))
	require.Equal(t, actor, storedActor)
	require.Equal(t, input.Reason, reason)
	after, err := svc.PreviewAdminCheckinRepair(context.Background(), target)
	require.NoError(t, err)
	require.False(t, after.CanRepair)
}

func TestAdminCheckinRepairDoesNotInventHistoricalEligibility(t *testing.T) {
	repo := &playRepository{client: integrationEntClient, sql: integrationDB}
	id := newCheckinExplorer(t, true, 30)
	day := time.Now().UTC().AddDate(0, 0, -1)
	require.NoError(t, repo.InsertCheckin(context.Background(), id, day, 0, 0))
	svc := service.NewPlayService(repo, nil, nil, nil, nil, integrationEntClient)
	preview, err := svc.PreviewAdminCheckinRepair(context.Background(), service.PlayCheckinRepairTarget{UserID: id, Date: day.Format("2006-01-02")})
	require.NoError(t, err)
	require.False(t, preview.CanRepair)
	require.Zero(t, preview.Bonus)
	require.Equal(t, "historical_evidence_missing", preview.Status)
}
