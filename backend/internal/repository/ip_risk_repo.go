package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type ipRiskRepository struct {
	db *sql.DB
}

func NewIPRiskRepository(db *sql.DB) service.IPRiskRepository {
	return &ipRiskRepository{db: db}
}

const insertAuthRiskEventSQL = `
INSERT INTO auth_risk_events (
    dedupe_key, event_type, user_id, ip_address, ip_network,
    user_agent_summary, user_agent_hmac, email_pattern_hmac,
    email_pattern_template, invitation_hmac, affiliate_hmac,
    signup_source, request_id, evidence_confidence, occurred_at
) VALUES (
    $1, $2, NULLIF($3, 0), $4::inet, $5::cidr,
    $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
)
ON CONFLICT DO NOTHING
RETURNING id`

const upsertRegistrationRiskEventSQL = `
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
RETURNING id`

func (r *ipRiskRepository) InsertAuthRiskEvent(ctx context.Context, event *service.AuthRiskEvent) (bool, error) {
	if r == nil || r.db == nil {
		return false, errors.New("nil ip risk repository")
	}
	if event == nil {
		return false, errors.New("nil auth risk event")
	}
	statement := insertAuthRiskEventSQL
	if event.EventType == service.AuthRiskEventRegister {
		statement = upsertRegistrationRiskEventSQL
	}
	var id int64
	err := r.db.QueryRowContext(
		ctx,
		statement,
		event.DedupeKey,
		string(event.EventType),
		event.UserID,
		event.IPAddress,
		event.IPNetwork,
		event.UserAgentSummary,
		nullableBytes(event.UserAgentHMAC),
		nullableBytes(event.EmailPatternHMAC),
		event.EmailPatternTemplate,
		nullableBytes(event.InvitationHMAC),
		nullableBytes(event.AffiliateHMAC),
		event.SignupSource,
		event.RequestID,
		string(event.EvidenceConfidence),
		event.OccurredAt.UTC(),
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	event.ID = id
	return true, nil
}

func (r *ipRiskRepository) ListIPRiskEvaluationCandidates(
	ctx context.Context,
	start,
	end time.Time,
	thresholds service.IPRiskRegistrationThresholds,
) ([]service.IPRiskEvaluationCandidate, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("nil ip risk repository")
	}
	if thresholds.TenMinutes <= 0 {
		thresholds.TenMinutes = 3
	}
	if thresholds.OneHour <= 0 {
		thresholds.OneHour = 5
	}
	if thresholds.TwentyFourHours <= 0 {
		thresholds.TwentyFourHours = 10
	}
	rows, err := r.db.QueryContext(ctx, `
WITH registration_windows AS (
    SELECT
        ip_network,
        occurred_at,
        COUNT(*) OVER (
            PARTITION BY ip_network
            ORDER BY occurred_at
            RANGE BETWEEN INTERVAL '10 minutes' PRECEDING AND CURRENT ROW
        ) AS registration_count_10m,
        COUNT(*) OVER (
            PARTITION BY ip_network
            ORDER BY occurred_at
            RANGE BETWEEN INTERVAL '1 hour' PRECEDING AND CURRENT ROW
        ) AS registration_count_1h,
        COUNT(*) OVER (
            PARTITION BY ip_network
            ORDER BY occurred_at
            RANGE BETWEEN INTERVAL '24 hours' PRECEDING AND CURRENT ROW
        ) AS registration_count_24h
    FROM auth_risk_events
    WHERE event_type = 'register'
      AND occurred_at >= $1::timestamptz - INTERVAL '24 hours'
      AND occurred_at <= $2
),
qualified AS (
    SELECT
        ip_network,
        occurred_at,
        ROW_NUMBER() OVER (
            PARTITION BY ip_network
            ORDER BY occurred_at DESC
        ) AS candidate_rank
    FROM registration_windows
    WHERE occurred_at >= $1
      AND (
          registration_count_10m >= $3
          OR registration_count_1h >= $4
          OR registration_count_24h >= $5
      )
)
SELECT ip_network::text, occurred_at
FROM qualified
WHERE candidate_rank = 1
ORDER BY occurred_at DESC, ip_network::text ASC`,
		start.UTC(),
		end.UTC(),
		thresholds.TenMinutes,
		thresholds.OneHour,
		thresholds.TwentyFourHours,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	candidates := make([]service.IPRiskEvaluationCandidate, 0)
	for rows.Next() {
		var candidate service.IPRiskEvaluationCandidate
		if err := rows.Scan(&candidate.Network, &candidate.DetectedAt); err != nil {
			return nil, err
		}
		candidate.DetectedAt = candidate.DetectedAt.UTC()
		candidates = append(candidates, candidate)
	}
	return candidates, rows.Err()
}

func (r *ipRiskRepository) LoadIPRiskCandidateSnapshot(
	ctx context.Context,
	network string,
	at time.Time,
) (*service.IPRiskCandidateSnapshot, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("nil ip risk repository")
	}
	at = at.UTC()
	windowStart := at.Add(-24 * time.Hour)
	evidence := service.IPRiskEvidence{
		PrimaryNetwork: strings.TrimSpace(network),
	}
	var allExact bool
	err := r.db.QueryRowContext(ctx, `
WITH registrations AS (
    SELECT ip_address, evidence_confidence, occurred_at
    FROM auth_risk_events
    WHERE event_type = 'register'
      AND ip_network = $1::cidr
      AND occurred_at >= $2
      AND occurred_at <= $3
),
primary_address AS (
    SELECT ip_address, COUNT(*) AS registration_count
    FROM registrations
    GROUP BY ip_address
    ORDER BY registration_count DESC, MAX(occurred_at) DESC, ip_address::text ASC
    LIMIT 1
)
SELECT
    COALESCE((SELECT host(ip_address) FROM primary_address), ''),
    COALESCE((SELECT registration_count FROM primary_address), 0),
    COUNT(*) FILTER (WHERE occurred_at >= $3 - INTERVAL '10 minutes'),
    COUNT(*) FILTER (WHERE occurred_at >= $3 - INTERVAL '1 hour'),
    COUNT(*),
    COUNT(*) FILTER (WHERE evidence_confidence = 'exact'),
    COALESCE(BOOL_AND(evidence_confidence = 'exact'), FALSE)
FROM registrations`,
		network,
		windowStart,
		at,
	).Scan(
		&evidence.PrimaryIP,
		&evidence.PrimaryIPRegistrationCount,
		&evidence.RegistrationCount10m,
		&evidence.RegistrationCount1h,
		&evidence.RegistrationCount24h,
		&evidence.ExactRegistrationCount,
		&allExact,
	)
	if err != nil {
		return nil, err
	}
	if evidence.RegistrationCount24h == 0 {
		return nil, nil
	}
	evidence.AllKeyEvidenceExact = allExact

	if err := r.db.QueryRowContext(ctx, `
SELECT COALESCE(MAX(shared_count), 0)
FROM (
    SELECT COUNT(DISTINCT user_id) AS shared_count
    FROM auth_risk_events
    WHERE event_type = 'register'
      AND ip_network = $1::cidr
      AND occurred_at >= $2
      AND occurred_at <= $3
      AND user_agent_hmac IS NOT NULL
      AND user_id IS NOT NULL
    GROUP BY user_agent_hmac
) shared_ua`,
		network,
		windowStart,
		at,
	).Scan(&evidence.MaxSharedUACount); err != nil {
		return nil, err
	}

	if err := r.db.QueryRowContext(ctx, `
SELECT COALESCE(MAX(pattern_count), 0)
FROM (
    SELECT COUNT(DISTINCT user_id) AS pattern_count
    FROM auth_risk_events
    WHERE event_type = 'register'
      AND ip_network = $1::cidr
      AND occurred_at >= $2
      AND occurred_at <= $3
      AND email_pattern_template = TRUE
      AND email_pattern_hmac IS NOT NULL
      AND user_id IS NOT NULL
    GROUP BY email_pattern_hmac
) email_patterns`,
		network,
		windowStart,
		at,
	).Scan(&evidence.EmailPatternAccountCount); err != nil {
		return nil, err
	}

	if err := r.db.QueryRowContext(ctx, `
SELECT COALESCE(MAX(shared_count), 0)
FROM (
    SELECT COUNT(DISTINCT user_id) AS shared_count
    FROM (
        SELECT user_id, invitation_hmac AS code_hmac
        FROM auth_risk_events
        WHERE event_type = 'register'
          AND ip_network = $1::cidr
          AND occurred_at >= $2
          AND occurred_at <= $3
          AND invitation_hmac IS NOT NULL
        UNION ALL
        SELECT user_id, affiliate_hmac AS code_hmac
        FROM auth_risk_events
        WHERE event_type = 'register'
          AND ip_network = $1::cidr
          AND occurred_at >= $2
          AND occurred_at <= $3
          AND affiliate_hmac IS NOT NULL
    ) signup_codes
    WHERE user_id IS NOT NULL
    GROUP BY code_hmac
) shared_codes`,
		network,
		windowStart,
		at,
	).Scan(&evidence.SharedSignupCodeCount); err != nil {
		return nil, err
	}

	if err := r.db.QueryRowContext(ctx, `
WITH registrations AS (
    SELECT user_id, MIN(occurred_at) AS registered_at
    FROM auth_risk_events
    WHERE event_type = 'register'
      AND ip_network = $1::cidr
      AND occurred_at >= $2
      AND occurred_at <= $3
      AND user_id IS NOT NULL
    GROUP BY user_id
)
SELECT COUNT(DISTINCT ul.user_id)
FROM usage_logs ul
JOIN registrations r ON r.user_id = ul.user_id
WHERE ul.created_at >= r.registered_at
  AND ul.created_at <= $3
  AND NULLIF(BTRIM(ul.ip_address), '') IS NOT NULL
	  AND ip_risk_try_parse_inet(BTRIM(ul.ip_address)) <<= $1::cidr`,
		network,
		windowStart,
		at,
	).Scan(&evidence.SharedAPIIPUserCount); err != nil {
		return nil, err
	}

	if err := r.db.QueryRowContext(ctx, `
WITH registrations AS (
    SELECT user_id, MIN(occurred_at) AS registered_at
    FROM auth_risk_events
    WHERE event_type = 'register'
      AND ip_network = $1::cidr
      AND occurred_at >= $2
      AND occurred_at <= $3
      AND user_id IS NOT NULL
    GROUP BY user_id
)
SELECT COUNT(*)
FROM registrations r
WHERE EXISTS (
	    SELECT 1
	    FROM api_keys k
	    WHERE k.user_id = r.user_id
	      AND k.created_at >= r.registered_at
	      AND k.created_at <= r.registered_at + INTERVAL '30 minutes'
)
	OR EXISTS (
	    SELECT 1
	    FROM balance_fund_allocations allocation
	    JOIN balance_fund_batches batch ON batch.id = allocation.batch_id
	    WHERE allocation.user_id = r.user_id
	      AND allocation.action = 'consume'
	      AND batch.source_kind IN (
	          'signup_gift', 'ops_gift', 'compensation',
	          'redeem_gift', 'promotion_gift', 'unknown'
	      )
	      AND allocation.created_at >= r.registered_at
	      AND allocation.created_at <= r.registered_at + INTERVAL '30 minutes'
	)`,
		network,
		windowStart,
		at,
	).Scan(&evidence.RapidKeyOrGiftUserCount); err != nil {
		return nil, err
	}

	if err := r.db.QueryRowContext(ctx, `
WITH seen_users AS (
    SELECT DISTINCT user_id
    FROM auth_risk_events
    WHERE event_type = 'successful_login'
      AND ip_network = $1::cidr
      AND occurred_at >= $2
      AND occurred_at <= $3
      AND user_id IS NOT NULL
    UNION
    SELECT DISTINCT user_id
    FROM usage_logs
    WHERE created_at >= $2
      AND created_at <= $3
      AND NULLIF(BTRIM(ip_address), '') IS NOT NULL
	      AND ip_risk_try_parse_inet(BTRIM(ip_address)) <<= $1::cidr
)
SELECT COUNT(*)
FROM seen_users s
JOIN users u ON u.id = s.user_id
WHERE u.deleted_at IS NULL
  AND (COALESCE(u.total_recharged, 0) > 0 OR u.created_at < $3 - INTERVAL '30 days')`,
		network,
		windowStart,
		at,
	).Scan(&evidence.TrustedAccountCount); err != nil {
		return nil, err
	}

	users, err := r.loadIPRiskRelatedUsers(ctx, network, evidence.PrimaryIP, windowStart, at)
	if err != nil {
		return nil, err
	}
	confidence := string(service.EvidenceConfidenceInferred)
	switch {
	case evidence.AllKeyEvidenceExact:
		confidence = string(service.EvidenceConfidenceExact)
	case evidence.ExactRegistrationCount > 0:
		confidence = "mixed"
	}
	return &service.IPRiskCandidateSnapshot{
		Evidence:           evidence,
		EvidenceConfidence: confidence,
		Users:              users,
	}, nil
}

