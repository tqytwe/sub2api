package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestIPRiskServiceRecordsExactRegistrationWithoutRawSecrets(t *testing.T) {
	t.Parallel()

	repo := &ipRiskRepositoryStub{}
	svc := NewIPRiskService(
		repo,
		nil,
		nil,
		NewIPRiskHasher([]byte("unit-test-ip-risk-key")),
		DefaultIPRiskRuntimeConfig(),
	)
	occurredAt := time.Date(2026, time.July, 23, 8, 0, 0, 0, time.UTC)
	ctx := WithIPRiskRequestMetadata(context.Background(), IPRiskRequestMetadata{
		ClientIP:   "[2001:db8:7a4::19]:443",
		UserAgent:  "Mozilla/5.0 Chrome/148.1.2.3",
		RequestID:  "risk-request-1",
		OccurredAt: occurredAt,
	})

	err := svc.RecordRegistration(ctx, IPRiskRegistrationInput{
		UserID:         42,
		Email:          "trial01@example.test",
		SignupSource:   "email",
		InvitationCode: "invite-secret",
		AffiliateCode:  "affiliate-secret",
	})
	require.NoError(t, err)
	require.Len(t, repo.events, 1)

	event := repo.events[0]
	require.Equal(t, AuthRiskEventRegister, event.EventType)
	require.Equal(t, int64(42), event.UserID)
	require.Equal(t, "2001:db8:7a4::19", event.IPAddress)
	require.Equal(t, "2001:db8:7a4::/64", event.IPNetwork)
	require.Equal(t, EvidenceConfidenceExact, event.EvidenceConfidence)
	require.Equal(t, occurredAt, event.OccurredAt)
	require.NotEmpty(t, event.UserAgentHMAC)
	require.NotEmpty(t, event.EmailPatternHMAC)
	require.NotEmpty(t, event.InvitationHMAC)
	require.NotEmpty(t, event.AffiliateHMAC)
	require.NotContains(t, string(event.InvitationHMAC), "invite-secret")
	require.NotContains(t, string(event.AffiliateHMAC), "affiliate-secret")
	require.Equal(t, "risk-request-1", event.RequestID)
	require.Equal(t, "register:exact:user:42", event.DedupeKey)
}

func TestIPRiskServiceOmitsEmptyOptionalRegistrationHashes(t *testing.T) {
	t.Parallel()

	repo := &ipRiskRepositoryStub{}
	svc := NewIPRiskService(
		repo,
		nil,
		nil,
		NewIPRiskHasher([]byte("unit-test-ip-risk-key")),
		DefaultIPRiskRuntimeConfig(),
	)
	ctx := WithIPRiskRequestMetadata(context.Background(), IPRiskRequestMetadata{
		ClientIP:   "203.0.113.8",
		UserAgent:  "",
		RequestID:  "risk-request-empty-optionals",
		OccurredAt: time.Date(2026, time.July, 23, 8, 0, 0, 0, time.UTC),
	})

	err := svc.RecordRegistration(ctx, IPRiskRegistrationInput{
		UserID:       43,
		Email:        "owner@example.test",
		SignupSource: "email",
	})
	require.NoError(t, err)
	require.Len(t, repo.events, 1)

	event := repo.events[0]
	require.Empty(t, event.UserAgentSummary)
	require.Empty(t, event.UserAgentHMAC)
	require.Empty(t, event.EmailPatternHMAC)
	require.Empty(t, event.InvitationHMAC)
	require.Empty(t, event.AffiliateHMAC)
}

