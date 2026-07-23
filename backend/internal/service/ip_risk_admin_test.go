package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestIPRiskAdminUpdateConfigValidatesAndHotReloadsRuntime(t *testing.T) {
	t.Parallel()

	repo := &ipRiskAdminRepositoryStub{}
	core := NewIPRiskService(
		repo,
		nil,
		nil,
		NewIPRiskHasher([]byte("unit-test-ip-risk-key")),
		DefaultIPRiskRuntimeConfig(),
	)
	svc := NewIPRiskAdminService(repo, core, nil, nil, nil, NewIPRiskHasher([]byte("unit-test-ip-risk-key")))

	config := DefaultIPRiskManagedConfig()
	config.AutoBlockEnabled = true
	config.AutoBlockDurationMinutes = 120
	updated, err := svc.UpdateConfig(context.Background(), config, 42)
	require.NoError(t, err)
	require.Equal(t, config, *updated)
	require.Equal(t, int64(42), repo.updatedBy)
	require.Equal(t, config, repo.updatedConfig)

	runtime := core.Runtime(context.Background())
	require.True(t, runtime.AutoBlockEnabled)
	require.False(t, runtime.ShadowMode)

	invalid := config
	invalid.AutoBlockScore = 101
	_, err = svc.UpdateConfig(context.Background(), invalid, 42)
	require.ErrorContains(t, err, "between 0 and 100")
	require.Equal(t, 1, repo.configWrites)
}

func TestIPRiskAdminPreviewProtectsAdministratorsAndFlagsTrustedInferredUsers(t *testing.T) {
	t.Parallel()

	repo := &ipRiskAdminRepositoryStub{detail: riskAdminTestCaseDetail()}
	svc := NewIPRiskAdminService(
		repo,
		nil,
		nil,
		nil,
		nil,
		NewIPRiskHasher([]byte("unit-test-ip-risk-key")),
	)

	preview, err := svc.PreviewAction(context.Background(), 7, IPRiskActionInput{
		ActionType: RiskActionDisableUsers,
		UserIDs:    []int64{1, 2, 3},
		Reason:     "confirmed clustered registrations",
	})
	require.NoError(t, err)
	require.Equal(t, []int64{1}, preview.ProtectedUsers)
	require.Equal(t, []int64{2, 3}, preview.UserIDs)
	require.Equal(t, []int64{2}, preview.TrustedUsers)
	require.Equal(t, []int64{3}, preview.InferredUsers)
	require.Equal(t, []int64{20, 30}, preview.APIKeyIDs)
	require.True(t, preview.RequiresStepUp)
	require.NotEmpty(t, preview.ConfirmationToken)
}

func TestIPRiskAdminPreviewRejectsMoreThanFiveHundredUsers(t *testing.T) {
	t.Parallel()

	users := make([]IPRiskRelatedUser, 0, 501)
	userIDs := make([]int64, 0, 501)
	for id := int64(1); id <= 501; id++ {
		users = append(users, IPRiskRelatedUser{UserID: id, Role: RoleUser})
		userIDs = append(userIDs, id)
	}
	repo := &ipRiskAdminRepositoryStub{detail: &IPRiskCaseDetail{
		Case:  IPRiskCaseSummary{ID: 7, Version: 3},
		Users: users,
	}}
	svc := NewIPRiskAdminService(repo, nil, nil, nil, nil, NewIPRiskHasher([]byte("unit-test-ip-risk-key")))

	_, err := svc.PreviewAction(context.Background(), 7, IPRiskActionInput{
		ActionType: RiskActionDisableUsers,
		UserIDs:    userIDs,
		Reason:     "bulk risk action review",
	})
	require.ErrorContains(t, err, "more than 500 users")
}

func TestIPRiskAdminPreviewTokenExpiresAndDetectsReasonChanges(t *testing.T) {
	t.Parallel()

	svc := NewIPRiskAdminService(
		&ipRiskAdminRepositoryStub{},
		nil,
		nil,
		nil,
		nil,
		NewIPRiskHasher([]byte("unit-test-ip-risk-key")),
	)
	preview := IPRiskActionPreview{
		CaseID:      7,
		CaseVersion: 3,
		ActionType:  RiskActionObserve,
		ExpiresAt:   time.Now().UTC().Add(-time.Second),
	}
	token, err := svc.signActionPreview(preview, "original reason")
	require.NoError(t, err)
	_, err = svc.verifyActionPreview(token, "original reason")
	require.ErrorIs(t, err, ErrIPRiskActionPreviewExpired)

	preview.ExpiresAt = time.Now().UTC().Add(time.Minute)
	token, err = svc.signActionPreview(preview, "original reason")
	require.NoError(t, err)
	_, err = svc.verifyActionPreview(token, "changed reason")
	require.ErrorIs(t, err, ErrIPRiskActionPreviewStale)
}

