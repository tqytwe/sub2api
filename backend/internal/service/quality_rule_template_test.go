package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type templatePlans struct {
	plans map[int64]*ScheduledTestPlan
	next  int64
}

func newTemplatePlans() *templatePlans { return &templatePlans{plans: map[int64]*ScheduledTestPlan{}} }
func (r *templatePlans) Create(_ context.Context, plan *ScheduledTestPlan) (*ScheduledTestPlan, error) {
	for _, existing := range r.plans {
		if existing.AccountID == plan.AccountID && qualityRuleScope(existing.PelicanConfig) == qualityRuleScope(plan.PelicanConfig) {
			return nil, ErrQualityTemplateSkipped
		}
	}
	r.next++
	copy := *plan
	copy.ID = r.next
	r.plans[copy.ID] = &copy
	return &copy, nil
}
func (r *templatePlans) GetByID(_ context.Context, id int64) (*ScheduledTestPlan, error) {
	plan, ok := r.plans[id]
	if !ok {
		return nil, ErrQualityTemplateNotFound
	}
	copy := *plan
	return &copy, nil
}
func (r *templatePlans) ListByAccountID(context.Context, int64) ([]*ScheduledTestPlan, error) {
	return nil, nil
}
func (r *templatePlans) ListQualityPlans(context.Context) ([]*ScheduledTestPlan, error) {
	out := make([]*ScheduledTestPlan, 0, len(r.plans))
	for _, p := range r.plans {
		out = append(out, p)
	}
	return out, nil
}
func (r *templatePlans) TriggerQuality(context.Context, int64) error { return nil }
func (r *templatePlans) ListDue(context.Context, time.Time) ([]*ScheduledTestPlan, error) {
	return nil, nil
}
func (r *templatePlans) Update(_ context.Context, plan *ScheduledTestPlan) (*ScheduledTestPlan, error) {
	copy := *plan
	r.plans[plan.ID] = &copy
	return &copy, nil
}
func (r *templatePlans) Delete(_ context.Context, id int64) error { delete(r.plans, id); return nil }
func (r *templatePlans) UpdateAfterRun(context.Context, int64, time.Time, time.Time) error {
	return nil
}

type templateRepoFake struct {
	plans     *templatePlans
	next      int64
	templates map[int64]*QualityRuleTemplate
	accounts  []QualityTemplateAccount
	links     map[int64]map[int64]int64
}

func newTemplateRepoFake(plans *templatePlans, accounts ...QualityTemplateAccount) *templateRepoFake {
	return &templateRepoFake{plans: plans, accounts: accounts, templates: map[int64]*QualityRuleTemplate{}, links: map[int64]map[int64]int64{}}
}
func (r *templateRepoFake) ListPresets(context.Context) ([]*QualityRulePreset, error) {
	return nil, nil
}
func (r *templateRepoFake) CreatePreset(context.Context, *QualityRulePreset) (*QualityRulePreset, error) {
	return nil, nil
}
func (r *templateRepoFake) List(context.Context) ([]*QualityRuleTemplate, error) {
	out := make([]*QualityRuleTemplate, 0, len(r.templates))
	for _, t := range r.templates {
		out = append(out, r.snapshot(t))
	}
	return out, nil
}
func (r *templateRepoFake) Get(_ context.Context, id int64) (*QualityRuleTemplate, error) {
	t, ok := r.templates[id]
	if !ok {
		return nil, ErrQualityTemplateNotFound
	}
	return r.snapshot(t), nil
}
func (r *templateRepoFake) Create(_ context.Context, t *QualityRuleTemplate) (*QualityRuleTemplate, error) {
	r.next++
	copy := *t
	copy.ID = r.next
	copy.UpdatedAt = time.Unix(r.next, 0)
	r.templates[copy.ID] = &copy
	r.links[copy.ID] = map[int64]int64{}
	return r.snapshot(&copy), nil
}
func (r *templateRepoFake) Update(_ context.Context, t *QualityRuleTemplate) (*QualityRuleTemplate, error) {
	current, ok := r.templates[t.ID]
	if !ok {
		return nil, ErrQualityTemplateNotFound
	}
	copy := *t
	copy.UpdatedAt = current.UpdatedAt.Add(time.Second)
	r.templates[t.ID] = &copy
	return r.snapshot(&copy), nil
}
func (r *templateRepoFake) Delete(_ context.Context, id int64) ([]int64, error) {
	if _, ok := r.templates[id]; !ok {
		return nil, ErrQualityTemplateNotFound
	}
	ids := []int64{}
	for _, planID := range r.links[id] {
		if planID != 0 {
			ids = append(ids, planID)
		}
	}
	delete(r.templates, id)
	delete(r.links, id)
	return ids, nil
}
func (r *templateRepoFake) MatchingAccounts(context.Context, QualityRuleAccountFilter) ([]QualityTemplateAccount, error) {
	return r.accounts, nil
}
func (r *templateRepoFake) LinkedAccountIDs(_ context.Context, id int64) (map[int64]struct{}, error) {
	out := map[int64]struct{}{}
	for accountID := range r.links[id] {
		out[accountID] = struct{}{}
	}
	return out, nil
}
func (r *templateRepoFake) CreateLinkedPlan(_ context.Context, t *QualityRuleTemplate, plan *ScheduledTestPlan) (*ScheduledTestPlan, error) {
	current, ok := r.templates[t.ID]
	if !ok || !current.Enabled || !current.UpdatedAt.Equal(t.UpdatedAt) {
		return nil, ErrQualityTemplateSkipped
	}
	if _, ok := r.links[t.ID][plan.AccountID]; ok {
		return nil, ErrQualityTemplateSkipped
	}
	created, err := r.plans.Create(context.Background(), plan)
	if err != nil {
		return nil, ErrQualityTemplateSkipped
	}
	r.links[t.ID][plan.AccountID] = created.ID
	return created, nil
}
func (r *templateRepoFake) MarkSynced(_ context.Context, id int64, at time.Time) error {
	if t, ok := r.templates[id]; ok {
		t.LastSyncedAt = &at
	}
	return nil
}
func (r *templateRepoFake) snapshot(t *QualityRuleTemplate) *QualityRuleTemplate {
	copy := *t
	copy.PelicanConfig = cloneQualityPelicanConfig(t.PelicanConfig)
	copy.PlanIDs = []int64{}
	for _, id := range r.links[t.ID] {
		if id != 0 {
			copy.PlanIDs = append(copy.PlanIDs, id)
		}
	}
	return &copy
}

