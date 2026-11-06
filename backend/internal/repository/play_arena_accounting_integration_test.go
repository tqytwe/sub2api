//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestArenaTokenAccountingPreservesLegacyPeriods(t *testing.T) {
	ctx := context.Background()
	repo := &playRepository{client: integrationEntClient, sql: integrationDB}
	userID := newCheckinExplorer(t, false, 30)
	start := time.Date(2032, 11, 4, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 1)
	var periodID int64
	err := integrationDB.QueryRow(`INSERT INTO play_arena_periods (name,start_at,end_at,status,period_type,token_accounting_version) VALUES ('test',$1,$2,'active','daily',1) RETURNING id`, start, end).Scan(&periodID)
	require.NoError(t, err)
	// Minimal real usage rows need the existing required key/account fixtures.
	var keyID, accountID int64
	require.NoError(t, integrationDB.QueryRow(`INSERT INTO api_keys (user_id,key,name) VALUES ($1,$2,'test') RETURNING id`, userID, "accounting-test-key").Scan(&keyID))
	require.NoError(t, integrationDB.QueryRow(`INSERT INTO accounts (name,platform,type,credentials) VALUES ('accounting','openai','apikey','{}') RETURNING id`).Scan(&accountID))
	_, err = integrationDB.Exec(`INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,input_tokens,output_tokens,cache_creation_tokens,cache_read_tokens,created_at) VALUES ($1,$2,$3,'accounting-test','fixture',10000,2000,1000,7000,$4)`, userID, keyID, accountID, start.Add(time.Hour))
	require.NoError(t, err)
	tokens, _, err := repo.GetUserArenaScore(ctx, userID, start, end)
	require.NoError(t, err)
	require.EqualValues(t, 13000, tokens)
	_, err = integrationDB.Exec(`UPDATE play_arena_periods SET token_accounting_version=2 WHERE id=$1`, periodID)
	require.NoError(t, err)
	tokens, _, err = repo.GetUserArenaScore(ctx, userID, start, end)
	require.NoError(t, err)
	require.EqualValues(t, 20000, tokens)
	rows, err := repo.ListArenaLeaderboard(ctx, start, end, 10)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.EqualValues(t, 20000, rows[0].TokenSum)
}