func TestIPRiskAdminExecuteRejectsTargetStateChangesAfterPreview(t *testing.T) {
	t.Parallel()

	repo := &ipRiskAdminRepositoryStub{detail: riskAdminTestCaseDetail()}
	svc := NewIPRiskAdminService(
		repo,
		nil,
		nil,
		nil,
		nil,
		NewIPRiskHasher([]byte("unit-test-ip-risk-key")),
	)
	input := IPRiskActionInput{
		ActionType: RiskActionDisableUsers,
		UserIDs:    []int64{2},
		Reason:     "disable exact clustered registration account",
	}
	preview, err := svc.PreviewAction(context.Background(), 7, input)
	require.NoError(t, err)
	require.NotEmpty(t, preview.StateDigest)
	input.PreviewToken = preview.ConfirmationToken

	repo.detail.Users[1].Status = StatusDisabled
	record, err := svc.ExecuteAction(context.Background(), 7, 42, input)
	require.ErrorIs(t, err, ErrIPRiskActionPreviewStale)
	require.Nil(t, record)
	require.Zero(t, repo.nextActionID)
}

func TestIPRiskAdminActionItemPersistenceFailureIsReported(t *testing.T) {
	t.Parallel()

	repo := &ipRiskAdminRepositoryStub{
		detail:     riskAdminTestCaseDetail(),
		addItemErr: errors.New("action item write failed"),
	}
	svc := NewIPRiskAdminService(
		repo,
		nil,
		nil,
		nil,
		nil,
		NewIPRiskHasher([]byte("unit-test-ip-risk-key")),
	)
	input := IPRiskActionInput{
		ActionType: RiskActionObserve,
		Reason:     "observe while evidence is reviewed",
	}
	preview, err := svc.PreviewAction(context.Background(), 7, input)
	require.NoError(t, err)
	input.PreviewToken = preview.ConfirmationToken

	record, err := svc.ExecuteAction(context.Background(), 7, 42, input)
	require.ErrorContains(t, err, "action item write failed")
	require.NotNil(t, record)
	require.Equal(t, "failed", repo.completedStatus)
	require.Equal(t, 1, repo.completedResult["failed_items"])
	require.Zero(t, repo.updateCaseStatusCalls)
}

func TestIPRiskAdminCaseBlockTargetsExactIPv6Address(t *testing.T) {
	t.Parallel()

	detail := riskAdminTestCaseDetail()
	detail.Case.PrimaryIP = "2001:db8:7a4::19"
	detail.Case.PrimaryNetwork = "2001:db8:7a4::/64"
	repo := &ipRiskAdminRepositoryStub{detail: detail}
	svc := NewIPRiskAdminService(
		repo,
		nil,
		nil,
		nil,
		nil,
		NewIPRiskHasher([]byte("unit-test-ip-risk-key")),
	)
	input := IPRiskActionInput{
		ActionType:      RiskActionTemporaryRegistrationBan,
		DurationMinutes: 30,
		Reason:          "temporarily stop exact-address registrations",
	}
	preview, err := svc.PreviewAction(context.Background(), 7, input)
	require.NoError(t, err)
	input.PreviewToken = preview.ConfirmationToken

	_, err = svc.ExecuteAction(context.Background(), 7, 42, input)
	require.NoError(t, err)
	require.Equal(t, "2001:db8:7a4::19", repo.createdPolicy.ExactIP)
	require.Empty(t, repo.createdPolicy.IPNetwork)
	require.Equal(t, IPPolicyBlockRegistration, repo.createdPolicy.Mode)
	require.NotNil(t, repo.createdPolicy.ExpiresAt)
	require.Len(t, repo.createdActionInput.PreviewTokenHash, sha256.Size)
	require.NotNil(t, repo.createdActionInput.PreviewExpiresAt)
}

