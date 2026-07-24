package repository

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestIPRiskRepositoryInsertAuthRiskEventIsIdempotent(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := NewIPRiskRepository(db)
	occurredAt := time.Date(2026, time.July, 23, 8, 0, 0, 0, time.UTC)
	event := &service.AuthRiskEvent{
		DedupeKey:            "register:exact:user:42",
		EventType:            service.AuthRiskEventRegister,
		UserID:               42,
		IPAddress:            "203.0.113.8",
		IPNetwork:            "203.0.113.8/32",
		UserAgentSummary:     "mozilla/{v}",
		UserAgentHMAC:        []byte{1, 2, 3},
		EmailPatternHMAC:     []byte{4, 5, 6},
		EmailPatternTemplate: true,
		InvitationHMAC:       []byte{7, 8},
		AffiliateHMAC:        []byte{9, 10},
		SignupSource:         "email",
		RequestID:            "request-1",
		EvidenceConfidence:   service.EvidenceConfidenceExact,
		OccurredAt:           occurredAt,
	}

	mock.ExpectQuery(regexp.QuoteMeta(`
	INSERT INTO auth_risk_events (
	    dedupe_key, event_type, user_id, ip_address, ip_network,
	    user_agent_summary, user_agent_hmac, email_pattern_hmac,
    email_pattern_template, invitation_hmac, affiliate_hmac,
    signup_source, request_id, evidence_confidence, occurred_at
	) VALUES (
	    $1, $2, NULLIF($3, 0), $4::inet, $5::cidr,
	    $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
	)
	ON CONFLICT (user_id)
	    WHERE event_type = 'register' AND user_id IS NOT NULL
	DO UPDATE SET
	    dedupe_key = EXCLUDED.dedupe_key,
	    ip_address = EXCLUDED.ip_address,
	    ip_network = EXCLUDED.ip_network,
	    user_agent_summary = EXCLUDED.user_agent_summary,
	    user_agent_hmac = EXCLUDED.user_agent_hmac,
	    email_pattern_hmac = EXCLUDED.email_pattern_hmac,
	    email_pattern_template = EXCLUDED.email_pattern_template,
	    invitation_hmac = EXCLUDED.invitation_hmac,
	    affiliate_hmac = EXCLUDED.affiliate_hmac,
	    signup_source = EXCLUDED.signup_source,
	    request_id = EXCLUDED.request_id,
	    evidence_confidence = EXCLUDED.evidence_confidence,
	    occurred_at = EXCLUDED.occurred_at,
	    created_at = NOW()
	WHERE auth_risk_events.evidence_confidence = 'inferred'
	  AND EXCLUDED.evidence_confidence = 'exact'
	RETURNING id`)).
		WithArgs(
			event.DedupeKey,
			string(event.EventType),
			event.UserID,
			event.IPAddress,
			event.IPNetwork,
			event.UserAgentSummary,
			event.UserAgentHMAC,
			event.EmailPatternHMAC,
			event.EmailPatternTemplate,
			event.InvitationHMAC,
			event.AffiliateHMAC,
			event.SignupSource,
			event.RequestID,
			string(event.EvidenceConfidence),
			occurredAt,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(99)))

	inserted, err := repo.InsertAuthRiskEvent(context.Background(), event)
	require.NoError(t, err)
	require.True(t, inserted)

	mock.ExpectQuery("INSERT INTO auth_risk_events").
		WillReturnError(sql.ErrNoRows)
	inserted, err = repo.InsertAuthRiskEvent(context.Background(), event)
	require.NoError(t, err)
	require.False(t, inserted)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIPRiskRepositoryCandidateQueryUsesBoundedSlidingWindows(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := NewIPRiskRepository(db)
	start := time.Date(2026, time.July, 22, 8, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	detectedAt := end.Add(-2 * time.Hour)
	mock.ExpectQuery(`WITH registration_windows AS`).
		WithArgs(start, end, 3, 5, 10).
		WillReturnRows(sqlmock.NewRows([]string{"ip_network", "occurred_at"}).
			AddRow("203.0.113.8/32", detectedAt))

	candidates, err := repo.ListIPRiskEvaluationCandidates(
		context.Background(),
		start,
		end,
		service.IPRiskRegistrationThresholds{
			TenMinutes:      3,
			OneHour:         5,
			TwentyFourHours: 10,
		},
	)
	require.NoError(t, err)
	require.Equal(t, []service.IPRiskEvaluationCandidate{{
		Network:    "203.0.113.8/32",
		DetectedAt: detectedAt,
	}}, candidates)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIPRiskRepositoryChecksCompletedHistoricalBackfill(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := NewIPRiskRepository(db)

	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs(string(service.IPRiskScanHistoricalBackfill)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	completed, err := repo.HasCompletedIPRiskScan(
		context.Background(),
		service.IPRiskScanHistoricalBackfill,
	)
	require.NoError(t, err)
	require.True(t, completed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIPRiskRepositoryCreateScanPinsStatusParameterType(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := NewIPRiskRepository(db)
	now := time.Date(2026, time.July, 23, 14, 0, 0, 0, time.UTC)
	adminID := int64(42)

	mock.ExpectQuery(`(?s)INSERT INTO ip_risk_scans.*\$2::VARCHAR.*CASE WHEN \$2::VARCHAR = 'running'`).
		WithArgs(
			string(service.IPRiskScanManual),
			string(service.IPRiskScanPending),
			&adminID,
			now.Add(-24*time.Hour),
			now,
		).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "scan_type", "status", "requested_by", "range_start", "range_end",
			"progress", "candidate_count", "case_count", "inferred_event_count",
			"error_message", "started_at", "completed_at", "created_at", "updated_at",
		}).AddRow(
			int64(7), string(service.IPRiskScanManual), string(service.IPRiskScanPending),
			adminID, now.Add(-24*time.Hour), now,
			0, 0, 0, 0, "", nil, nil, now, now,
		))

	scan, err := repo.CreateIPRiskScan(context.Background(), &service.IPRiskScanCreate{
		ScanType:    service.IPRiskScanManual,
		Status:      service.IPRiskScanPending,
		RequestedBy: &adminID,
		RangeStart:  now.Add(-24 * time.Hour),
		RangeEnd:    now,
	})
	require.NoError(t, err)
	require.Equal(t, int64(7), scan.ID)
	require.Equal(t, service.IPRiskScanPending, scan.Status)
	require.NoError(t, mock.ExpectationsWereMet())
}
