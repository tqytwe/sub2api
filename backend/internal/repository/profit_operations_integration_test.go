//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestProfitOperationsPersistedProfitIndependentFromAccountDebit(t *testing.T) {
	f := newProfitOperationsFixture(t)
	now := time.Now().UTC().Truncate(time.Second)
	_, err := integrationDB.Exec(`UPDATE accounts SET type='apikey',credentials='{"base_url":"HTTPS://RELAY.EXAMPLE:8443/tenant/v1/","api_key":"fixture-secret"}',extra='{"cost_multiplier":0.25}',rate_multiplier=99 WHERE id=$1`, f.account)
	require.NoError(t, err)
	for i, v := range []struct{ base, income, stats float64 }{{10, 4, 8}, {2, 0, 4}, {0, 0, 0}} {
		_, err = integrationDB.Exec(`INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,requested_model,upstream_model,total_cost,actual_cost,account_stats_cost,account_rate_multiplier,created_at) VALUES($1,$2,$3,$4,'fixture-model','fixture-public','fixture-upstream',$5,$6,$7,99,$8)`, f.user, f.key, f.account, "profit-fixture-"+now.Format("150405")+string(rune('a'+i)), v.base, v.income, v.stats, now.Add(-time.Hour))
		require.NoError(t, err)
	}
	q := service.UpstreamOperationsFilter{From: now.Add(-24 * time.Hour), Until: now, AccountID: f.account, Dimension: "upstream", ModelScope: "public", Sort: "requests", Direction: "desc", Page: 1, PageSize: 1}
	out, err := f.repo.ReadProfitOperations(context.Background(), q)
	require.NoError(t, err)
	type profitData struct {
		Summary struct {
			Requests int      `json:"requests"`
			Standard float64  `json:"standard_charge"`
			Income   float64  `json:"request_income"`
			Cost     *float64 `json:"estimated_cost"`
			Profit   *float64 `json:"estimated_profit"`
			Margin   *float64 `json:"margin"`
		}
		Rows []struct {
			Key string `json:"key"`
		}
		Total int
	}
	var data profitData
	require.NoError(t, json.Unmarshal(out.Usage, &data))
	require.Equal(t, 3, data.Summary.Requests, "automatically attributed usage must not need model-channel bindings")
	require.Equal(t, 12.0, data.Summary.Standard)
	require.Equal(t, 4.0, data.Summary.Income)
	require.NotNil(t, data.Summary.Cost)
	require.InDelta(t, 3, *data.Summary.Cost, 1e-9, "cost uses saved standard model charge × independent cost multiplier, never account debit 99")
	require.NotNil(t, data.Summary.Profit)
	require.InDelta(t, 1, *data.Summary.Profit, 1e-9)
	require.NotNil(t, data.Summary.Margin)
	require.InDelta(t, .25, *data.Summary.Margin, 1e-9, "margin uses total amounts")
	require.Len(t, data.Rows, 1)
	require.Equal(t, "https://relay.example:8443/tenant/v1", data.Rows[0].Key)
	require.NotContains(t, string(out.Usage), "fixture-secret")
	baseline := data.Summary
	for _, tc := range []struct{ scope, model string }{{"public", "fixture-public"}, {"recorded", "fixture-model"}, {"upstream", "fixture-upstream"}} {
		q.Dimension, q.ModelScope, q.Model = "model", tc.scope, tc.model
		out, err = f.repo.ReadProfitOperations(context.Background(), q)
		require.NoError(t, err)
		var drill profitData
		require.NoError(t, json.Unmarshal(out.Usage, &drill))
		require.Equal(t, baseline, drill.Summary, "model drilldown must preserve persisted financial totals after narrowing the usage projection")
		require.Len(t, drill.Rows, 1)
		require.Equal(t, tc.model, drill.Rows[0].Key)
	}
}