func (r *ipRiskRepository) loadIPRiskRelatedUsers(
	ctx context.Context,
	network string,
	primaryIP string,
	start,
	end time.Time,
) ([]service.IPRiskRelatedUserSnapshot, error) {
	rows, err := r.db.QueryContext(ctx, `
WITH registration_events AS (
    SELECT user_id, ip_address, user_agent_hmac, evidence_confidence, occurred_at
    FROM auth_risk_events
    WHERE event_type = 'register'
      AND ip_network = $1::cidr
      AND occurred_at >= $2
      AND occurred_at <= $3
      AND user_id IS NOT NULL
),
shared_ua AS (
    SELECT user_agent_hmac, COUNT(DISTINCT user_id) AS account_count
    FROM registration_events
    WHERE user_agent_hmac IS NOT NULL
    GROUP BY user_agent_hmac
),
registrations AS (
    SELECT
        event.user_id,
        MIN(event.occurred_at) AS first_seen_at,
        MAX(event.occurred_at) AS last_seen_at,
        BOOL_OR(event.evidence_confidence = 'exact') AS has_exact,
        COUNT(*) AS registration_count,
        COUNT(*) FILTER (WHERE event.ip_address = $4::inet) AS primary_ip_registration_count,
        COALESCE(MAX(shared.account_count), 0) AS shared_ua_account_count
    FROM registration_events event
    LEFT JOIN shared_ua shared ON shared.user_agent_hmac = event.user_agent_hmac
    GROUP BY event.user_id
),
trusted_seen AS (
    SELECT
        seen.user_id,
        MIN(seen.seen_at) AS first_seen_at,
        MAX(seen.seen_at) AS last_seen_at
    FROM (
        SELECT user_id, occurred_at AS seen_at
        FROM auth_risk_events
        WHERE event_type = 'successful_login'
          AND ip_network = $1::cidr
          AND occurred_at >= $2
          AND occurred_at <= $3
          AND user_id IS NOT NULL
        UNION ALL
        SELECT user_id, created_at AS seen_at
        FROM usage_logs
        WHERE created_at >= $2
          AND created_at <= $3
          AND NULLIF(BTRIM(ip_address), '') IS NOT NULL
	          AND ip_risk_try_parse_inet(BTRIM(ip_address)) <<= $1::cidr
    ) seen
    GROUP BY seen.user_id
)
SELECT
    u.id,
    CASE
        WHEN u.status = 'disabled' THEN 'disabled'
        WHEN COALESCE(u.total_recharged, 0) > 0 OR u.created_at < $3 - INTERVAL '30 days'
            THEN 'trusted_existing'
        ELSE 'suspected_new'
    END AS relation_type,
    CASE WHEN r.has_exact THEN 'exact' ELSE 'inferred' END AS evidence_confidence,
    CASE
        WHEN u.role = 'admin' THEN FALSE
        WHEN u.status <> 'active' THEN FALSE
        WHEN COALESCE(u.total_recharged, 0) > 0 OR u.created_at < $3 - INTERVAL '30 days' THEN FALSE
        WHEN r.has_exact THEN TRUE
        ELSE FALSE
    END AS recommended_selected,
    COALESCE(r.first_seen_at, ts.first_seen_at),
    COALESCE(r.last_seen_at, ts.last_seen_at),
	    COALESCE(r.registration_count, 0),
	    COALESCE(r.primary_ip_registration_count, 0),
	    COALESCE(r.shared_ua_account_count, 0),
	    u.role,
    u.status,
    COALESCE(u.total_recharged, 0)::double precision,
    u.created_at
FROM users u
LEFT JOIN registrations r ON r.user_id = u.id
LEFT JOIN trusted_seen ts ON ts.user_id = u.id
WHERE u.deleted_at IS NULL
  AND (r.user_id IS NOT NULL OR (
      ts.user_id IS NOT NULL
      AND (COALESCE(u.total_recharged, 0) > 0 OR u.created_at < $3 - INTERVAL '30 days')
  ))
ORDER BY recommended_selected DESC, COALESCE(r.last_seen_at, ts.last_seen_at) DESC, u.id ASC`,
		network,
		start.UTC(),
		end.UTC(),
		primaryIP,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	users := make([]service.IPRiskRelatedUserSnapshot, 0)
	for rows.Next() {
		var (
			item              service.IPRiskRelatedUserSnapshot
			relationType      string
			confidence        string
			registrationCount int
			primaryIPCount    int
			sharedUACount     int
			role              string
			status            string
			totalRecharged    float64
			createdAt         time.Time
		)
		if err := rows.Scan(
			&item.UserID,
			&relationType,
			&confidence,
			&item.RecommendedSelected,
			&item.FirstSeenAt,
			&item.LastSeenAt,
			&registrationCount,
			&primaryIPCount,
			&sharedUACount,
			&role,
			&status,
			&totalRecharged,
			&createdAt,
		); err != nil {
			return nil, err
		}
		item.RelationType = service.IPRiskUserRelation(relationType)
		item.EvidenceConfidence = service.EvidenceConfidence(confidence)
		item.Evidence = map[string]any{
			"registration_count":            registrationCount,
			"primary_ip_registration_count": primaryIPCount,
			"shared_ua_account_count":       sharedUACount,
			"role":                          role,
			"status":                        status,
			"total_recharged":               totalRecharged,
			"account_created_at":            createdAt.UTC(),
		}
		users = append(users, item)
	}
	return users, rows.Err()
}

func (r *ipRiskRepository) MatchIPRiskPolicies(
	ctx context.Context,
	exactIP,
	network string,
	at time.Time,
) (service.IPRiskPolicyMatch, error) {
	if r == nil || r.db == nil {
		return service.IPRiskPolicyMatch{}, errors.New("nil ip risk repository")
	}
	var match service.IPRiskPolicyMatch
	err := r.db.QueryRowContext(ctx, `
SELECT
    COALESCE(BOOL_OR(mode = 'allowlist'), FALSE),
    COALESCE(BOOL_OR(mode = 'shared_network'), FALSE),
    COALESCE(BOOL_OR(mode = 'observe'), FALSE),
    COALESCE(BOOL_OR(mode = 'block_registration'), FALSE)
FROM ip_risk_policies
WHERE enabled = TRUE
  AND (expires_at IS NULL OR expires_at > $3)
  AND (
      (exact_ip IS NOT NULL AND exact_ip = $1::inet)
      OR
      (ip_network IS NOT NULL AND $1::inet <<= ip_network)
      OR
      (ip_network IS NOT NULL AND $2::cidr <<= ip_network)
  )`,
		exactIP,
		network,
		at.UTC(),
	).Scan(
		&match.Allowlisted,
		&match.KnownSharedNetwork,
		&match.Observed,
		&match.RegistrationBlock,
	)
	return match, err
}

func (r *ipRiskRepository) UpsertIPRiskCase(
	ctx context.Context,
	input *service.IPRiskCaseUpsert,
) (*service.IPRiskCase, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("nil ip risk repository")
	}
	if input == nil {
		return nil, errors.New("nil ip risk case")
	}
	evidenceSnapshot, err := json.Marshal(map[string]any{
		"signals":     input.Signals,
		"evidence":    input.Evidence,
		"shadow_mode": input.ShadowMode,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal ip risk evidence: %w", err)
	}
	recommendedActions, err := json.Marshal(input.RecommendedActions)
	if err != nil {
		return nil, fmt.Errorf("marshal ip risk recommendations: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	item := &service.IPRiskCase{}
	caseKey := "network:" + strings.TrimSpace(input.PrimaryNetwork)
	err = tx.QueryRowContext(ctx, `
INSERT INTO ip_risk_cases (
    case_key, primary_ip, primary_network, score, level, status,
    evidence_confidence, evidence_snapshot, recommended_actions,
    auto_block_eligible, first_detected_at, last_detected_at
) VALUES (
    $1, $2::inet, $3::cidr, $4, $5, 'open',
    $6, $7::jsonb, $8::jsonb, $9, $10, $10
)
ON CONFLICT (case_key) DO UPDATE SET
    primary_ip = EXCLUDED.primary_ip,
    primary_network = EXCLUDED.primary_network,
    score = EXCLUDED.score,
    level = EXCLUDED.level,
    status = CASE
        WHEN ip_risk_cases.status IN ('resolved', 'ignored') THEN 'open'
        ELSE ip_risk_cases.status
    END,
    evidence_confidence = EXCLUDED.evidence_confidence,
    evidence_snapshot = EXCLUDED.evidence_snapshot,
    recommended_actions = EXCLUDED.recommended_actions,
    auto_block_eligible = EXCLUDED.auto_block_eligible,
    last_detected_at = EXCLUDED.last_detected_at,
    resolved_at = NULL,
    version = ip_risk_cases.version + 1,
    updated_at = NOW()
RETURNING
    id, case_key, host(primary_ip), primary_network::text, score, level,
    status, evidence_confidence, auto_block_eligible,
    first_detected_at, last_detected_at, version`,
		caseKey,
		input.PrimaryIP,
		input.PrimaryNetwork,
		input.Score,
		string(input.Level),
		input.EvidenceConfidence,
		string(evidenceSnapshot),
		string(recommendedActions),
		input.AutoBlockEligible,
		input.DetectedAt.UTC(),
	).Scan(
		&item.ID,
		&item.CaseKey,
		&item.PrimaryIP,
		&item.PrimaryNetwork,
		&item.Score,
		&item.Level,
		&item.Status,
		&item.EvidenceConfidence,
		&item.AutoBlockEligible,
		&item.FirstDetectedAt,
		&item.LastDetectedAt,
		&item.Version,
	)
	if err != nil {
		return nil, err
	}

	userIDs := make([]int64, 0, len(input.Users))
	for _, related := range input.Users {
		if related.UserID <= 0 {
			continue
		}
		userIDs = append(userIDs, related.UserID)
		evidenceJSON, marshalErr := json.Marshal(related.Evidence)
		if marshalErr != nil {
			return nil, marshalErr
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO ip_risk_case_users (
    case_id, user_id, relation_type, evidence_confidence,
    evidence_snapshot, recommended_selected, first_seen_at, last_seen_at
) VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7, $8)
ON CONFLICT (case_id, user_id) DO UPDATE SET
    relation_type = EXCLUDED.relation_type,
    evidence_confidence = EXCLUDED.evidence_confidence,
    evidence_snapshot = EXCLUDED.evidence_snapshot,
    recommended_selected = EXCLUDED.recommended_selected,
    first_seen_at = LEAST(ip_risk_case_users.first_seen_at, EXCLUDED.first_seen_at),
    last_seen_at = EXCLUDED.last_seen_at,
    updated_at = NOW()`,
			item.ID,
			related.UserID,
			string(related.RelationType),
			string(related.EvidenceConfidence),
			string(evidenceJSON),
			related.RecommendedSelected,
			related.FirstSeenAt.UTC(),
			related.LastSeenAt.UTC(),
		); err != nil {
			return nil, err
		}
	}
	if len(userIDs) == 0 {
		if _, err := tx.ExecContext(ctx, `DELETE FROM ip_risk_case_users WHERE case_id = $1`, item.ID); err != nil {
			return nil, err
		}
	} else {
		if _, err := tx.ExecContext(ctx, `
DELETE FROM ip_risk_case_users
WHERE case_id = $1
  AND NOT (user_id = ANY($2::bigint[]))`,
			item.ID,
			int64ArrayLiteral(userIDs),
		); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return item, nil
}

func (r *ipRiskRepository) CreateIPRiskScan(
	ctx context.Context,
	input *service.IPRiskScanCreate,
) (*service.IPRiskScan, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("nil ip risk repository")
	}
	if input == nil {
		return nil, errors.New("nil ip risk scan")
	}
	item := &service.IPRiskScan{}
	var requestedBy sql.NullInt64
	err := r.db.QueryRowContext(ctx, `
INSERT INTO ip_risk_scans (
    scan_type, status, requested_by, range_start, range_end, started_at
) VALUES (
    $1,
    $2::VARCHAR,
    $3,
    $4,
    $5,
    CASE WHEN $2::VARCHAR = 'running' THEN NOW() ELSE NULL END
)
RETURNING
    id, scan_type, status, requested_by, range_start, range_end,
    progress, candidate_count, case_count, inferred_event_count,
    error_message, started_at, completed_at, created_at, updated_at`,
		string(input.ScanType),
		string(input.Status),
		input.RequestedBy,
		input.RangeStart.UTC(),
		input.RangeEnd.UTC(),
	).Scan(
		&item.ID,
		&item.ScanType,
		&item.Status,
		&requestedBy,
		&item.RangeStart,
		&item.RangeEnd,
		&item.Progress,
		&item.CandidateCount,
		&item.CaseCount,
		&item.InferredEventCount,
		&item.ErrorMessage,
		&item.StartedAt,
		&item.CompletedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if requestedBy.Valid {
		value := requestedBy.Int64
		item.RequestedBy = &value
	}
	return item, err
}

func (r *ipRiskRepository) UpdateIPRiskScan(
	ctx context.Context,
	scanID int64,
	update *service.IPRiskScanUpdate,
) error {
	if r == nil || r.db == nil {
		return errors.New("nil ip risk repository")
	}
	if update == nil || scanID <= 0 {
		return errors.New("invalid ip risk scan update")
	}
	_, err := r.db.ExecContext(ctx, `
UPDATE ip_risk_scans
SET
    status = COALESCE(NULLIF($2, ''), status),
    progress = $3,
    candidate_count = $4,
    case_count = $5,
    inferred_event_count = $6,
    error_message = $7,
    started_at = COALESCE($8, started_at),
    completed_at = COALESCE($9, completed_at),
    updated_at = NOW()
WHERE id = $1`,
		scanID,
		string(update.Status),
		update.Progress,
		update.CandidateCount,
		update.CaseCount,
		update.InferredEventCount,
		update.ErrorMessage,
		update.StartedAt,
		update.CompletedAt,
	)
	return err
}

func (r *ipRiskRepository) ListHistoricalRegistrationCandidates(
	ctx context.Context,
	start,
	end time.Time,
	afterAuditID int64,
	limit int,
) (*service.IPRiskHistoricalPage, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("nil ip risk repository")
	}
	if limit <= 0 {
		limit = 500
	}
	if limit > 1000 {
		limit = 1000
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT
    id, created_at, COALESCE(client_ip, ''), COALESCE(user_agent, ''),
    COALESCE(request_id, ''), COALESCE(request_body, '')
FROM audit_logs
WHERE id > $1
  AND created_at >= $2
  AND created_at <= $3
	  AND action IN ('auth.register', 'auth.mobile.register')
  AND status_code >= 200
  AND status_code < 300
  AND method = 'POST'
  AND path IN ('/api/v1/auth/register', '/api/v1/auth/mobile/register')
ORDER BY id ASC
LIMIT $4`,
		afterAuditID,
		start.UTC(),
		end.UTC(),
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	type auditCandidate struct {
		id          int64
		occurredAt  time.Time
		clientIP    string
		userAgent   string
		requestID   string
		requestBody string
	}
	raw := make([]auditCandidate, 0, limit)
	for rows.Next() {
		var item auditCandidate
		if err := rows.Scan(
			&item.id,
			&item.occurredAt,
			&item.clientIP,
			&item.userAgent,
			&item.requestID,
			&item.requestBody,
		); err != nil {
			return nil, err
		}
		raw = append(raw, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	page := &service.IPRiskHistoricalPage{
		Candidates: make([]service.IPRiskHistoricalRegistrationCandidate, 0, len(raw)),
		Done:       len(raw) < limit,
	}
	if len(raw) > 0 {
		page.NextAuditID = raw[len(raw)-1].id
	}
	for _, candidate := range raw {
		email := historicalRegistrationEmail(candidate.requestBody)
		if email == "" || strings.TrimSpace(candidate.clientIP) == "" {
			continue
		}
		var userID int64
		var matchedEmail string
		err := r.db.QueryRowContext(ctx, `
SELECT id, email
FROM users
WHERE LOWER(email) = LOWER($1)
  AND deleted_at IS NULL
  AND COALESCE(NULLIF(signup_source, ''), 'email') = 'email'
  AND created_at >= $2::timestamptz - INTERVAL '5 minutes'
  AND created_at <= $2::timestamptz + INTERVAL '5 minutes'
ORDER BY ABS(EXTRACT(EPOCH FROM (created_at - $2::timestamptz))) ASC, id ASC
LIMIT 1`,
			email,
			candidate.occurredAt.UTC(),
		).Scan(&userID, &matchedEmail)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		page.Candidates = append(page.Candidates, service.IPRiskHistoricalRegistrationCandidate{
			AuditID:    candidate.id,
			UserID:     userID,
			Email:      matchedEmail,
			ClientIP:   candidate.clientIP,
			UserAgent:  candidate.userAgent,
			RequestID:  candidate.requestID,
			OccurredAt: candidate.occurredAt.UTC(),
		})
	}
	return page, nil
}

func (r *ipRiskRepository) DeleteAuthRiskEventsBefore(
	ctx context.Context,
	cutoff time.Time,
	batchSize int,
) (int64, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("nil ip risk repository")
	}
	if batchSize <= 0 {
		batchSize = 5000
	}
	result, err := r.db.ExecContext(ctx, `
WITH doomed AS (
    SELECT id
    FROM auth_risk_events
    WHERE occurred_at < $1
    ORDER BY id ASC
    LIMIT $2
)
DELETE FROM auth_risk_events
WHERE id IN (SELECT id FROM doomed)`,
		cutoff.UTC(),
		batchSize,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (r *ipRiskRepository) DeleteIPRiskRecordsBefore(
	ctx context.Context,
	cutoff time.Time,
	batchSize int,
) (int64, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("nil ip risk repository")
	}
	if batchSize <= 0 {
		batchSize = 5000
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	var total int64
	statements := []string{
		`WITH doomed AS (
		    SELECT id FROM ip_risk_actions
		    WHERE created_at < $1
		    ORDER BY id ASC LIMIT $2
		)
		DELETE FROM ip_risk_actions WHERE id IN (SELECT id FROM doomed)`,
		`WITH doomed AS (
		    SELECT id FROM ip_risk_cases
		    WHERE last_detected_at < $1
		    ORDER BY id ASC LIMIT $2
		)
		DELETE FROM ip_risk_cases WHERE id IN (SELECT id FROM doomed)`,
		`WITH doomed AS (
		    SELECT id FROM ip_risk_scans
		    WHERE created_at < $1
		    ORDER BY id ASC LIMIT $2
		)
		DELETE FROM ip_risk_scans WHERE id IN (SELECT id FROM doomed)`,
	}
	for _, statement := range statements {
		result, err := tx.ExecContext(ctx, statement, cutoff.UTC(), batchSize)
		if err != nil {
			return 0, err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		total += affected
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return total, nil
}

func (r *ipRiskRepository) LatestIPRiskScan(ctx context.Context) (*service.IPRiskScan, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("nil ip risk repository")
	}
	item := &service.IPRiskScan{}
	var requestedBy sql.NullInt64
	err := r.db.QueryRowContext(ctx, `
SELECT
    id, scan_type, status, requested_by, range_start, range_end,
    progress, candidate_count, case_count, inferred_event_count,
    error_message, started_at, completed_at, created_at, updated_at
FROM ip_risk_scans
ORDER BY created_at DESC, id DESC
LIMIT 1`).Scan(
		&item.ID,
		&item.ScanType,
		&item.Status,
		&requestedBy,
		&item.RangeStart,
		&item.RangeEnd,
		&item.Progress,
		&item.CandidateCount,
		&item.CaseCount,
		&item.InferredEventCount,
		&item.ErrorMessage,
		&item.StartedAt,
		&item.CompletedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if requestedBy.Valid {
		value := requestedBy.Int64
		item.RequestedBy = &value
	}
	return item, err
}

func (r *ipRiskRepository) HasCompletedIPRiskScan(
	ctx context.Context,
	scanType service.IPRiskScanType,
) (bool, error) {
	if r == nil || r.db == nil {
		return false, errors.New("nil ip risk repository")
	}
	var completed bool
	err := r.db.QueryRowContext(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM ip_risk_scans
    WHERE scan_type = $1
      AND status = 'completed'
)`,
		string(scanType),
	).Scan(&completed)
	return completed, err
}

func historicalRegistrationEmail(requestBody string) string {
	requestBody = strings.TrimSpace(requestBody)
	if requestBody == "" {
		return ""
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(requestBody), &body); err != nil {
		return ""
	}
	for key, value := range body {
		if !strings.EqualFold(strings.TrimSpace(key), "email") {
			continue
		}
		email, _ := value.(string)
		return strings.TrimSpace(strings.ToLower(email))
	}
	return ""
}

func nullableBytes(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return value
}

func int64ArrayLiteral(values []int64) string {
	if len(values) == 0 {
		return "{}"
	}
	var builder strings.Builder
	_ = builder.WriteByte('{')
	for index, value := range values {
		if index > 0 {
			_ = builder.WriteByte(',')
		}
		_, _ = fmt.Fprintf(&builder, "%d", value)
	}
	_ = builder.WriteByte('}')
	return builder.String()
}