func TestIPRiskServiceDeduplicatesSuccessfulLoginByFiveMinuteBucket(t *testing.T) {
	t.Parallel()

	repo := &ipRiskRepositoryStub{}
	svc := NewIPRiskService(
		repo,
		nil,
		nil,
		NewIPRiskHasher([]byte("unit-test-ip-risk-key")),
		DefaultIPRiskRuntimeConfig(),
	)
	ctx := WithIPRiskRequestMetadata(context.Background(), IPRiskRequestMetadata{
		ClientIP:   "203.0.113.8",
		UserAgent:  "test-agent/1.0",
		RequestID:  "risk-login-1",
		OccurredAt: time.Date(2026, time.July, 23, 8, 3, 0, 0, time.UTC),
	})

	require.NoError(t, svc.RecordSuccessfulLogin(ctx, 42))
	require.Len(t, repo.events, 1)
	require.Equal(t, AuthRiskEventSuccessfulLogin, repo.events[0].EventType)
	require.Equal(t, "203.0.113.8/32", repo.events[0].IPNetwork)
	require.Contains(t, repo.events[0].DedupeKey, "login:42:203.0.113.8:")
	require.Contains(t, repo.events[0].DedupeKey, ":2026-07-23T08:00:00Z")
}

func TestIPRiskServiceEvaluateNetworkPersistsExplainableShadowCase(t *testing.T) {
	t.Parallel()

	repo := &ipRiskRepositoryStub{
		evidence: &IPRiskCandidateSnapshot{
			Evidence: IPRiskEvidence{
				PrimaryIP:                  "203.0.113.8",
				PrimaryNetwork:             "203.0.113.8/32",
				PrimaryIPRegistrationCount: 10,
				RegistrationCount24h:       10,
				ExactRegistrationCount:     10,
				MaxSharedUACount:           5,
				SharedAPIIPUserCount:       3,
				AllKeyEvidenceExact:        true,
			},
			Users: []IPRiskRelatedUserSnapshot{{
				UserID:              42,
				RelationType:        IPRiskUserRelationSuspectedNew,
				EvidenceConfidence:  EvidenceConfidenceExact,
				RecommendedSelected: true,
			}},
		},
	}
	svc := NewIPRiskService(
		repo,
		nil,
		nil,
		NewIPRiskHasher([]byte("unit-test-ip-risk-key")),
		DefaultIPRiskRuntimeConfig(),
	)
	detectedAt := time.Date(2026, time.July, 23, 8, 10, 0, 0, time.UTC)

	result, err := svc.EvaluateNetwork(context.Background(), "203.0.113.8/32", detectedAt)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 90, result.Score)
	require.Equal(t, RiskLevelCritical, result.Level)
	require.True(t, result.AutoBlockEligible)
	require.Len(t, repo.cases, 1)
	require.True(t, repo.cases[0].AutoBlockEligible)
	require.True(t, repo.cases[0].ShadowMode)
	require.Equal(t, []string{"temporary_registration_block", "review_related_users"}, repo.cases[0].RecommendedActions)
}

func TestIPRiskServiceDoesNotCreateCaseBelowMediumRisk(t *testing.T) {
	t.Parallel()

	repo := &ipRiskRepositoryStub{
		evidence: &IPRiskCandidateSnapshot{
			Evidence: IPRiskEvidence{
				PrimaryIP:            "203.0.113.8",
				PrimaryNetwork:       "203.0.113.8/32",
				RegistrationCount10m: 3,
			},
		},
	}
	svc := NewIPRiskService(
		repo,
		nil,
		nil,
		NewIPRiskHasher([]byte("unit-test-ip-risk-key")),
		DefaultIPRiskRuntimeConfig(),
	)

	result, err := svc.EvaluateNetwork(context.Background(), "203.0.113.8/32", time.Now().UTC())
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 25, result.Score)
	require.Empty(t, repo.cases)
}