func TestProfitOperationsZeroLossUnknownAndCurrentConfiguration(t *testing.T) {
	f := newProfitOperationsFixture(t)
	now := time.Now().UTC().Truncate(time.Second)
	_, err := integrationDB.Exec(`UPDATE accounts SET type='apikey',credentials='{"base_url":"https://source.example/tenant/v1"}',extra='{"cost_multiplier":0.5}' WHERE id=$1`, f.account)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,total_cost,actual_cost,account_stats_cost,created_at) VALUES($1,$2,$3,'loss-fixture','model',10,0,8,$4)`, f.user, f.key, f.account, now.Add(-time.Hour))
	require.NoError(t, err)
	q := service.UpstreamOperationsFilter{From: now.Add(-24 * time.Hour), Until: now, AccountID: f.account, Dimension: "account", ModelScope: "public", Sort: "requests", Direction: "desc", Page: 1, PageSize: 20}
	read := func() map[string]any {
		out, e := f.repo.ReadProfitOperations(context.Background(), q)
		require.NoError(t, e)
		var data map[string]any
		require.NoError(t, json.Unmarshal(out.Usage, &data))
		return data
	}
	data := read()
	sum := data["summary"].(map[string]any)
	require.Equal(t, 5.0, sum["estimated_cost"])
	require.Equal(t, -5.0, sum["estimated_profit"])
	require.Nil(t, sum["margin"], "zero revenue margin is unknown")
	for _, tc := range []struct {
		extra    string
		cost     any
		defaults float64
		unknown  float64
	}{
		{`{"cost_multiplier":0}`, 0.0, 0, 0}, {`{}`, 1.0, 1, 0}, {`{"cost_multiplier":"bad"}`, nil, 0, 1}, {`{"cost_multiplier":-1}`, nil, 0, 1},
	} {
		_, err = integrationDB.Exec(`UPDATE accounts SET extra=$2::jsonb,updated_at=clock_timestamp() WHERE id=$1`, f.account, tc.extra)
		require.NoError(t, err)
		data = read()
		sum = data["summary"].(map[string]any)
		require.Equal(t, tc.cost, sum["estimated_cost"])
		require.Equal(t, tc.defaults, sum["default_multiplier_requests"])
		require.Equal(t, tc.unknown, sum["unknown_cost_requests"])
	}
	_, err = integrationDB.Exec(`UPDATE accounts SET credentials='{"base_url":"https://new.example/different/v1"}',extra='{"cost_multiplier":0.25}',deleted_at=NOW() WHERE id=$1`, f.account)
	require.NoError(t, err)
	data = read()
	sum = data["summary"].(map[string]any)
	require.Equal(t, 1.0, sum["deleted_account_requests"])
	require.Equal(t, 2.5, sum["estimated_cost"])
	var base, income, stats float64
	require.NoError(t, integrationDB.QueryRow(`SELECT total_cost,actual_cost,account_stats_cost FROM usage_logs WHERE account_id=$1`, f.account).Scan(&base, &income, &stats))
	require.Equal(t, 10.0, base)
	require.Zero(t, income)
	require.Equal(t, 8.0, stats, "configuration edits never rewrite history")
}

func TestProfitOperationsUnknownURLNoSecretsAndInvalidAmounts(t *testing.T) {
	f := newProfitOperationsFixture(t)
	now := time.Now().UTC().Truncate(time.Second)
	_, err := integrationDB.Exec(`UPDATE accounts SET type='apikey',credentials='{"base_url":"https://user:private@secret.example/v1?token=private#private","api_key":"not-for-response"}',extra='{"cost_multiplier":0.25}' WHERE id=$1`, f.account)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,total_cost,actual_cost,account_stats_cost,created_at) VALUES($1,$2,$3,'unknown-fixture','missing-price','NaN'::numeric,2,8,$4)`, f.user, f.key, f.account, now.Add(-time.Minute))
	require.NoError(t, err)
	q := service.UpstreamOperationsFilter{From: now.Add(-time.Hour), Until: now, AccountID: f.account, Dimension: "upstream", ModelScope: "public", Sort: "estimated_profit", Direction: "desc", Page: 1, PageSize: 20}
	out, err := f.repo.ReadProfitOperations(context.Background(), q)
	require.NoError(t, err)
	var data struct {
		Summary struct {
			Cost            *float64 `json:"estimated_cost"`
			Unknown         int      `json:"unknown_cost_requests"`
			UpstreamUnknown int      `json:"unknown_upstream_requests"`
		}
		Rows []struct{ Key string }
	}
	require.NoError(t, json.Unmarshal(out.Usage, &data))
	require.Nil(t, data.Summary.Cost)
	require.Equal(t, 1, data.Summary.Unknown)
	require.Equal(t, 1, data.Summary.UpstreamUnknown)
	require.Len(t, data.Rows, 1)
	require.Equal(t, "unknown:invalid_url", data.Rows[0].Key)
	for _, secret := range []string{"private", "secret.example", "not-for-response", "NaN", "Infinity"} {
		require.NotContains(t, string(out.Usage), secret)
	}
	q.Upstream = data.Rows[0].Key
	filtered, err := f.repo.ReadProfitOperations(context.Background(), q)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(filtered.Usage, &data))
	require.Len(t, data.Rows, 1, "unknown attribution stays reachable by its bucket")
}