func TestArenaAccountingActivationRequiresCurrentPreviewAndRecordsActor(t *testing.T) {
	userID := newCheckinExplorer(t, false, 30)
	start := time.Date(2033, 11, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 1)
	var id int64
	require.NoError(t, integrationDB.QueryRow(`INSERT INTO play_arena_periods(name,start_at,end_at,period_type,token_accounting_version) VALUES('preview',$1,$2,'daily',1) RETURNING id`, start, end).Scan(&id))
	repo := &playRepository{client: integrationEntClient, sql: integrationDB}
	svc := service.NewPlayService(repo, nil, nil, nil, nil, integrationEntClient)
	ctx := context.Background()
	preview, err := svc.PreviewArenaAccounting(ctx, id)
	require.NoError(t, err)
	require.True(t, preview.CanActivate)
	require.Equal(t, "legacy_live", preview.RewardPolicySource)
	input := service.PlayArenaAccountingActivation{PeriodID: id, ActorUserID: userID, ExpectedDigest: "00000000000000000000000000000000", Reason: "Review accounting delta before activation"}
	_, err = svc.ActivateArenaAccounting(ctx, input)
	require.Error(t, err)
	input.ExpectedDigest = preview.Digest
	updated, err := svc.ActivateArenaAccounting(ctx, input)
	require.NoError(t, err)
	require.Equal(t, 2, updated.Version)
	require.False(t, updated.CanActivate)
	require.Equal(t, "frozen", updated.RewardPolicySource)
	var actor int64
	require.NoError(t, integrationDB.QueryRow(`SELECT actor_user_id FROM play_arena_accounting_events WHERE period_id=$1`, id).Scan(&actor))
	require.Equal(t, userID, actor)
	_, err = svc.ActivateArenaAccounting(ctx, input)
	require.Error(t, err)
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM play_arena_accounting_events WHERE period_id=$1`, id).Scan(&count))
	require.Equal(t, 1, count)
}

func TestArenaSettlementRollsBackWholePeriodAndRetriesOnce(t *testing.T) {
	ctx := context.Background()
	first := newCheckinExplorer(t, false, 30)
	second := newCheckinExplorer(t, false, 30)
	start := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	var periodID int64
	require.NoError(t, integrationDB.QueryRow(`INSERT INTO play_arena_periods(name,start_at,end_at,status,period_type,reward_rules_json,reward_rules_frozen_at) VALUES('atomic',$1,$2,'active','monthly','[{"rank_max":2,"amount":1}]',NOW()) RETURNING id`, start, end).Scan(&periodID))
	var accountID int64
	require.NoError(t, integrationDB.QueryRow(`INSERT INTO accounts(name,platform,type,credentials) VALUES('atomic','openai','apikey','{}') RETURNING id`).Scan(&accountID))
	for i, id := range []int64{first, second} {
		var keyID int64
		require.NoError(t, integrationDB.QueryRow(`INSERT INTO api_keys(user_id,key,name) VALUES($1,$2,'atomic') RETURNING id`, id, fmt.Sprintf("atomic-%d", id)).Scan(&keyID))
		_, err := integrationDB.Exec(`INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,input_tokens,created_at) VALUES($1,$2,$3,$4,'fixture',$5,$6)`, id, keyID, accountID, fmt.Sprintf("atomic-%d", id), 20000-i*1000, start.Add(time.Hour))
		require.NoError(t, err)
	}
	repo := &playRepository{client: integrationEntClient, sql: integrationDB}
	settings := service.NewSettingService(&blindboxIntegrationSettingRepo{values: map[string]string{service.SettingKeyPlayArenaEnabled: "true"}}, nil)
	svc := service.NewPlayService(repo, nil, nil, settings, nil, integrationEntClient)
	// The second user cannot receive an update; the first grant must roll back.
	_, err := integrationDB.Exec(`UPDATE users SET deleted_at=NOW() WHERE id=$1`, second)
	require.NoError(t, err)
	_, err = svc.SettleArenaPeriod(ctx, periodID)
	require.Error(t, err)
	var ledgerCount, snapshotCount int
	var state string
	var balance float64
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM play_reward_ledger WHERE detail->>'period_id'=$1::text`, periodID).Scan(&ledgerCount))
	require.Zero(t, ledgerCount)
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM play_arena_season_snapshots WHERE period_id=$1`, periodID).Scan(&snapshotCount))
	require.Zero(t, snapshotCount)
	require.NoError(t, integrationDB.QueryRow(`SELECT balance FROM users WHERE id=$1`, first).Scan(&balance))
	require.InDelta(t, 1, balance, 1e-8)
	_, err = integrationDB.Exec(`UPDATE users SET deleted_at=NULL WHERE id=$1`, second)
	require.NoError(t, err)
	result, err := svc.SettleArenaPeriod(ctx, periodID)
	require.NoError(t, err)
	require.Equal(t, 2, result.WinnersCount)
	require.NoError(t, integrationDB.QueryRow(`SELECT status FROM play_arena_periods WHERE id=$1`, periodID).Scan(&state))
	require.Equal(t, "settled", state)
	_, err = svc.SettleArenaPeriod(ctx, periodID)
	require.Error(t, err)
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM play_reward_ledger WHERE detail->>'period_id'=$1::text`, periodID).Scan(&ledgerCount))
	require.Equal(t, 2, ledgerCount)
}

func TestSettledArenaAdminReportUsesPayoutEvidenceAfterUsageChanges(t *testing.T) {
	ctx := context.Background()
	userID := newCheckinExplorer(t, false, 30)
	for _, periodType := range []string{"monthly", "daily"} {
		t.Run(periodType, func(t *testing.T) {
			start := time.Date(2002, 2, 1, 0, 0, 0, 0, time.UTC)
			end := start.AddDate(0, 1, 0)
			if periodType == "daily" {
				end = start.AddDate(0, 0, 1)
			}
			var periodID int64
			require.NoError(t, integrationDB.QueryRow(`INSERT INTO play_arena_periods(name,start_at,end_at,status,period_type) VALUES('historical',$1,$2,'settled',$3) RETURNING id`, start, end, periodType).Scan(&periodID))
			source := "arena_settlement"
			if periodType == "daily" {
				source = "arena_daily_settlement"
			}
			_, err := integrationDB.Exec(`INSERT INTO play_reward_ledger(user_id,source,amount,idempotency_key,detail) VALUES($1,$2,1.25,$3,jsonb_build_object('period_id',$4::bigint,'rank',2,'token_sum',30000))`, userID, source, fmt.Sprintf("%s:%d:%d", source, periodID, userID), periodID)
			require.NoError(t, err)
			repo := &playRepository{client: integrationEntClient, sql: integrationDB}
			svc := service.NewPlayService(repo, nil, nil, nil, nil, integrationEntClient)
			rows, _, _, err := svc.ListAdminArenaLeaderboard(ctx, periodType, periodID, 50)
			require.NoError(t, err)
			require.Len(t, rows, 1, "historical ranking must survive usage cleanup and must not query today's reward config")
			require.Equal(t, 2, rows[0].Rank)
			require.EqualValues(t, 30000, rows[0].TokenSum)
			require.Equal(t, 1.25, rows[0].PaidReward)
			require.Zero(t, rows[0].EstimatedReward)
			require.Equal(t, "payout_recorded", rows[0].EvidenceStatus)
		})
	}
}