func templateFixture() *QualityRuleTemplate {
	return &QualityRuleTemplate{AccountFilter: QualityRuleAccountFilter{Group: "7", Statuses: []string{StatusActive}}, ModelID: "gpt-5.1", CronExpression: "*/30 * * * *", Enabled: true, MaxResults: 10, PelicanConfig: &PelicanTestConfig{Prompt: "say 21", Quality: &QualityPolicy{Action: "disable_scheduling", ExpectedAnswer: "21", Judge: &QualityJudgeConfig{Kind: "expected"}}}}
}
func templateAccount(id int64) QualityTemplateAccount {
	return QualityTemplateAccount{ID: id, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
}

func TestQualityTemplateCreatesMatchingAccountsAndIsIdempotent(t *testing.T) {
	plans := newTemplatePlans()
	repo := newTemplateRepoFake(plans, templateAccount(1), templateAccount(2))
	svc := NewScheduledTestService(plans, nil, repo)
	created, count, err := svc.CreateQualityTemplate(context.Background(), templateFixture())
	require.NoError(t, err)
	require.Equal(t, 2, count)
	require.Len(t, plans.plans, 2)
	count, err = svc.SyncQualityTemplates(context.Background())
	require.NoError(t, err)
	require.Zero(t, count)
	require.NotNil(t, created.LastSyncedAt)
}

func TestQualityTemplateAddsLaterAccountAndPauseStopsFollowing(t *testing.T) {
	plans := newTemplatePlans()
	repo := newTemplateRepoFake(plans, templateAccount(1))
	svc := NewScheduledTestService(plans, nil, repo)
	template, _, err := svc.CreateQualityTemplate(context.Background(), templateFixture())
	require.NoError(t, err)
	repo.accounts = append(repo.accounts, templateAccount(2))
	added, err := svc.SyncQualityTemplates(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, added)
	template.Enabled = false
	updated, err := svc.UpdateQualityTemplate(context.Background(), template)
	require.NoError(t, err)
	require.Equal(t, 2, updated.Updated)
	repo.accounts = append(repo.accounts, templateAccount(3))
	added, err = svc.SyncQualityTemplates(context.Background())
	require.NoError(t, err)
	require.Zero(t, added)
}

func TestQualityTemplateDeleteRetainsOrDeletesPlans(t *testing.T) {
	for _, deletePlans := range []bool{false, true} {
		t.Run(map[bool]string{false: "retain", true: "delete"}[deletePlans], func(t *testing.T) {
			plans := newTemplatePlans()
			repo := newTemplateRepoFake(plans, templateAccount(1))
			svc := NewScheduledTestService(plans, nil, repo)
			template, _, err := svc.CreateQualityTemplate(context.Background(), templateFixture())
			require.NoError(t, err)
			deleted, err := svc.DeleteQualityTemplate(context.Background(), template.ID, deletePlans)
			require.NoError(t, err)
			if deletePlans {
				require.Equal(t, 1, deleted)
				require.Empty(t, plans.plans)
			} else {
				require.Zero(t, deleted)
				require.Len(t, plans.plans, 1)
			}
		})
	}
}