func TestIPRiskAdminRollbackDoesNotOverwriteLaterUserChanges(t *testing.T) {
	t.Parallel()

	userID := int64(2)
	repo := &ipRiskAdminRepositoryStub{
		sourceAction: &IPRiskActionRecord{
			ID:             9,
			ActionType:     RiskActionDisableUsers,
			RollbackStatus: "eligible",
			Items: []IPRiskActionItem{{
				TargetType:     "user",
				TargetID:       &userID,
				BeforeState:    map[string]any{"status": StatusActive},
				AfterState:     map[string]any{"status": StatusDisabled},
				Status:         "completed",
				RollbackStatus: "eligible",
			}},
		},
	}
	admin := &ipRiskAdminServiceStub{
		user: &User{ID: userID, Role: RoleUser, Status: StatusActive},
	}
	svc := NewIPRiskAdminService(
		repo,
		nil,
		admin,
		nil,
		nil,
		NewIPRiskHasher([]byte("unit-test-ip-risk-key")),
	)

	record, err := svc.RollbackAction(context.Background(), 9, 42, "restore only unchanged action state")
	require.NoError(t, err)
	require.NotNil(t, record)
	require.Zero(t, admin.updateCalls)
	require.Equal(t, "failed", repo.completedStatus)
	require.Equal(t, 1, repo.completedResult["conflict_items"])
	require.Equal(t, "failed", repo.markedRollbackStatus)
}

func TestIPRiskAdminRollbackDoesNotDeleteEditedPolicy(t *testing.T) {
	t.Parallel()

	policyID := int64(55)
	sourceActionID := int64(9)
	repo := &ipRiskAdminRepositoryStub{
		sourceAction: &IPRiskActionRecord{
			ID:             sourceActionID,
			ActionType:     RiskActionTemporaryRegistrationBan,
			RollbackStatus: "eligible",
			Items: []IPRiskActionItem{{
				TargetType:     "ip_policy",
				TargetID:       &policyID,
				Status:         "completed",
				RollbackStatus: "eligible",
				AfterState: map[string]any{
					"enabled":  true,
					"mode":     string(IPPolicyBlockRegistration),
					"exact_ip": "203.0.113.8",
					"reason":   "original action reason",
				},
			}},
		},
		policies: []IPRiskPolicy{{
			ID:             policyID,
			Mode:           IPPolicyBlockRegistration,
			ExactIP:        "203.0.113.8",
			Reason:         "edited by another administrator",
			Enabled:        true,
			SourceActionID: &sourceActionID,
		}},
	}
	svc := NewIPRiskAdminService(
		repo,
		nil,
		&ipRiskAdminServiceStub{},
		nil,
		nil,
		NewIPRiskHasher([]byte("unit-test-ip-risk-key")),
	)

	record, err := svc.RollbackAction(context.Background(), sourceActionID, 42, "restore only unchanged action state")
	require.NoError(t, err)
	require.NotNil(t, record)
	require.Zero(t, repo.deletePolicyCalls)
	require.Equal(t, "failed", repo.completedStatus)
	require.Equal(t, 1, repo.completedResult["conflict_items"])
}

func TestIPRiskAdminRollbackSkipsFailedSourceItems(t *testing.T) {
	t.Parallel()

	completedUserID := int64(2)
	failedUserID := int64(3)
	repo := &ipRiskAdminRepositoryStub{
		sourceAction: &IPRiskActionRecord{
			ID:             9,
			ActionType:     RiskActionDisableUsers,
			Status:         "partial",
			RollbackStatus: "eligible",
			Items: []IPRiskActionItem{
				{
					TargetType:     "user",
					TargetID:       &completedUserID,
					BeforeState:    map[string]any{"status": StatusActive},
					AfterState:     map[string]any{"status": StatusDisabled},
					Status:         "completed",
					RollbackStatus: "eligible",
				},
				{
					TargetType:     "user",
					TargetID:       &failedUserID,
					BeforeState:    map[string]any{"status": StatusActive},
					AfterState:     map[string]any{"status": StatusDisabled},
					Status:         "failed",
					RollbackStatus: "not_requested",
				},
			},
		},
	}
	admin := &ipRiskAdminServiceStub{
		user: &User{ID: completedUserID, Role: RoleUser, Status: StatusDisabled},
	}
	svc := NewIPRiskAdminService(
		repo,
		nil,
		admin,
		nil,
		nil,
		NewIPRiskHasher([]byte("unit-test-ip-risk-key")),
	)

	record, err := svc.RollbackAction(context.Background(), 9, 42, "restore only completed source items")
	require.NoError(t, err)
	require.NotNil(t, record)
	require.Equal(t, 1, admin.updateCalls)
	require.Equal(t, StatusActive, admin.user.Status)
	require.Equal(t, "completed", repo.completedStatus)
	require.Equal(t, 1, repo.completedResult["completed_items"])
	require.Equal(t, 0, repo.completedResult["conflict_items"])
	require.Equal(t, 1, repo.completedResult["skipped_items"])
	require.Len(t, repo.addedItems, 2)
	require.Equal(t, "completed", repo.addedItems[0].Status)
	require.Equal(t, "skipped", repo.addedItems[1].Status)
	require.Equal(t, "not_requested", repo.addedItems[1].RollbackStatus)
}