func TestProfitOperationsSameURLWeightedMarginWithoutGroupJoinDuplication(t *testing.T) {
	f := newProfitOperationsFixture(t)
	now := time.Now().UTC().Truncate(time.Second)
	var otherGroup int64
	require.NoError(t, integrationDB.QueryRow(`INSERT INTO groups(name,platform) VALUES('profit duplicate-group fixture','openai') RETURNING id`).Scan(&otherGroup))
	t.Cleanup(func() { _, _ = integrationDB.Exec(`DELETE FROM groups WHERE id=$1`, otherGroup) })
	for _, id := range []int64{f.account, f.outsider} {
		_, err := integrationDB.Exec(`UPDATE accounts SET type='apikey',credentials='{"base_url":"https://shared.example:8443/tenant/v1"}',extra='{"cost_multiplier":0.5}' WHERE id=$1`, id)
		require.NoError(t, err)
		_, err = integrationDB.Exec(`INSERT INTO account_groups(account_id,group_id) VALUES($1,$2)`, id, otherGroup)
		require.NoError(t, err)
	}
	for i, v := range []struct {
		id           int64
		income, base float64
	}{{f.account, 10, 10}, {f.outsider, 2, 8}} {
		_, err := integrationDB.Exec(`INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,total_cost,actual_cost,account_stats_cost,created_at) VALUES($1,$2,$3,$4,'cached-per-request',10,$5,$6,$7)`, f.user, f.key, v.id, fmt.Sprintf("weighted-%d-%d", f.account, i), v.income, v.base, now.Add(-time.Minute))
		require.NoError(t, err)
	}
	q := service.UpstreamOperationsFilter{From: now.Add(-time.Hour), Until: now, Upstream: "https://shared.example:8443/tenant/v1", Dimension: "account", ModelScope: "public", Sort: "request_income", Direction: "desc", Page: 1, PageSize: 1}
	out, err := f.repo.ReadProfitOperations(context.Background(), q)
	require.NoError(t, err)
	var data struct {
		Summary struct {
			Requests int
			Income   float64 `json:"request_income"`
			Cost     float64 `json:"estimated_cost"`
			Margin   float64
		}
		Rows  []struct{ Margin float64 }
		Total int
	}
	require.NoError(t, json.Unmarshal(out.Usage, &data))
	require.Equal(t, 2, data.Summary.Requests)
	require.Equal(t, 12.0, data.Summary.Income)
	require.Equal(t, 10.0, data.Summary.Cost)
	require.InDelta(t, 1.0/6, data.Summary.Margin, 1e-9, "weighted margin is (12-10)/12, not average of 50 percent and -150 percent")
	require.Equal(t, 2, data.Total)
	require.Len(t, data.Rows, 1)
	require.InDelta(t, .5, data.Rows[0].Margin, 1e-9)
	q.Dimension = "upstream"
	out, err = f.repo.ReadProfitOperations(context.Background(), q)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(out.Usage, &data))
	require.Equal(t, 1, data.Total, "same effective URL groups independent accounts once")
}

