package service

import (
	"context"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type qualityModelValidationAccountRepo struct {
	AccountRepository
	account *Account
	err     error
}

func (r *qualityModelValidationAccountRepo) GetByID(context.Context, int64) (*Account, error) {
	return r.account, r.err
}

type qualityModelValidationPlanRepo struct {
	ScheduledTestPlanRepository
	created int
	updated int
}

func (r *qualityModelValidationPlanRepo) Create(_ context.Context, plan *ScheduledTestPlan) (*ScheduledTestPlan, error) {
	r.created++
	return plan, nil
}

func (r *qualityModelValidationPlanRepo) Update(_ context.Context, plan *ScheduledTestPlan) (*ScheduledTestPlan, error) {
	r.updated++
	return plan, nil
}

func qualityModelValidationPlan(enabled bool, models ...string) *ScheduledTestPlan {
	return &ScheduledTestPlan{
		AccountID:      42,
		ModelID:        models[0],
		CronExpression: "0 * * * *",
		Enabled:        enabled,
		PelicanConfig: &PelicanTestConfig{
			ModelIDs: models,
			Prompt:   "say 42",
			Quality: &QualityPolicy{
				Action:         "observe_only",
				ExpectedAnswer: "42",
				Judge:          &QualityJudgeConfig{Kind: "expected"},
			},
		},
	}
}

func qualityModelValidationService(repo *qualityModelValidationPlanRepo, account *Account) *ScheduledTestService {
	accountRepo := &qualityModelValidationAccountRepo{account: account}
	svc := NewScheduledTestService(repo, nil)
	svc.SetAccountTestService(&AccountTestService{accountRepo: accountRepo})
	return svc
}

func TestCreateQualityPlanRejectsModelOutsideAccountContract(t *testing.T) {
	repo := &qualityModelValidationPlanRepo{}
	svc := qualityModelValidationService(repo, &Account{
		ID:          42,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"model_mapping": map[string]any{"allowed": "gpt-6-sol"}},
	})

	_, err := svc.CreatePlan(context.Background(), qualityModelValidationPlan(true, "blocked"))

	require.Equal(t, "QUALITY_MODEL_NOT_ALLOWED", infraerrors.Reason(err))
	require.Zero(t, repo.created)
}

func TestCreateQualityPlanAcceptsWildcardAccountContract(t *testing.T) {
	repo := &qualityModelValidationPlanRepo{}
	svc := qualityModelValidationService(repo, &Account{
		ID:          42,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"model_mapping": map[string]any{"gpt-*": "*"}},
	})

	_, err := svc.CreatePlan(context.Background(), qualityModelValidationPlan(true, "gpt-6-sol"))

	require.NoError(t, err)
	require.Equal(t, 1, repo.created)
}

func TestUpdateDisabledQualityPlanDoesNotRequireCurrentModelSupport(t *testing.T) {
	repo := &qualityModelValidationPlanRepo{}
	svc := qualityModelValidationService(repo, &Account{
		ID:          42,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"model_mapping": map[string]any{"allowed": "gpt-6-sol"}},
	})

	_, err := svc.UpdatePlan(context.Background(), qualityModelValidationPlan(false, "old-model"))

	require.NoError(t, err)
	require.Equal(t, 1, repo.updated)
}
