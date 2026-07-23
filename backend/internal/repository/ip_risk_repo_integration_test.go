//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestIPRiskRepositoryPostgreSQLAggregationAndCaseUpsert(t *testing.T) {
	ctx := context.Background()
	repo := NewIPRiskRepository(integrationDB)
	now := time.Now().UTC().Truncate(time.Second)
	network := "203.0.113.8/32"
	users := make([]*service.User, 0, 5)

	for index := 0; index < 5; index++ {
		user := mustCreateUser(t, integrationEntClient, &service.User{
			Email: fmt.Sprintf("ip-risk-%d-%d@example.test", now.UnixNano(), index),
		})
		users = append(users, user)
		inserted, err := repo.InsertAuthRiskEvent(ctx, &service.AuthRiskEvent{
			DedupeKey:            fmt.Sprintf("integration:register:%d", user.ID),
			EventType:            service.AuthRiskEventRegister,
			UserID:               user.ID,
			IPAddress:            "203.0.113.8",
			IPNetwork:            network,
			UserAgentSummary:     "mozilla/{v}",
			UserAgentHMAC:        []byte("shared-ua"),
			EmailPatternHMAC:     []byte("shared-email-pattern"),
			EmailPatternTemplate: true,
			SignupSource:         "email",
			RequestID:            fmt.Sprintf("risk-request-%d", index),
			EvidenceConfidence:   service.EvidenceConfidenceExact,
			OccurredAt:           now.Add(-time.Duration(index) * time.Minute),
		})
		require.NoError(t, err)
		require.True(t, inserted)
	}
	deletedKey := mustCreateApiKey(t, integrationEntClient, &service.APIKey{UserID: users[4].ID})
	_, err := integrationDB.ExecContext(ctx, `
UPDATE api_keys
SET created_at = $2, deleted_at = $3
WHERE id = $1`,
		deletedKey.ID,
		now.Add(-3*time.Minute),
		now.Add(-2*time.Minute),
	)
	require.NoError(t, err)

	candidates, err := repo.ListIPRiskEvaluationCandidates(
		ctx,
		now.Add(-time.Hour),
		now,
		service.IPRiskRegistrationThresholds{
			TenMinutes:      3,
			OneHour:         5,
			TwentyFourHours: 10,
		},
	)
	require.NoError(t, err)
	require.Contains(t, candidates, service.IPRiskEvaluationCandidate{
		Network:    network,
		DetectedAt: now,
	})

	snapshot, err := repo.LoadIPRiskCandidateSnapshot(ctx, network, now)
	require.NoError(t, err)
	require.NotNil(t, snapshot)
	require.Equal(t, "203.0.113.8", snapshot.Evidence.PrimaryIP)
	require.Equal(t, 5, snapshot.Evidence.PrimaryIPRegistrationCount)
	require.Equal(t, 5, snapshot.Evidence.RegistrationCount10m)
	require.Equal(t, 5, snapshot.Evidence.RegistrationCount1h)
	require.Equal(t, 5, snapshot.Evidence.RegistrationCount24h)
	require.Equal(t, 5, snapshot.Evidence.ExactRegistrationCount)
	require.Equal(t, 5, snapshot.Evidence.MaxSharedUACount)
	require.Equal(t, 5, snapshot.Evidence.EmailPatternAccountCount)
	require.Equal(t, 1, snapshot.Evidence.RapidKeyOrGiftUserCount)
	require.True(t, snapshot.Evidence.AllKeyEvidenceExact)
	require.Len(t, snapshot.Users, 5)

	policyID := int64(0)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
INSERT INTO ip_risk_policies (mode, ip_network, reason)
VALUES ('shared_network', $1::cidr, 'integration test')
RETURNING id`,
		network,
	).Scan(&policyID))
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM ip_risk_policies WHERE id = $1`, policyID)
	})
	match, err := repo.MatchIPRiskPolicies(ctx, "203.0.113.8", network, now)
	require.NoError(t, err)
	require.True(t, match.KnownSharedNetwork)

	assessment := service.CalculateIPRiskAssessment(service.DefaultIPRiskConfig(), snapshot.Evidence)
	riskCase, err := repo.UpsertIPRiskCase(ctx, &service.IPRiskCaseUpsert{
		PrimaryIP:          snapshot.Evidence.PrimaryIP,
		PrimaryNetwork:     snapshot.Evidence.PrimaryNetwork,
		Score:              assessment.Score,
		Level:              assessment.Level,
		EvidenceConfidence: snapshot.EvidenceConfidence,
		Signals:            assessment.Signals,
		Evidence:           snapshot.Evidence,
		Users:              snapshot.Users,
		RecommendedActions: []string{"review_related_users"},
		AutoBlockEligible:  assessment.AutoBlockEligible,
		ShadowMode:         true,
		DetectedAt:         now,
	})
	require.NoError(t, err)
	require.NotZero(t, riskCase.ID)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM ip_risk_cases WHERE id = $1`, riskCase.ID)
	})

	var relatedCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
SELECT COUNT(*) FROM ip_risk_case_users WHERE case_id = $1`,
		riskCase.ID,
	).Scan(&relatedCount))
	require.Equal(t, 5, relatedCount)
}