func TestProfitOperationsHalfOpenPagingGroupsAndPathIsolation(t *testing.T) {
	f := newProfitOperationsFixture(t)
	now := time.Now().UTC().Truncate(time.Second)
	from := now.Add(-24 * time.Hour)
	for _, a := range []struct {
		id   int64
		base string
	}{{f.account, "https://same.example:8443/A/v1/"}, {f.outsider, "https://same.example:8443/B/v1/"}} {
		_, err := integrationDB.Exec(`UPDATE accounts SET type='apikey',credentials=jsonb_build_object('base_url',$2::text),extra='{"cost_multiplier":0}' WHERE id=$1`, a.id, a.base)
		require.NoError(t, err)
		for i, at := range []time.Time{from, now, from.Add(-time.Microsecond)} {
			_, err = integrationDB.Exec(`INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,total_cost,actual_cost,group_id,created_at) VALUES($1,$2,$3,$4,'per-request',2,1,$5,$6)`, f.user, f.key, a.id, fmt.Sprintf("range-%d-%d", a.id, i), f.group, at)
			require.NoError(t, err)
		}
	}
	q := service.UpstreamOperationsFilter{From: from, Until: now, GroupID: f.group, Dimension: "upstream", ModelScope: "public", Sort: "name", Direction: "asc", Page: 1, PageSize: 1}
	read := func() map[string]any {
		out, err := f.repo.ReadProfitOperations(context.Background(), q)
		require.NoError(t, err)
		var data map[string]any
		require.NoError(t, json.Unmarshal(out.Usage, &data))
		require.Equal(t, "ready", out.Errors.Status)
		if q.Model != "" && q.ModelScope != "upstream" {
			require.Equal(t, "unsupported_scope", out.Quality.Status)
		} else {
			require.Equal(t, "ready", out.Quality.Status)
		}
		return data
	}
	first := read()
	require.Equal(t, 2.0, first["total"])
	require.Equal(t, 2.0, first["summary"].(map[string]any)["requests"])
	require.Len(t, first["rows"], 1)
	require.Equal(t, "https://same.example:8443/A/v1", first["rows"].([]any)[0].(map[string]any)["key"])
	require.Equal(t, 1.0, first["summary"].(map[string]any)["margin"], "zero procurement cost remains an explicit 100 percent margin")
	q.Page = 2
	second := read()
	require.Equal(t, first["summary"], second["summary"])
	require.Equal(t, "https://same.example:8443/B/v1", second["rows"].([]any)[0].(map[string]any)["key"])
	q.Model = "nonexistent"
	empty := read()
	require.Equal(t, 0.0, empty["summary"].(map[string]any)["requests"])
}

type profitOperationsFixture struct {
	repo                                         *channelRepository
	channel, group, account, outsider, user, key int64
}