func TestIPRiskAdminRollbackRejectsIneligibleOrRepeatedActions(t *testing.T) {
	t.Parallel()

	repo := &ipRiskAdminRepositoryStub{
		sourceAction: &IPRiskActionRecord{
			ID:             9,
			ActionType:     RiskActionDisableUsers,
			Status:         "rolled_back",
			RollbackStatus: "completed",
		},
	}
	svc := NewIPRiskAdminService(
		repo,
		nil,
		&ipRiskAdminServiceStub{},
		nil,
		nil,
		NewIPRiskHasher([]byte("unit-test-ip-risk-key")),
	)

	record, err := svc.RollbackAction(context.Background(), 9, 42, "must not replay rollback")
	require.ErrorIs(t, err, ErrIPRiskActionNotRollbackEligible)
	require.Nil(t, record)
	require.Zero(t, repo.nextActionID)
}

func riskAdminTestCaseDetail() *IPRiskCaseDetail {
	return &IPRiskCaseDetail{
		Case: IPRiskCaseSummary{
			ID:             7,
			PrimaryIP:      "203.0.113.8",
			PrimaryNetwork: "203.0.113.8/32",
			Status:         string(RiskCaseStatusOpen),
			Version:        3,
		},
		Users: []IPRiskRelatedUser{
			{
				UserID: 1,
				Role:   RoleAdmin,
				APIKeys: []IPRiskRelatedKey{{
					ID: 10,
				}},
			},
			{
				UserID:             2,
				Role:               RoleUser,
				RelationType:       IPRiskUserRelationTrustedExisting,
				EvidenceConfidence: EvidenceConfidenceExact,
				APIKeys: []IPRiskRelatedKey{{
					ID: 20,
				}},
			},
			{
				UserID:             3,
				Role:               RoleUser,
				RelationType:       IPRiskUserRelationSuspectedNew,
				EvidenceConfidence: EvidenceConfidenceInferred,
				APIKeys: []IPRiskRelatedKey{{
					ID: 30,
				}},
			},
		},
	}
}

type ipRiskAdminRepositoryStub struct {
	IPRiskRepository
	detail                *IPRiskCaseDetail
	updatedConfig         IPRiskManagedConfig
	updatedBy             int64
	configWrites          int
	addItemErr            error
	completedStatus       string
	completedResult       map[string]any
	sourceAction          *IPRiskActionRecord
	markedRollbackStatus  string
	nextActionID          int64
	createdPolicy         IPRiskPolicyInput
	policies              []IPRiskPolicy
	deletePolicyCalls     int
	updateCaseStatusCalls int
	addedItems            []IPRiskActionItemCreate
	createdActionInput    IPRiskActionCreate
}

func (r *ipRiskAdminRepositoryStub) GetIPRiskCaseDetail(context.Context, int64) (*IPRiskCaseDetail, error) {
	if r.detail == nil {
		return nil, errors.New("case not found")
	}
	return r.detail, nil
}

func (r *ipRiskAdminRepositoryStub) UpdateIPRiskManagedConfig(_ context.Context, config IPRiskManagedConfig, actorID int64) error {
	r.updatedConfig = config
	r.updatedBy = actorID
	r.configWrites++
	return nil
}

func (r *ipRiskAdminRepositoryStub) LatestIPRiskScan(context.Context) (*IPRiskScan, error) {
	return nil, sql.ErrNoRows
}