func TestIPRiskRepositoryHistoricalInferenceMatchesEmailRegistrationOnly(t *testing.T) {
	ctx := context.Background()
	repo := NewIPRiskRepository(integrationDB)
	now := time.Now().UTC().Truncate(time.Second)
	user := mustCreateUser(t, integrationEntClient, &service.User{
		Email:     fmt.Sprintf("ip-risk-history-%d@example.test", now.UnixNano()),
		CreatedAt: now,
	})

	var auditID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
INSERT INTO audit_logs (
    created_at, action, method, path, request_id, client_ip,
    user_agent, request_body, status_code
) VALUES (
    $1, 'auth.register', 'POST', '/api/v1/auth/register', 'history-request',
    '198.51.100.42', 'history-agent/1.0', $2, 200
)
RETURNING id`,
		now,
		fmt.Sprintf(`{"email":%q,"password":"***"}`, user.Email),
	).Scan(&auditID))
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM audit_logs WHERE id = $1`, auditID)
	})
	var redirectAuditID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
INSERT INTO audit_logs (
    created_at, action, method, path, request_id, client_ip,
    user_agent, request_body, status_code
) VALUES (
    $1, 'auth.register', 'POST', '/api/v1/auth/register', 'history-redirect',
    '198.51.100.43', 'history-agent/1.0', $2, 302
)
RETURNING id`,
		now,
		fmt.Sprintf(`{"email":%q,"password":"***"}`, user.Email),
	).Scan(&redirectAuditID))
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM audit_logs WHERE id = $1`, redirectAuditID)
	})
	mobileUser := mustCreateUser(t, integrationEntClient, &service.User{
		Email:     fmt.Sprintf("ip-risk-mobile-history-%d@example.test", now.UnixNano()),
		CreatedAt: now,
	})
	var mobileAuditID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
INSERT INTO audit_logs (
    created_at, action, method, path, request_id, client_ip,
    user_agent, request_body, status_code
) VALUES (
    $1, 'auth.mobile.register', 'POST', '/api/v1/auth/mobile/register', 'history-mobile',
    '198.51.100.44', 'history-mobile-agent/1.0', $2, 200
)
RETURNING id`,
		now,
		fmt.Sprintf(`{"email":%q,"password":"***"}`, mobileUser.Email),
	).Scan(&mobileAuditID))
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM audit_logs WHERE id = $1`, mobileAuditID)
	})

	page, err := repo.ListHistoricalRegistrationCandidates(
		ctx,
		now.Add(-time.Minute),
		now.Add(time.Minute),
		auditID-1,
		100,
	)
	require.NoError(t, err)
	require.Len(t, page.Candidates, 2)
	require.Equal(t, auditID, page.Candidates[0].AuditID)
	require.Equal(t, user.ID, page.Candidates[0].UserID)
	require.Equal(t, "198.51.100.42", page.Candidates[0].ClientIP)
	require.Equal(t, mobileAuditID, page.Candidates[1].AuditID)
	require.Equal(t, mobileUser.ID, page.Candidates[1].UserID)
	require.Equal(t, "198.51.100.44", page.Candidates[1].ClientIP)
}