func newProfitOperationsFixture(t *testing.T) *profitOperationsFixture {
	t.Helper()
	ctx := context.Background()
	f := &profitOperationsFixture{repo: &channelRepository{db: integrationDB}}
	ch := &service.Channel{Name: fmt.Sprintf("%s-%d", t.Name(), time.Now().UnixNano()), Status: service.StatusActive, BillingModelSource: service.BillingModelSourceRequested}
	require.NoError(t, f.repo.Create(ctx, ch))
	f.channel = ch.ID
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO groups(name,platform) VALUES($1,'openai') RETURNING id`, fmt.Sprintf("operations-%d", f.channel)).Scan(&f.group))
	require.NoError(t, f.repo.SetGroupIDs(ctx, f.channel, []int64{f.group}))
	a := mustCreateAccount(t, integrationEntClient, &service.Account{Name: "Operations account", Platform: service.PlatformOpenAI, Credentials: map[string]any{"api_key": "synthetic-fixture-only"}})
	f.account = a.ID
	a = mustCreateAccount(t, integrationEntClient, &service.Account{Name: "Outside account", Platform: service.PlatformOpenAI})
	f.outsider = a.ID
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO account_groups(account_id,group_id) VALUES($1,$2)`, f.account, f.group)
	require.NoError(t, err)
	u := mustCreateUser(t, integrationEntClient, &service.User{Email: fmt.Sprintf("operations-%d@test.invalid", f.channel)})
	f.user = u.ID
	k := mustCreateApiKey(t, integrationEntClient, &service.APIKey{UserID: u.ID, Key: fmt.Sprintf("operations-fixture-%d", f.channel), Name: "Operations fixture"})
	f.key = k.ID
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM ops_error_logs WHERE account_id IN ($1,$2)`, f.account, f.outsider)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM scheduled_test_plans WHERE account_id IN ($1,$2)`, f.account, f.outsider)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM usage_logs WHERE account_id IN ($1,$2)`, f.account, f.outsider)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM accounts WHERE id IN ($1,$2)`, f.account, f.outsider)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, f.user)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM channels WHERE id=$1`, f.channel)
		_, _ = integrationDB.ExecContext(ctx, `DELETE FROM groups WHERE id=$1`, f.group)
	})
	return f
}

func TestProfitOperationsExplicitChannelIsolatesUsageQualityAndErrors(t *testing.T) {
	ctx := context.Background()
	fixture := newProfitOperationsFixture(t)
	plans := NewScheduledTestPlanRepository(integrationDB)
	results := NewScheduledTestResultRepository(integrationDB)
	at := time.Now().UTC()
	var channelIDs []int64
	t.Cleanup(func() {
		for _, id := range channelIDs {
			_, err := integrationDB.Exec(`DELETE FROM upstream_channel_bindings WHERE channel_id=$1`, id)
			require.NoError(t, err)
			_, err = integrationDB.Exec(`DELETE FROM upstream_channels WHERE id=$1`, id)
			require.NoError(t, err)
		}
	})
	for _, accountID := range []int64{fixture.account, fixture.outsider} {
		name := fmt.Sprintf("channel-%d", accountID)
		channel, err := fixture.repo.CreateUpstreamChannel(ctx, service.UpstreamChannelMutation{Code: name, Name: name, Confirm: true})
		require.NoError(t, err)
		channelIDs = append(channelIDs, channel.ID)
		_, err = fixture.repo.BindUpstreamChannelAccounts(ctx, service.UpstreamChannelBindingsMutation{
			ChannelID: channel.ID, Bindings: []service.UpstreamChannelBinding{{AccountID: accountID}}, Confirm: true,
		})
		require.NoError(t, err)
		_, err = integrationDB.Exec(`UPDATE accounts SET type='apikey',credentials='{"base_url":"https://same.example/v1"}' WHERE id=$1`, accountID)
		require.NoError(t, err)
		_, err = integrationDB.Exec(`INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,total_cost,actual_cost,created_at) VALUES($1,$2,$3,$4,'model',2,1,clock_timestamp())`, fixture.user, fixture.key, accountID, name)
		require.NoError(t, err)
		_, err = integrationDB.Exec(`INSERT INTO ops_error_logs(account_id,error_phase,error_type,severity,status_code,created_at) VALUES($1,'upstream','upstream_error','error',500,$2)`, accountID, at)
		require.NoError(t, err)
		config := &service.PelicanTestConfig{ModelID: "model", ParallelCount: 1, Quality: &service.QualityPolicy{Action: "observe_only"}}
		plan, err := plans.Create(ctx, &service.ScheduledTestPlan{AccountID: accountID, ModelID: "model", CronExpression: "*/30 * * * *", Enabled: true, MaxResults: 10, PelicanConfig: config})
		require.NoError(t, err)
		_, err = results.Create(ctx, &service.ScheduledTestResult{PlanID: plan.ID, StartedAt: at, FinishedAt: at, Status: "success", PelicanConfig: config, QualityRoundID: name, QualityAction: "observed", QualityJudgment: map[string]any{"verdict": "correct"}})
		require.NoError(t, err)
	}
	for _, id := range channelIDs {
		report, err := fixture.repo.ReadProfitOperations(ctx, service.UpstreamOperationsFilter{From: at.Add(-time.Minute), Until: at.Add(time.Minute), ChannelID: id, ModelScope: "upstream", Dimension: "upstream", Sort: "requests", Direction: "desc", Page: 1, PageSize: 20})
		require.NoError(t, err)
		type counts struct {
			Summary struct {
				Requests int `json:"requests"`
			} `json:"summary"`
			Rows []struct {
				ChannelID int64 `json:"channel_id"`
			} `json:"rows"`
		}
		var usage counts
		require.NoError(t, json.Unmarshal(report.Usage, &usage))
		require.Equal(t, 1, usage.Summary.Requests)
		require.Len(t, usage.Rows, 1)
		require.Equal(t, id, usage.Rows[0].ChannelID)
		require.Equal(t, "ready", report.Quality.Status, report.Quality.Error)
		require.Equal(t, "ready", report.Errors.Status, report.Errors.Error)
		var quality struct{ Total int }
		var errors struct{ Records int }
		require.NoError(t, json.Unmarshal(report.Quality.Data, &quality))
		require.NoError(t, json.Unmarshal(report.Errors.Data, &errors))
		require.Equal(t, 1, quality.Total)
		require.Equal(t, 1, errors.Records)
	}
}

func TestProfitOperationsFollowsEventTimeOwnershipAfterRelinkAndUnlink(t *testing.T) {
	f := newProfitOperationsFixture(t)
	ctx := context.Background()
	provider, err := f.repo.CreateMatureProvider(ctx, service.MatureProviderInput{
		Name: fmt.Sprintf("profit-fixture-%d", f.account), SiteType: "newapi", BaseURL: "https://fixture.example",
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := integrationDB.Exec(`DELETE FROM mature_providers WHERE id=$1`, provider.ID)
		require.NoError(t, err)
	})
	var importedChannel int64
	require.NoError(t, integrationDB.QueryRow(`SELECT channel_id FROM provider_profit_links WHERE provider_id=$1`, provider.ID).Scan(&importedChannel))
	manualName := fmt.Sprintf("profit-override-%d", f.account)
	manual, err := f.repo.CreateUpstreamChannel(ctx, service.UpstreamChannelMutation{Code: manualName, Name: manualName, Confirm: true})
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE accounts SET type='apikey',platform='openai',
		credentials='{"base_url":"https://fixture.example","api_key":"fixture"}',
		extra=jsonb_build_object('provider_binding',jsonb_build_object('provider_id',$2::bigint,'provider_revision',$3::bigint,'key_id','1')) WHERE id=$1`, f.account, provider.ID, provider.Revision)
	require.NoError(t, err)
	start := time.Now().UTC().Add(-time.Minute)
	insertUsage := func(request string) {
		t.Helper()
		_, err := integrationDB.Exec(`INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,total_cost,actual_cost,created_at)
			VALUES($1,$2,$3,$4,'model',2,1,clock_timestamp())`, f.user, f.key, f.account, request)
		require.NoError(t, err)
	}
	insertUsage("provider-import")
	_, err = integrationDB.Exec(`INSERT INTO upstream_channel_bindings(account_id,channel_id,revision) VALUES($1,$2,1)`, f.account, manual.ID)
	require.NoError(t, err)
	insertUsage("manual-override")
	_, err = integrationDB.Exec(`UPDATE upstream_channel_bindings SET channel_id=NULL,revision=2 WHERE account_id=$1`, f.account)
	require.NoError(t, err)
	insertUsage("manual-unlink")
	filt := service.UpstreamOperationsFilter{From: start, Until: time.Now().UTC().Add(time.Second), AccountID: f.account, Dimension: "upstream", ModelScope: "public", Sort: "requests", Direction: "desc", Page: 1, PageSize: 20}
	read := func() map[int64]int {
		t.Helper()
		out, err := f.repo.ReadProfitOperations(ctx, filt)
		require.NoError(t, err)
		var data struct {
			Rows []struct {
				ChannelID int64 `json:"channel_id"`
				Requests  int   `json:"requests"`
			} `json:"rows"`
		}
		require.NoError(t, json.Unmarshal(out.Usage, &data))
		byChannel := map[int64]int{}
		for _, row := range data.Rows {
			byChannel[row.ChannelID] += row.Requests
		}
		return byChannel
	}
	require.Equal(t, map[int64]int{importedChannel: 1, manual.ID: 1, 0: 1}, read())
	filt.ChannelID = importedChannel
	require.Equal(t, map[int64]int{importedChannel: 1}, read())
}