func TestIPRiskServicePersistsInferredCaseWithoutAutoEligibility(t *testing.T) {
	t.Parallel()

	repo := &ipRiskRepositoryStub{
		evidence: &IPRiskCandidateSnapshot{
			Evidence: IPRiskEvidence{
				PrimaryIP:              "203.0.113.8",
				PrimaryNetwork:         "203.0.113.8/32",
				RegistrationCount24h:   10,
				ExactRegistrationCount: 0,
				MaxSharedUACount:       5,
				SharedAPIIPUserCount:   3,
				AllKeyEvidenceExact:    false,
			},
			EvidenceConfidence: string(EvidenceConfidenceInferred),
			Users: []IPRiskRelatedUserSnapshot{{
				UserID:              44,
				RelationType:        IPRiskUserRelationSuspectedNew,
				EvidenceConfidence:  EvidenceConfidenceInferred,
				RecommendedSelected: false,
			}},
		},
	}
	svc := NewIPRiskService(
		repo,
		nil,
		nil,
		NewIPRiskHasher([]byte("unit-test-ip-risk-key")),
		DefaultIPRiskRuntimeConfig(),
	)

	result, err := svc.EvaluateNetwork(
		context.Background(),
		"203.0.113.8/32",
		time.Date(2026, time.July, 23, 8, 10, 0, 0, time.UTC),
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 90, result.Score)
	require.False(t, result.AutoBlockEligible)
	require.Len(t, repo.cases, 1)
	require.Equal(t, string(EvidenceConfidenceInferred), repo.cases[0].EvidenceConfidence)
	require.False(t, repo.cases[0].AutoBlockEligible)
	require.False(t, repo.cases[0].Users[0].RecommendedSelected)
}

func TestIPRiskServiceScanEvaluatesHistoricalClusterAtCandidateTime(t *testing.T) {
	t.Parallel()

	end := time.Date(2026, time.July, 23, 8, 0, 0, 0, time.UTC)
	clusterAt := end.Add(-48 * time.Hour)
	repo := &ipRiskRepositoryStub{
		candidates: []IPRiskEvaluationCandidate{{
			Network:    "203.0.113.8/32",
			DetectedAt: clusterAt,
		}},
		evidence: &IPRiskCandidateSnapshot{
			Evidence: IPRiskEvidence{
				PrimaryIP:                  "203.0.113.8",
				PrimaryNetwork:             "203.0.113.8/32",
				PrimaryIPRegistrationCount: 5,
				RegistrationCount1h:        5,
				RegistrationCount24h:       5,
				ExactRegistrationCount:     5,
				MaxSharedUACount:           5,
				AllKeyEvidenceExact:        true,
			},
			EvidenceConfidence: string(EvidenceConfidenceExact),
		},
	}
	svc := NewIPRiskService(
		repo,
		nil,
		nil,
		NewIPRiskHasher([]byte("unit-test-ip-risk-key")),
		DefaultIPRiskRuntimeConfig(),
	)

	_, err := svc.RunScan(
		context.Background(),
		IPRiskScanManual,
		end.Add(-30*24*time.Hour),
		end,
		nil,
	)
	require.NoError(t, err)
	require.Equal(t, []time.Time{clusterAt}, repo.snapshotTimes)
}

func TestIPRiskServiceCanceledScanKeepsPartialProgress(t *testing.T) {
	t.Parallel()

	end := time.Date(2026, time.July, 23, 8, 0, 0, 0, time.UTC)
	repo := &ipRiskRepositoryStub{
		candidates: []IPRiskEvaluationCandidate{
			{Network: "203.0.113.8/32", DetectedAt: end.Add(-time.Hour)},
			{Network: "203.0.113.9/32", DetectedAt: end},
		},
		evidence: &IPRiskCandidateSnapshot{
			Evidence: IPRiskEvidence{
				PrimaryIP:                  "203.0.113.8",
				PrimaryNetwork:             "203.0.113.8/32",
				PrimaryIPRegistrationCount: 3,
				RegistrationCount10m:       3,
				ExactRegistrationCount:     3,
				AllKeyEvidenceExact:        true,
			},
		},
		cancelOnSnapshot: 2,
	}
	svc := NewIPRiskService(
		repo,
		nil,
		nil,
		NewIPRiskHasher([]byte("unit-test-ip-risk-key")),
		DefaultIPRiskRuntimeConfig(),
	)

	scan, err := svc.RunScan(
		context.Background(),
		IPRiskScanManual,
		end.Add(-24*time.Hour),
		end,
		nil,
	)
	require.ErrorIs(t, err, context.Canceled)
	require.NotNil(t, scan)
	require.Equal(t, IPRiskScanCanceled, scan.Status)
	require.Equal(t, 50, scan.Progress)
	require.Equal(t, 2, scan.CandidateCount)
	require.NotEmpty(t, repo.scanUpdates)
	require.Equal(t, IPRiskScanCanceled, repo.scanUpdates[len(repo.scanUpdates)-1].Status)
	require.Equal(t, 50, repo.scanUpdates[len(repo.scanUpdates)-1].Progress)
}