func TestAdminArenaPeriodListKeepsTypesAndEmptyPageTotals(t *testing.T) {
	repo := &playRepository{client: integrationEntClient, sql: integrationDB}
	svc := service.NewPlayService(repo, nil, nil, nil, nil, integrationEntClient)
	start := time.Date(2048, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 2; i++ {
		_, err := integrationDB.Exec(`INSERT INTO play_arena_periods(name,start_at,end_at,status,period_type) VALUES($1,$2,$3,'settled','daily')`, fmt.Sprintf("history-%d", i), start.AddDate(0, 0, i), start.AddDate(0, 0, i+1))
		require.NoError(t, err)
	}
	result, err := svc.ListAdminArenaPeriods(context.Background(), "daily", 1, 1)
	require.NoError(t, err)
	require.GreaterOrEqual(t, result.Total, 2)
	require.Len(t, result.Items, 1)
	require.Equal(t, "history-1", result.Items[0].Name)
	next, err := svc.ListAdminArenaPeriods(context.Background(), "daily", 2, 1)
	require.NoError(t, err)
	require.Len(t, next.Items, 1)
	require.NotEqual(t, result.Items[0].ID, next.Items[0].ID)
	empty, err := svc.ListAdminArenaPeriods(context.Background(), "daily", 10000, 100)
	require.NoError(t, err)
	require.Empty(t, empty.Items)
	require.Equal(t, result.Total, empty.Total)
	_, err = svc.ListAdminArenaPeriods(context.Background(), "anything", 1, 20)
	require.Error(t, err)
}

func TestAdminFarmReportExplainsCurrentEnergyWithoutWritingProgress(t *testing.T) {
	require.NoError(t, timezone.Init("Asia/Shanghai"))
	t.Cleanup(func() { require.NoError(t, timezone.Init("UTC")) })
	ctx := context.Background()
	userID := newCheckinExplorer(t, false, 30)
	repo := &playRepository{client: integrationEntClient, sql: integrationDB}
	settings := service.NewSettingService(&blindboxIntegrationSettingRepo{values: map[string]string{
		service.SettingKeyPlayArenaEnabled:       "true",
		service.SettingKeyPlayDailyQuestsEnabled: "true",
		service.SettingKeyPlayCheckinEnabled:     "true",
		service.SettingKeyPlayDailyQuests:        `[{"key":"checkin","energy":10}]`,
	}}, nil)
	svc := service.NewPlayService(repo, nil, nil, settings, nil, integrationEntClient)
	now := time.Now().In(timezone.Location())
	_, err := svc.PrepareCurrentArenaMonthlySeason(ctx, now)
	require.NoError(t, err)
	require.NoError(t, repo.InsertCheckin(ctx, userID, now, 0, 1))
	var keyID, accountID int64
	require.NoError(t, integrationDB.QueryRow(`INSERT INTO api_keys(user_id,key,name) VALUES($1,$2,'energy report') RETURNING id`, userID, fmt.Sprintf("report-%d", userID)).Scan(&keyID))
	require.NoError(t, integrationDB.QueryRow(`INSERT INTO accounts(name,platform,type,credentials) VALUES('energy report','openai','apikey','{}') RETURNING id`).Scan(&accountID))
	_, err = integrationDB.Exec(`INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,input_tokens,output_tokens,cache_creation_tokens,cache_read_tokens,created_at) VALUES($1,$2,$3,$4,'fixture',10000,2000,1000,7000,$5)`, userID, keyID, accountID, fmt.Sprintf("report-%d", userID), now)
	require.NoError(t, err)
	report, err := svc.GetAdminArenaUserReport(ctx, userID, "")
	require.NoError(t, err)
	raw, err := json.Marshal(report)
	require.NoError(t, err)
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &fields))
	require.Contains(t, fields, "current_farm", "admin must be able to explain the same 10 + 2 = 12 as the user")
	var farm service.PlayQuestToday
	require.NoError(t, json.Unmarshal(fields["current_farm"], &farm))
	require.Equal(t, 10, farm.TaskEnergy)
	require.EqualValues(t, 2, farm.MonthlyTokenEnergy)
	require.Equal(t, 12, farm.Energy)
	progress, err := repo.ListQuestProgress(ctx, userID, now)
	require.NoError(t, err)
	require.Empty(t, progress, "admin inspection must not mark quests complete")
	userFarm, err := svc.GetQuestsToday(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, userFarm.Energy, farm.Energy)
	require.Equal(t, userFarm.Tasks, farm.Tasks)
	historical, err := svc.GetAdminArenaUserReport(ctx, userID, now.AddDate(0, 0, -1).Format("2006-01-02"))
	require.NoError(t, err)
	raw, err = json.Marshal(historical)
	require.NoError(t, err)
	require.NotContains(t, string(raw), `"current_farm"`, "do not present current task rules as historical evidence")
}