func TestIPRiskRepositorySharedAPIIPCountsOnlyUsersRegisteredOnCandidateNetwork(t *testing.T) {
	ctx := context.Background()
	repo := NewIPRiskRepository(integrationDB)
	now := time.Now().UTC().Truncate(time.Second)
	network := "198.51.100.90/32"

	for index := 0; index < 3; index++ {
		user := mustCreateUser(t, integrationEntClient, &service.User{
			Email: fmt.Sprintf("ip-risk-api-new-%d-%d@example.test", now.UnixNano(), index),
		})
		inserted, err := repo.InsertAuthRiskEvent(ctx, &service.AuthRiskEvent{
			DedupeKey:          fmt.Sprintf("integration:api-register:%d", user.ID),
			EventType:          service.AuthRiskEventRegister,
			UserID:             user.ID,
			IPAddress:          "198.51.100.90",
			IPNetwork:          network,
			SignupSource:       "email",
			EvidenceConfidence: service.EvidenceConfidenceExact,
			OccurredAt:         now.Add(-time.Duration(index) * time.Minute),
		})
		require.NoError(t, err)
		require.True(t, inserted)
	}

	unrelated := mustCreateUser(t, integrationEntClient, &service.User{
		Email: fmt.Sprintf("ip-risk-api-unrelated-%d@example.test", now.UnixNano()),
	})
	account := mustCreateAccount(t, integrationEntClient, &service.Account{Name: "ip-risk-api-unrelated"})
	apiKey := mustCreateApiKey(t, integrationEntClient, &service.APIKey{UserID: unrelated.ID})
	_, err := integrationDB.ExecContext(ctx, `
INSERT INTO usage_logs (
    user_id, api_key_id, account_id, model, input_tokens, output_tokens,
    total_cost, actual_cost, ip_address, created_at
) VALUES ($1, $2, $3, 'ip-risk-test', 1, 1, 0.01, 0.01, '198.51.100.90', $4)`,
		unrelated.ID,
		apiKey.ID,
		account.ID,
		now,
	)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `
INSERT INTO usage_logs (
    user_id, api_key_id, account_id, model, input_tokens, output_tokens,
    total_cost, actual_cost, ip_address, created_at
) VALUES ($1, $2, $3, 'ip-risk-invalid-ip', 1, 1, 0.01, 0.01, 'not-an-ip', $4)`,
		unrelated.ID,
		apiKey.ID,
		account.ID,
		now,
	)
	require.NoError(t, err)

	snapshot, err := repo.LoadIPRiskCandidateSnapshot(ctx, network, now)
	require.NoError(t, err)
	require.NotNil(t, snapshot)
	require.Zero(t, snapshot.Evidence.SharedAPIIPUserCount)
}

func TestIPRiskRepositoryDeduplicatesInferredRegistrationPerUser(t *testing.T) {
	ctx := context.Background()
	repo := NewIPRiskRepository(integrationDB)
	now := time.Now().UTC().Truncate(time.Second)
	user := mustCreateUser(t, integrationEntClient, &service.User{
		Email: fmt.Sprintf("ip-risk-inferred-dedupe-%d@example.test", now.UnixNano()),
	})

	first := &service.AuthRiskEvent{
		DedupeKey:          fmt.Sprintf("integration:inferred:first:%d", user.ID),
		EventType:          service.AuthRiskEventRegister,
		UserID:             user.ID,
		IPAddress:          "203.0.113.77",
		IPNetwork:          "203.0.113.77/32",
		SignupSource:       "email",
		EvidenceConfidence: service.EvidenceConfidenceInferred,
		OccurredAt:         now,
	}
	inserted, err := repo.InsertAuthRiskEvent(ctx, first)
	require.NoError(t, err)
	require.True(t, inserted)

	second := *first
	second.DedupeKey = fmt.Sprintf("integration:inferred:second:%d", user.ID)
	second.OccurredAt = now.Add(time.Second)
	inserted, err = repo.InsertAuthRiskEvent(ctx, &second)
	require.NoError(t, err)
	require.False(t, inserted)

	exact := second
	exact.DedupeKey = fmt.Sprintf("integration:exact:%d", user.ID)
	exact.EvidenceConfidence = service.EvidenceConfidenceExact
	exact.IPAddress = "203.0.113.78"
	exact.IPNetwork = "203.0.113.78/32"
	inserted, err = repo.InsertAuthRiskEvent(ctx, &exact)
	require.NoError(t, err)
	require.True(t, inserted, "exact evidence must atomically replace inferred evidence")

	var (
		dedupeKey  string
		confidence string
		ipAddress  string
	)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
SELECT dedupe_key, evidence_confidence, host(ip_address)
FROM auth_risk_events
WHERE event_type = 'register' AND user_id = $1`,
		user.ID,
	).Scan(&dedupeKey, &confidence, &ipAddress))
	require.Equal(t, exact.DedupeKey, dedupeKey)
	require.Equal(t, string(service.EvidenceConfidenceExact), confidence)
	require.Equal(t, exact.IPAddress, ipAddress)

	third := *first
	third.DedupeKey = fmt.Sprintf("integration:inferred:third:%d", user.ID)
	inserted, err = repo.InsertAuthRiskEvent(ctx, &third)
	require.NoError(t, err)
	require.False(t, inserted, "historical inference must not replace exact evidence")
}