func TestIPRiskServiceInitialHistoricalBackfillRunsOnlyWhenIncomplete(t *testing.T) {
	t.Parallel()

	runtimeConfig := DefaultIPRiskRuntimeConfig()
	runtimeConfig.HistoricalBackfillEnabled = true

	completedRepo := &ipRiskRepositoryStub{completedScan: true}
	completedService := NewIPRiskService(
		completedRepo,
		nil,
		nil,
		NewIPRiskHasher([]byte("unit-test-ip-risk-key")),
		runtimeConfig,
	)
	completedService.wg.Add(1)
	completedService.runInitialHistoricalBackfill()
	require.Zero(t, completedRepo.scanCreates)

	incompleteRepo := &ipRiskRepositoryStub{}
	incompleteService := NewIPRiskService(
		incompleteRepo,
		nil,
		nil,
		NewIPRiskHasher([]byte("unit-test-ip-risk-key")),
		runtimeConfig,
	)
	incompleteService.wg.Add(1)
	incompleteService.runInitialHistoricalBackfill()
	require.Equal(t, 1, incompleteRepo.scanCreates)
}

func TestIPRiskServiceUsesOneCrossTypeScanLock(t *testing.T) {
	t.Parallel()

	repo := &ipRiskRepositoryStub{}
	lock := &ipRiskLeaderLockStub{}
	svc := NewIPRiskService(
		repo,
		lock,
		nil,
		NewIPRiskHasher([]byte("unit-test-ip-risk-key")),
		DefaultIPRiskRuntimeConfig(),
	)
	end := time.Date(2026, time.July, 23, 8, 0, 0, 0, time.UTC)

	_, err := svc.RunScan(
		context.Background(),
		IPRiskScanManual,
		end.Add(-time.Hour),
		end,
		nil,
	)
	require.NoError(t, err)
	_, err = svc.RunScan(
		context.Background(),
		IPRiskScanDaily,
		end.Add(-24*time.Hour),
		end,
		nil,
	)
	require.NoError(t, err)
	require.Equal(t, []string{"ip-risk:scan", "ip-risk:scan"}, lock.keys)
}

func TestIPRiskServicePersistsCanceledTerminalStateWithDetachedContext(t *testing.T) {
	t.Parallel()

	repo := &ipRiskRepositoryStub{candidateErr: context.Canceled}
	svc := NewIPRiskService(
		repo,
		nil,
		nil,
		NewIPRiskHasher([]byte("unit-test-ip-risk-key")),
		DefaultIPRiskRuntimeConfig(),
	)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	end := time.Date(2026, time.July, 23, 8, 0, 0, 0, time.UTC)

	scan, err := svc.RunScan(
		ctx,
		IPRiskScanManual,
		end.Add(-time.Hour),
		end,
		nil,
	)
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, IPRiskScanCanceled, scan.Status)
	require.NoError(t, repo.terminalUpdateContextErr)
}