func (r *ipRiskAdminRepositoryStub) CreateIPRiskPolicy(
	_ context.Context,
	input IPRiskPolicyInput,
	sourceActionID *int64,
) (*IPRiskPolicy, error) {
	r.createdPolicy = input
	return &IPRiskPolicy{
		ID:             55,
		Mode:           input.Mode,
		IPNetwork:      input.IPNetwork,
		ExactIP:        input.ExactIP,
		Reason:         input.Reason,
		Enabled:        input.Enabled,
		ExpiresAt:      input.ExpiresAt,
		SourceActionID: sourceActionID,
	}, nil
}

func (r *ipRiskAdminRepositoryStub) ListIPRiskPolicies(context.Context) ([]IPRiskPolicy, error) {
	return r.policies, nil
}

func (r *ipRiskAdminRepositoryStub) DeleteIPRiskPolicy(context.Context, int64) error {
	r.deletePolicyCalls++
	return nil
}

func (r *ipRiskAdminRepositoryStub) CreateIPRiskAction(_ context.Context, input IPRiskActionCreate) (*IPRiskActionRecord, error) {
	r.createdActionInput = input
	r.nextActionID++
	if r.nextActionID == 0 {
		r.nextActionID = 1
	}
	return &IPRiskActionRecord{
		ID:                 100 + r.nextActionID,
		CaseID:             input.CaseID,
		ActionType:         input.ActionType,
		Status:             "running",
		RollbackOfActionID: input.RollbackOfActionID,
	}, nil
}

func (r *ipRiskAdminRepositoryStub) AddIPRiskActionItem(_ context.Context, item IPRiskActionItemCreate) error {
	r.addedItems = append(r.addedItems, item)
	return r.addItemErr
}

func (r *ipRiskAdminRepositoryStub) ReserveIPRiskActionItem(_ context.Context, item IPRiskActionItemCreate) (int64, error) {
	if r.addItemErr != nil {
		return 0, r.addItemErr
	}
	item.Status = "pending"
	item.RollbackStatus = "not_requested"
	r.addedItems = append(r.addedItems, item)
	return int64(len(r.addedItems)), nil
}

func (r *ipRiskAdminRepositoryStub) FinalizeIPRiskActionItem(
	_ context.Context,
	itemID int64,
	targetID *int64,
	status,
	errorMessage,
	rollbackStatus string,
) error {
	index := int(itemID - 1)
	if index < 0 || index >= len(r.addedItems) {
		return errors.New("reserved action item not found")
	}
	r.addedItems[index].TargetID = targetID
	r.addedItems[index].Status = status
	r.addedItems[index].ErrorMessage = errorMessage
	r.addedItems[index].RollbackStatus = rollbackStatus
	return nil
}

func (r *ipRiskAdminRepositoryStub) CompleteIPRiskAction(_ context.Context, _ int64, status string, result map[string]any, _ bool) error {
	r.completedStatus = status
	r.completedResult = result
	return nil
}

func (r *ipRiskAdminRepositoryStub) GetIPRiskAction(_ context.Context, id int64) (*IPRiskActionRecord, error) {
	if r.sourceAction != nil && id == r.sourceAction.ID {
		return r.sourceAction, nil
	}
	return &IPRiskActionRecord{
		ID:     id,
		Status: r.completedStatus,
		Result: r.completedResult,
	}, nil
}

func (r *ipRiskAdminRepositoryStub) UpdateIPRiskCaseStatus(context.Context, int64, RiskCaseStatus) error {
	r.updateCaseStatusCalls++
	return nil
}

func (r *ipRiskAdminRepositoryStub) MarkIPRiskActionRolledBack(_ context.Context, _ int64, status string) error {
	r.markedRollbackStatus = status
	return nil
}

type ipRiskAdminServiceStub struct {
	AdminService
	user        *User
	updateCalls int
}

func (s *ipRiskAdminServiceStub) GetUser(context.Context, int64) (*User, error) {
	if s.user == nil {
		return nil, errors.New("user not found")
	}
	clone := *s.user
	return &clone, nil
}

func (s *ipRiskAdminServiceStub) UpdateUser(_ context.Context, _ int64, input *UpdateUserInput) (*User, error) {
	s.updateCalls++
	s.user.Status = input.Status
	clone := *s.user
	return &clone, nil
}
