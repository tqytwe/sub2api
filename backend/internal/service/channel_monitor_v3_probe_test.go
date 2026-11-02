package service

import (
	"context"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type channelMonitorV3ProbeGroupFake struct {
	GroupRepository
	group *Group
}

func (f channelMonitorV3ProbeGroupFake) GetByID(context.Context, int64) (*Group, error) {
	return f.group, nil
}

type channelMonitorV3ProbeAccountFake struct {
	AccountRepository
	accounts []Account
}

func (f channelMonitorV3ProbeAccountFake) ListSchedulableByGroupIDAndPlatform(context.Context, int64, string) ([]Account, error) {
	return f.accounts, nil
}

type channelMonitorV3ProbeRunnerFake struct {
	called  int
	account int64
	model   string
	prompt  string
	effort  string
	result  *ScheduledTestResult
	err     error
	onRun   func()
}

func (f *channelMonitorV3ProbeRunnerFake) RunQualityProbeBackground(_ context.Context, accountID int64, modelID, prompt string) (*ScheduledTestResult, error) {
	f.called++
	f.account, f.model, f.prompt = accountID, modelID, prompt
	if f.onRun != nil {
		f.onRun()
	}
	return f.result, f.err
}

func (f *channelMonitorV3ProbeRunnerFake) RunQualityProbeWithEffortBackground(_ context.Context, accountID int64, modelID, prompt, effort string) (*ScheduledTestResult, error) {
	f.effort = effort
	return f.RunQualityProbeBackground(context.Background(), accountID, modelID, prompt)
}

var channelMonitorV3ProbeQuestion = regexp.MustCompile(`Q: (\d+) ([+-]) (\d+) = \?`)

func channelMonitorV3ProbeAnswer(prompt string, wrong bool) string {
	matches := channelMonitorV3ProbeQuestion.FindAllStringSubmatch(prompt, -1)
	if len(matches) == 0 {
		return ""
	}
	match := matches[len(matches)-1]
	a, _ := strconv.Atoi(match[1])
	b, _ := strconv.Atoi(match[3])
	answer := a + b
	if match[2] == "-" {
		answer = a - b
	}
	if wrong {
		answer++
	}
	return strconv.Itoa(answer)
}

func channelMonitorV3ProbeFixture(runner *channelMonitorV3ProbeRunnerFake) (*ChannelMonitorV3Prober, *monitorV3RepoFake) {
	now := time.Date(2026, 11, 2, 9, 34, 30, 0, time.UTC)
	cfg := monitorV3Config()
	cfg.IdleProbe = ChannelMonitorV3IdleProbe{Enabled: true, Groups: []ChannelMonitorV3IdleProbeGroup{{GroupID: 4, Model: "gpt-5.5", ReasoningEffort: "high"}}}
	repo := &monitorV3RepoFake{
		config:     cfg,
		components: []ChannelMonitorV3Component{{ID: 1, GroupID: 4, Model: "gpt-5.5", Enabled: true, GroupStatus: StatusActive}},
	}
	group := &Group{ID: 4, Platform: PlatformOpenAI, Status: StatusActive}
	accounts := channelMonitorV3ProbeAccountFake{accounts: []Account{{ID: 77, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true}}}
	prober := NewChannelMonitorV3Prober(repo, channelMonitorV3ProbeGroupFake{group: group}, accounts, runner,
		channelMonitorV2RuntimeStub{rt: ChannelMonitorRuntime{Enabled: true, Mode: ChannelMonitorModeV3}})
	prober.now = func() time.Time { return now }
	return prober, repo
}

func TestChannelMonitorV3ProberRunDueVerdictsAndEffort(t *testing.T) {
	for _, tc := range []struct {
		name     string
		result   func(prompt string) *ScheduledTestResult
		err      error
		outcome  string
		category string
	}{
		{name: "correct answer", result: func(prompt string) *ScheduledTestResult {
			return &ScheduledTestResult{Status: "success", ResponseText: channelMonitorV3ProbeAnswer(prompt, false)}
		}, outcome: ChannelMonitorV3ProbeSuccess},
		{name: "wrong answer", result: func(prompt string) *ScheduledTestResult {
			return &ScheduledTestResult{Status: "success", ResponseText: channelMonitorV3ProbeAnswer(prompt, true)}
		}, outcome: ChannelMonitorV3ProbeFailure, category: ChannelMonitorV3ProbeWrongAnswer},
		{name: "upstream failure", result: func(string) *ScheduledTestResult {
			return &ScheduledTestResult{Status: "failed", ErrorMessage: "API returned 503: unavailable"}
		}, outcome: ChannelMonitorV3ProbeFailure, category: "upstream_5xx"},
		{name: "timeout", err: context.DeadlineExceeded, outcome: ChannelMonitorV3ProbeFailure, category: "timeout"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner := &channelMonitorV3ProbeRunnerFake{}
			runner.err = tc.err
			if tc.result != nil {
				runner.onRun = func() { runner.result = tc.result(runner.prompt) }
			}
			prober, repo := channelMonitorV3ProbeFixture(runner)
			prober.RunDue(context.Background(), prober.now())
			require.Equal(t, 1, runner.called)
			require.Equal(t, int64(77), runner.account)
			require.Equal(t, "gpt-5.5", runner.model)
			require.Equal(t, "high", runner.effort)
			require.Len(t, repo.finished, 1)
			require.Equal(t, tc.outcome, repo.finished[0].Outcome)
			require.Equal(t, tc.category, repo.finished[0].Category)
		})
	}
}