func TestIPRiskServiceSuccessfulEmptyScanClearsPriorRuntimeError(t *testing.T) {
	t.Parallel()

	repo := &ipRiskRepositoryStub{}
	svc := NewIPRiskService(
		repo,
		nil,
		nil,
		NewIPRiskHasher([]byte("unit-test-ip-risk-key")),
		DefaultIPRiskRuntimeConfig(),
	)
	svc.setEvaluationState(
		time.Date(2026, time.July, 23, 7, 0, 0, 0, time.UTC),
		"previous queue error",
	)
	end := time.Date(2026, time.July, 23, 8, 0, 0, 0, time.UTC)

	_, err := svc.RunScan(
		context.Background(),
		IPRiskScanManual,
		end.Add(-time.Hour),
		end,
		nil,
	)
	require.NoError(t, err)
	require.Empty(t, svc.Runtime(context.Background()).LastError)
}

func TestIPRiskRuntimeReportsLatestFailedScanAfterRestart(t *testing.T) {
	t.Parallel()

	repo := &ipRiskRepositoryStub{latestScan: &IPRiskScan{
		ID:           9,
		ScanType:     IPRiskScanDaily,
		Status:       IPRiskScanFailed,
		ErrorMessage: "daily scan failed",
	}}
	svc := NewIPRiskService(
		repo,
		nil,
		nil,
		NewIPRiskHasher([]byte("unit-test-ip-risk-key")),
		DefaultIPRiskRuntimeConfig(),
	)
	svc.started.Store(true)

	runtime := svc.Runtime(context.Background())
	require.True(t, runtime.Degraded)
	require.Equal(t, "latest scan failed", runtime.DegradedReason)
	require.Equal(t, "daily scan failed", runtime.LastError)
}

func TestIPRiskRuntimeReportsAndClearsEventPersistenceFailure(t *testing.T) {
	t.Parallel()

	repo := &ipRiskRepositoryStub{insertErr: errors.New("risk event database unavailable")}
	svc := NewIPRiskService(
		repo,
		nil,
		nil,
		NewIPRiskHasher([]byte("unit-test-ip-risk-key")),
		DefaultIPRiskRuntimeConfig(),
	)
	svc.started.Store(true)
	ctx := WithIPRiskRequestMetadata(context.Background(), IPRiskRequestMetadata{
		ClientIP:   "203.0.113.8",
		OccurredAt: time.Date(2026, time.July, 23, 8, 0, 0, 0, time.UTC),
	})

	err := svc.RecordRegistration(ctx, IPRiskRegistrationInput{
		UserID:       42,
		Email:        "event-error@example.test",
		SignupSource: "email",
	})
	require.ErrorContains(t, err, "risk event database unavailable")
	runtime := svc.Runtime(context.Background())
	require.True(t, runtime.Degraded)
	require.Contains(t, runtime.LastError, "risk event database unavailable")

	repo.insertErr = nil
	require.NoError(t, svc.RecordRegistration(ctx, IPRiskRegistrationInput{
		UserID:       43,
		Email:        "event-recovered@example.test",
		SignupSource: "email",
	}))
	runtime = svc.Runtime(context.Background())
	require.False(t, runtime.Degraded)
	require.Empty(t, runtime.LastError)
}

type ipRiskLeaderLockStub struct {
	keys []string
}

func (s *ipRiskLeaderLockStub) TryAcquireLeaderLock(
	_ context.Context,
	key,
	_ string,
	_ time.Duration,
) (bool, error) {
	s.keys = append(s.keys, key)
	return true, nil
}

func (s *ipRiskLeaderLockStub) ReleaseLeaderLock(context.Context, string, string) error {
	return nil
}

type ipRiskRepositoryStub struct {
	events                   []*AuthRiskEvent
	candidates               []IPRiskEvaluationCandidate
	evidence                 *IPRiskCandidateSnapshot
	cases                    []*IPRiskCaseUpsert
	policy                   IPRiskPolicyMatch
	snapshotTimes            []time.Time
	cancelOnSnapshot         int
	scanCreates              int
	scanUpdates              []*IPRiskScanUpdate
	completedScan            bool
	candidateErr             error
	terminalUpdateContextErr error
	latestScan               *IPRiskScan
	insertErr                error
}

func (r *ipRiskRepositoryStub) InsertAuthRiskEvent(_ context.Context, event *AuthRiskEvent) (bool, error) {
	if r.insertErr != nil {
		return false, r.insertErr
	}
	clone := *event
	clone.UserAgentHMAC = append([]byte(nil), event.UserAgentHMAC...)
	clone.EmailPatternHMAC = append([]byte(nil), event.EmailPatternHMAC...)
	clone.InvitationHMAC = append([]byte(nil), event.InvitationHMAC...)
	clone.AffiliateHMAC = append([]byte(nil), event.AffiliateHMAC...)
	r.events = append(r.events, &clone)
	return true, nil
}

func (r *ipRiskRepositoryStub) ListIPRiskEvaluationCandidates(
	context.Context,
	time.Time,
	time.Time,
	IPRiskRegistrationThresholds,
) ([]IPRiskEvaluationCandidate, error) {
	if r.candidateErr != nil {
		return nil, r.candidateErr
	}
	return r.candidates, nil
}

func (r *ipRiskRepositoryStub) LoadIPRiskCandidateSnapshot(_ context.Context, _ string, at time.Time) (*IPRiskCandidateSnapshot, error) {
	r.snapshotTimes = append(r.snapshotTimes, at)
	if r.cancelOnSnapshot > 0 && len(r.snapshotTimes) == r.cancelOnSnapshot {
		return nil, context.Canceled
	}
	return r.evidence, nil
}

func (r *ipRiskRepositoryStub) MatchIPRiskPolicies(context.Context, string, string, time.Time) (IPRiskPolicyMatch, error) {
	return r.policy, nil
}

func (r *ipRiskRepositoryStub) UpsertIPRiskCase(_ context.Context, input *IPRiskCaseUpsert) (*IPRiskCase, error) {
	clone := *input
	clone.Signals = append([]IPRiskSignal(nil), input.Signals...)
	clone.Users = append([]IPRiskRelatedUserSnapshot(nil), input.Users...)
	clone.RecommendedActions = append([]string(nil), input.RecommendedActions...)
	r.cases = append(r.cases, &clone)
	return &IPRiskCase{ID: int64(len(r.cases)), Score: input.Score, Level: input.Level}, nil
}

func (r *ipRiskRepositoryStub) CreateIPRiskScan(context.Context, *IPRiskScanCreate) (*IPRiskScan, error) {
	r.scanCreates++
	return &IPRiskScan{ID: 1}, nil
}

func (r *ipRiskRepositoryStub) UpdateIPRiskScan(ctx context.Context, _ int64, update *IPRiskScanUpdate) error {
	clone := *update
	r.scanUpdates = append(r.scanUpdates, &clone)
	if update.Status == IPRiskScanCanceled || update.Status == IPRiskScanFailed {
		r.terminalUpdateContextErr = ctx.Err()
	}
	return nil
}

func (r *ipRiskRepositoryStub) ListHistoricalRegistrationCandidates(context.Context, time.Time, time.Time, int64, int) (*IPRiskHistoricalPage, error) {
	return &IPRiskHistoricalPage{Done: true}, nil
}

func (r *ipRiskRepositoryStub) DeleteAuthRiskEventsBefore(context.Context, time.Time, int) (int64, error) {
	return 0, nil
}

func (r *ipRiskRepositoryStub) DeleteIPRiskRecordsBefore(context.Context, time.Time, int) (int64, error) {
	return 0, nil
}

func (r *ipRiskRepositoryStub) LatestIPRiskScan(context.Context) (*IPRiskScan, error) {
	if r.latestScan != nil {
		return r.latestScan, nil
	}
	return nil, sql.ErrNoRows
}

func (r *ipRiskRepositoryStub) HasCompletedIPRiskScan(context.Context, IPRiskScanType) (bool, error) {
	return r.completedScan, nil
}