func TestChannelMonitorV3ProberRecordsUnavailableAccountPool(t *testing.T) {
	runner := &channelMonitorV3ProbeRunnerFake{}
	prober, repo := channelMonitorV3ProbeFixture(runner)
	prober.accounts = channelMonitorV3ProbeAccountFake{}
	prober.RunDue(context.Background(), prober.now())
	require.Zero(t, runner.called)
	require.Len(t, repo.finished, 1)
	require.Equal(t, "account_pool_unavailable", repo.finished[0].Category)
}

func TestChannelMonitorV3ProberRespectsAccountGroupModelAllowlist(t *testing.T) {
	runner := &channelMonitorV3ProbeRunnerFake{}
	prober, repo := channelMonitorV3ProbeFixture(runner)
	prober.accounts = channelMonitorV3ProbeAccountFake{accounts: []Account{{
		ID: 77, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true,
		AccountGroups: []AccountGroup{{GroupID: 4, AllowedModels: []string{"gpt-5.4"}}},
	}}}
	prober.RunDue(context.Background(), prober.now())
	require.Zero(t, runner.called)
	require.Len(t, repo.finished, 1)
	require.Equal(t, "account_pool_unavailable", repo.finished[0].Category)
	require.Nil(t, repo.finished[0].CostUSD, "cost is unknown when no upstream request ran")
}

func TestChannelMonitorV3ProberAbandonsOnShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	runner := &channelMonitorV3ProbeRunnerFake{result: &ScheduledTestResult{Status: "failed", ErrorMessage: "context canceled"}}
	runner.onRun = cancel
	prober, repo := channelMonitorV3ProbeFixture(runner)
	prober.RunDue(ctx, prober.now())
	require.Len(t, repo.claims, 1)
	require.Empty(t, repo.finished, "shutdown interruption must not mark a channel down")
}

func TestChannelMonitorV3ProberUsesPerModelTrafficAndDeduplicatesSlot(t *testing.T) {
	runner := &channelMonitorV3ProbeRunnerFake{}
	runner.result = &ScheduledTestResult{Status: "success"}
	runner.onRun = func() { runner.result.ResponseText = channelMonitorV3ProbeAnswer(runner.prompt, false) }
	prober, repo := channelMonitorV3ProbeFixture(runner)
	repo.components = append(repo.components, ChannelMonitorV3Component{ID: 2, GroupID: 4, Model: "gpt-b", Enabled: true, GroupStatus: StatusActive})
	repo.successCounts = []ChannelMonitorV3Fact{{GroupID: 4, Model: "gpt-5.5", Success: 1}}
	prober.RunDue(context.Background(), prober.now())
	require.Equal(t, 1, runner.called, "traffic on gpt-5.5 must not suppress gpt-b")
	require.Equal(t, "gpt-b", runner.model)
	prober.RunDue(context.Background(), prober.now())
	require.Equal(t, 1, runner.called, "the same group/model/slot is claimed once")
}

func TestChannelMonitorV3ProberPrunesHourlyUsingAvailabilityRange(t *testing.T) {
	runner := &channelMonitorV3ProbeRunnerFake{}
	prober, repo := channelMonitorV3ProbeFixture(runner)
	repo.config.IdleProbe.Enabled = false
	prober.RunDue(context.Background(), prober.now())
	prober.RunDue(context.Background(), prober.now().Add(30*time.Minute))
	require.Equal(t, []time.Time{prober.now().Add(-8 * 24 * time.Hour)}, repo.prunedBefore)
	repo.config.AvailabilityRange = "30d"
	prober.RunDue(context.Background(), prober.now().Add(time.Hour))
	require.Equal(t, prober.now().Add(time.Hour-31*24*time.Hour), repo.prunedBefore[1])
}

func TestChannelMonitorV3ProbeTargetsRequireEnabledUsableComponents(t *testing.T) {
	probe := ChannelMonitorV3IdleProbe{Enabled: true, Groups: []ChannelMonitorV3IdleProbeGroup{{GroupID: 4, Model: "default", ReasoningEffort: "low"}}}
	targets := channelMonitorV3ProbeTargets(probe, []ChannelMonitorV3Component{
		{GroupID: 4, Enabled: true, GroupStatus: StatusActive},
		{GroupID: 4, Model: "other", Enabled: true, GroupStatus: StatusActive},
		{GroupID: 4, Model: "disabled", Enabled: false, GroupStatus: StatusActive},
		{GroupID: 5, Model: "unchecked", Enabled: true, GroupStatus: StatusActive},
		{GroupID: 4, Model: "deleted", Enabled: true, GroupDeleted: true},
	})
	require.Equal(t, []channelMonitorV3ProbeTarget{
		{GroupID: 4, Filter: "", Model: "default", ReasoningEffort: "low"},
		{GroupID: 4, Filter: "other", Model: "other", ReasoningEffort: "low"},
	}, targets)
}

func TestChannelMonitorV3DueProbesUseComponentModelTraffic(t *testing.T) {
	targets := []channelMonitorV3ProbeTarget{
		{GroupID: 4, Filter: "gpt-a", Model: "gpt-a"},
		{GroupID: 4, Filter: "gpt-b", Model: "gpt-b"},
	}
	due := channelMonitorV3DueProbes(targets, []ChannelMonitorV3Fact{{GroupID: 4, Model: "gpt-a", Success: 3}}, 3)
	require.Equal(t, []channelMonitorV3ProbeTarget{targets[1]}, due)
}

func TestChannelMonitorV3ProbeVerdictAndErrorCategory(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		result *ScheduledTestResult
		want   string
	}{
		{name: "correct", result: &ScheduledTestResult{Status: "success", ResponseText: "42"}, want: ChannelMonitorV3ProbeSuccess},
		{name: "wrong", result: &ScheduledTestResult{Status: "success", ResponseText: "7"}, want: ChannelMonitorV3ProbeFailure},
		{name: "upstream error", result: &ScheduledTestResult{Status: "failed", ErrorMessage: "API returned 503: unavailable"}, want: ChannelMonitorV3ProbeFailure},
		{name: "timeout", err: context.DeadlineExceeded, want: ChannelMonitorV3ProbeFailure},
		{name: "no result", want: ChannelMonitorV3ProbeFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := channelMonitorV3ProbeVerdict(tc.err, tc.result, "42")
			require.Equal(t, tc.want, got)
		})
	}
	require.Equal(t, ChannelMonitorV3ProbeWrongAnswer, channelMonitorV3ProbeCategory("answer_mismatch: expected 42"))
	require.Equal(t, "upstream_5xx", channelMonitorV3ProbeCategory("API returned 503: unavailable"))
	require.Equal(t, "timeout", channelMonitorV3ProbeCategory("request timeout"))
}

func TestChannelMonitorV3ProbeRetentionCoversHistoryWindow(t *testing.T) {
	require.Equal(t, 8*24*time.Hour, channelMonitorV3ProbeRetention("24h"))
	require.Equal(t, 8*24*time.Hour, channelMonitorV3ProbeRetention("7d"))
	require.Equal(t, 31*24*time.Hour, channelMonitorV3ProbeRetention("30d"))
}
