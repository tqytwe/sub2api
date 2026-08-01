package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/shopspring/decimal"
)

func (r *playRepository) ListPublicTeamDirectory(
	ctx context.Context,
	start time.Time,
	end time.Time,
	limit int,
) (result []service.PlayTeamDirectoryBase, err error) {
	rows, err := r.sqlExec(ctx).QueryContext(ctx, teamCompetitionScoreCTE+`
		SELECT t.id AS team_id,
		       t.name AS team_name,
		       COALESCE(active_members.member_count, 0)::int AS member_count,
		       t.is_recruiting,
		       COALESCE(team_scores.spend, 0)::text AS monthly_spend
		FROM play_teams t
		LEFT JOIN team_scores ON team_scores.team_id = t.id
		LEFT JOIN active_members ON active_members.team_id = t.id
		WHERE t.archived_at IS NULL
		ORDER BY
			CASE WHEN t.is_recruiting AND COALESCE(active_members.member_count, 0) < 30 THEN 0 ELSE 1 END,
			COALESCE(team_scores.spend, 0) DESC,
			t.id ASC
		LIMIT $3`, start, end, normalizePublicTeamLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list public team directory: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()

	for rows.Next() {
		var row service.PlayTeamDirectoryBase
		var spend string
		if err := rows.Scan(&row.TeamID, &row.TeamName, &row.MemberCount, &row.Recruiting, &spend); err != nil {
			return nil, fmt.Errorf("scan public team directory: %w", err)
		}
		row.Spend, err = decimal.NewFromString(spend)
		if err != nil {
			return nil, fmt.Errorf("parse public team directory spend: %w", err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate public team directory: %w", err)
	}
	return result, nil
}

func (r *playRepository) ListPublicTeamLeaderboard(
	ctx context.Context,
	start time.Time,
	end time.Time,
	limit int,
) (result []service.PlayTeamPublicLeaderboardBase, total int, err error) {
	rows, err := r.sqlExec(ctx).QueryContext(ctx, teamCompetitionScoreCTE+`
		, board_values AS (
			SELECT t.id AS team_id,
			       t.name AS team_name,
			       COALESCE(active_members.member_count, 0)::int AS member_count,
			       COALESCE(team_scores.spend, 0) AS spend
			FROM play_teams t
			LEFT JOIN team_scores ON team_scores.team_id = t.id
			LEFT JOIN active_members ON active_members.team_id = t.id
			WHERE t.archived_at IS NULL
		), ranked AS (
			SELECT
				ROW_NUMBER() OVER (ORDER BY spend DESC, team_id ASC)::int AS rank,
				team_id,
				team_name,
				member_count,
				spend,
				COALESCE(LAG(spend) OVER (ORDER BY spend DESC, team_id ASC), spend) AS previous_spend,
				COUNT(*) OVER ()::int AS total_teams
			FROM board_values
		)
		SELECT rank, team_id, team_name, member_count,
		       spend::text,
		       GREATEST(previous_spend - spend, 0)::text AS gap_to_previous,
		       total_teams
		FROM ranked
		ORDER BY rank ASC
		LIMIT $3`, start, end, normalizePublicTeamLimit(limit))
	if err != nil {
		return nil, 0, fmt.Errorf("list public team leaderboard: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
			total = 0
		}
	}()

	for rows.Next() {
		var row service.PlayTeamPublicLeaderboardBase
		var spend, gap string
		if err := rows.Scan(&row.Rank, &row.TeamID, &row.TeamName, &row.MemberCount, &spend, &gap, &total); err != nil {
			return nil, 0, fmt.Errorf("scan public team leaderboard: %w", err)
		}
		row.Spend, err = decimal.NewFromString(spend)
		if err != nil {
			return nil, 0, fmt.Errorf("parse public team leaderboard spend: %w", err)
		}
		row.GapToPrev, err = decimal.NewFromString(gap)
		if err != nil {
			return nil, 0, fmt.Errorf("parse public team leaderboard gap: %w", err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate public team leaderboard: %w", err)
	}
	return result, total, nil
}

func (r *playRepository) ListPublicTeamSeasons(ctx context.Context, limit int) (result []service.PlayTeamSeason, err error) {
	rows, err := r.sqlExec(ctx).QueryContext(ctx, `
		SELECT s.id, s.period_start, s.window_start, s.window_end,
		       s.rules_json::text, s.status, s.frozen_at, s.settled_at
		FROM play_team_seasons s
		WHERE s.status IN ('settled', 'legacy')
		ORDER BY s.period_start DESC
		LIMIT $1`, normalizePublicTeamSeasonLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list public team seasons: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()
	for rows.Next() {
		season, scanErr := scanPublicTeamSeason(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, *season)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate public team seasons: %w", err)
	}
	return result, nil
}

func (r *playRepository) GetPublicTeamSeason(
	ctx context.Context,
	month time.Time,
	limit int,
) (*service.PlayTeamSeason, []service.PlayTeamSeasonRanking, int, error) {
	var seasonID int64
	var periodStart time.Time
	var windowStart, windowEnd time.Time
	var rules string
	var status string
	var frozenAt, settledAt sql.NullTime
	err := scanSingleRow(ctx, r.sqlExec(ctx), `
		SELECT id, period_start, window_start, window_end,
		       rules_json::text, status, frozen_at, settled_at
		FROM play_team_seasons
		WHERE period_start = $1
		  AND status IN ('settled', 'legacy')`, []any{month.Format("2006-01-02")},
		&seasonID, &periodStart, &windowStart, &windowEnd, &rules, &status, &frozenAt, &settledAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, []service.PlayTeamSeasonRanking{}, 0, nil
	}
	if err != nil {
		return nil, nil, 0, fmt.Errorf("get public team season: %w", err)
	}
	season, err := newPublicTeamSeason(seasonID, periodStart, windowStart, windowEnd, rules, status, frozenAt, settledAt)
	if err != nil {
		return nil, nil, 0, err
	}

	rows, err := r.sqlExec(ctx).QueryContext(ctx, `
		SELECT rank, team_id, team_name, member_count,
		       team_spend::text, reached_threshold::text, reward_rate::text,
		       pool_amount::text, paid_amount::text, settlement_status,
		       COUNT(*) OVER ()::int AS total_teams
		FROM play_team_season_rankings
		WHERE season_id = $1
		ORDER BY rank ASC
		LIMIT $2`, seasonID, normalizePublicTeamSeasonRankingLimit(limit))
	if err != nil {
		return nil, nil, 0, fmt.Errorf("list public team season rankings: %w", err)
	}
	defer func() { _ = rows.Close() }()
	rankings := make([]service.PlayTeamSeasonRanking, 0)
	total := 0
	for rows.Next() {
		var row service.PlayTeamSeasonRanking
		var spend, threshold, rate, pool, paid string
		if err := rows.Scan(
			&row.Rank, &row.TeamID, &row.TeamName, &row.MemberCount,
			&spend, &threshold, &rate, &pool, &paid, &row.SettlementStatus, &total,
		); err != nil {
			return nil, nil, 0, fmt.Errorf("scan public team season ranking: %w", err)
		}
		if row.TeamSpend, err = decimal.NewFromString(spend); err != nil {
			return nil, nil, 0, fmt.Errorf("parse public team season spend: %w", err)
		}
		if row.ReachedThreshold, err = decimal.NewFromString(threshold); err != nil {
			return nil, nil, 0, fmt.Errorf("parse public team season threshold: %w", err)
		}
		if row.RewardRate, err = decimal.NewFromString(rate); err != nil {
			return nil, nil, 0, fmt.Errorf("parse public team season rate: %w", err)
		}
		if row.PoolAmount, err = decimal.NewFromString(pool); err != nil {
			return nil, nil, 0, fmt.Errorf("parse public team season pool: %w", err)
		}
		if row.PaidAmount, err = decimal.NewFromString(paid); err != nil {
			return nil, nil, 0, fmt.Errorf("parse public team season paid amount: %w", err)
		}
		rankings = append(rankings, row)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, 0, fmt.Errorf("iterate public team season rankings: %w", err)
	}
	return season, rankings, total, nil
}

func scanPublicTeamSeason(scanner interface{ Scan(...any) error }) (*service.PlayTeamSeason, error) {
	var id int64
	var periodStart, windowStart, windowEnd time.Time
	var rules, status string
	var frozenAt, settledAt sql.NullTime
	if err := scanner.Scan(&id, &periodStart, &windowStart, &windowEnd, &rules, &status, &frozenAt, &settledAt); err != nil {
		return nil, fmt.Errorf("scan public team season: %w", err)
	}
	return newPublicTeamSeason(id, periodStart, windowStart, windowEnd, rules, status, frozenAt, settledAt)
}

func newPublicTeamSeason(id int64, periodStart, windowStart, windowEnd time.Time, rules, status string, frozenAt, settledAt sql.NullTime) (*service.PlayTeamSeason, error) {
	season := &service.PlayTeamSeason{
		ID:          id,
		Month:       periodStart.Format("2006-01"),
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
		Rules:       map[string]any{},
		Status:      status,
	}
	if rules != "" {
		if err := json.Unmarshal([]byte(rules), &season.Rules); err != nil {
			return nil, fmt.Errorf("decode public team season rules: %w", err)
		}
	}
	if frozenAt.Valid {
		value := frozenAt.Time
		season.FrozenAt = &value
	}
	if settledAt.Valid {
		value := settledAt.Time
		season.SettledAt = &value
	}
	return season, nil
}

func normalizePublicTeamLimit(limit int) int {
	if limit <= 0 || limit > 50 {
		return 50
	}
	return limit
}

func normalizePublicTeamSeasonLimit(limit int) int {
	if limit <= 0 || limit > 24 {
		return 12
	}
	return limit
}

func normalizePublicTeamSeasonRankingLimit(limit int) int {
	if limit <= 0 || limit > 10 {
		return 10
	}
	return limit
}

// teamCompetitionEligibleMembersCTE is the canonical monthly scoring window.
// All live team score readers and settlement inputs must build on this exact
// membership interval so a member cannot be credited before joining, after
// leaving, or before late-month reward eligibility begins.
const teamCompetitionEligibleMembersCTE = `
	WITH eligible_members AS (
		SELECT
			m.team_id,
			m.user_id,
			GREATEST(m.joined_at, m.reward_eligible_at, $1) AS eligible_at,
			LEAST(COALESCE(m.left_at, $2), $2) AS inactive_at
		FROM play_team_members m
		JOIN play_teams active_team ON active_team.id = m.team_id
		WHERE active_team.archived_at IS NULL
		  AND m.joined_at < $2
		  AND m.reward_eligible_at < $2
		  AND (m.left_at IS NULL OR m.left_at > $1)
 	)
`

const teamCompetitionScoreCTE = teamCompetitionEligibleMembersCTE + `
	, team_scores AS (
		SELECT
			em.team_id,
			COALESCE(SUM(ul.actual_cost), 0) AS spend
		FROM eligible_members em
		LEFT JOIN usage_logs ul
		  ON ul.user_id = em.user_id
		 AND ul.actual_cost > 0
		 AND ul.created_at >= em.eligible_at
		 AND ul.created_at < em.inactive_at
		GROUP BY em.team_id
	), active_members AS (
		SELECT team_id, COUNT(*)::int AS member_count
		FROM play_team_members
		WHERE left_at IS NULL
		GROUP BY team_id
	)
`

func (r *playRepository) WithTeamCompetitionAdmissionTx(ctx context.Context, fn func(context.Context) error) error {
	if dbent.TxFromContext(ctx) != nil {
		return fn(ctx)
	}
	if r.client == nil {
		return fmt.Errorf("team competition admission transaction: ent client missing")
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin team competition admission tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	if err := fn(txCtx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit team competition admission tx: %w", err)
	}
	return nil
}

func (r *playRepository) CreateTeamWithInvite(ctx context.Context, name string, captainUserID int64, invite service.PlayTeamInvite) (*service.PlayTeamDB, error) {
	var team service.PlayTeamDB
	err := scanSingleRow(ctx, r.sqlExec(ctx), `
		INSERT INTO play_teams (
			name, captain_user_id, invite_code,
			invite_code_expires_at, invite_code_rotated_at, is_recruiting
		)
		VALUES ($1, $2, $3, $4, $5, TRUE)
		RETURNING id, name, captain_user_id, invite_code`,
		[]any{name, captainUserID, invite.Code, invite.ExpiresAt, invite.RotatedAt},
		&team.ID, &team.Name, &team.CaptainUserID, &team.InviteCode,
	)
	if err != nil {
		return nil, fmt.Errorf("create team with competition invite: %w", err)
	}
	return &team, nil
}

func (r *playRepository) LockTeamByInviteCode(ctx context.Context, inviteCode string) (*service.PlayTeamCompetitionTeam, error) {
	return r.lockTeamCompetition(ctx, `invite_code = $1`, inviteCode)
}

func (r *playRepository) LockTeamCompetition(ctx context.Context, teamID int64) (*service.PlayTeamCompetitionTeam, error) {
	return r.lockTeamCompetition(ctx, `id = $1`, teamID)
}

func (r *playRepository) lockTeamCompetition(ctx context.Context, predicate string, arg any) (*service.PlayTeamCompetitionTeam, error) {
	var team service.PlayTeamCompetitionTeam
	var expiresAt, rotatedAt sql.NullTime
	err := scanSingleRow(ctx, r.sqlExec(ctx), `
		SELECT id, name, captain_user_id, invite_code,
		       invite_code_expires_at, invite_code_rotated_at, is_recruiting
		FROM play_teams
		WHERE `+predicate+`
		  AND archived_at IS NULL
		FOR UPDATE`, []any{arg},
		&team.ID, &team.Name, &team.CaptainUserID, &team.InviteCode,
		&expiresAt, &rotatedAt, &team.Recruiting,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lock team competition: %w", err)
	}
	if expiresAt.Valid {
		value := expiresAt.Time
		team.InviteCodeExpiresAt = &value
	}
	if rotatedAt.Valid {
		value := rotatedAt.Time
		team.InviteCodeRotatedAt = &value
	}
	return &team, nil
}

func (r *playRepository) UpdateTeamInvite(ctx context.Context, teamID int64, invite service.PlayTeamInvite) error {
	result, err := r.sqlExec(ctx).ExecContext(ctx, `
		UPDATE play_teams
		SET invite_code = $2,
		    invite_code_expires_at = $3,
		    invite_code_rotated_at = $4
		WHERE id = $1
		  AND archived_at IS NULL`, teamID, invite.Code, invite.ExpiresAt, invite.RotatedAt)
	if err != nil {
		return fmt.Errorf("update team invite: %w", err)
	}
	return requireTeamMutationRow(result, service.ErrPlayTeamNotFound, "update team invite")
}

func (r *playRepository) UpdateTeamRecruiting(ctx context.Context, teamID int64, recruiting bool) error {
	result, err := r.sqlExec(ctx).ExecContext(ctx, `
		UPDATE play_teams
		SET is_recruiting = $2
		WHERE id = $1
		  AND archived_at IS NULL`, teamID, recruiting)
	if err != nil {
		return fmt.Errorf("update team recruiting: %w", err)
	}
	return requireTeamMutationRow(result, service.ErrPlayTeamNotFound, "update team recruiting")
}

func (r *playRepository) GetTeamRecruiting(ctx context.Context, teamID int64) (bool, error) {
	var recruiting bool
	err := scanSingleRow(ctx, r.sqlExec(ctx), `
		SELECT is_recruiting
		FROM play_teams
		WHERE id = $1
		  AND archived_at IS NULL`, []any{teamID}, &recruiting)
	if errors.Is(err, sql.ErrNoRows) {
		return false, service.ErrPlayTeamNotFound
	}
	if err != nil {
		return false, fmt.Errorf("get team recruiting: %w", err)
	}
	return recruiting, nil
}

func (r *playRepository) GetLatestTeamLeftAt(ctx context.Context, userID int64) (*time.Time, error) {
	var leftAt time.Time
	err := scanSingleRow(ctx, r.sqlExec(ctx), `
		SELECT left_at
		FROM play_team_members
		WHERE user_id = $1
		  AND left_at IS NOT NULL
		ORDER BY left_at DESC, id DESC
		LIMIT 1`, []any{userID}, &leftAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest team leave time: %w", err)
	}
	return &leftAt, nil
}

func (r *playRepository) JoinTeamWithEligibility(ctx context.Context, teamID, userID int64, joinedAt, rewardEligibleAt time.Time) error {
	_, err := r.sqlExec(ctx).ExecContext(ctx, `
		INSERT INTO play_team_members (team_id, user_id, joined_at, reward_eligible_at)
		VALUES ($1, $2, $3, $4)`, teamID, userID, joinedAt, rewardEligibleAt)
	if err != nil {
		if isPlayTeamUniqueViolation(err) {
			return service.ErrPlayTeamAlreadyJoined
		}
		return fmt.Errorf("join team with eligibility: %w", err)
	}
	return nil
}

func (r *playRepository) CreateTeamJoinApplication(ctx context.Context, application service.PlayTeamJoinApplication) (*service.PlayTeamJoinApplication, error) {
	return scanTeamJoinApplicationQuery(ctx, r.sqlExec(ctx), `
		INSERT INTO play_team_join_applications (
			team_id, applicant_user_id, status, message,
			requested_at, sla_due_at, expires_at, decision_note
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, team_id, applicant_user_id, status, message,
		          requested_at, sla_due_at, expires_at,
		          handled_at, handled_by_user_id, decision_note`,
		application.TeamID, application.ApplicantID, application.Status, application.Message,
		application.RequestedAt, application.SLADueAt, application.ExpiresAt, application.DecisionNote,
	)
}

func (r *playRepository) LockTeamJoinApplication(ctx context.Context, applicationID int64) (*service.PlayTeamJoinApplication, error) {
	application, err := scanTeamJoinApplicationQuery(ctx, r.sqlExec(ctx), `
		SELECT id, team_id, applicant_user_id, status, message,
		       requested_at, sla_due_at, expires_at,
		       handled_at, handled_by_user_id, decision_note
		FROM play_team_join_applications
		WHERE id = $1
		FOR UPDATE`, applicationID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lock team join application: %w", err)
	}
	return application, nil
}

func (r *playRepository) UpdateTeamJoinApplication(ctx context.Context, application service.PlayTeamJoinApplication) error {
	result, err := r.sqlExec(ctx).ExecContext(ctx, `
		UPDATE play_team_join_applications
		SET status = $2,
		    handled_at = $3,
		    handled_by_user_id = $4,
		    decision_note = $5,
		    updated_at = NOW()
		WHERE id = $1`, application.ID, application.Status, application.HandledAt, application.HandledByID, application.DecisionNote)
	if err != nil {
		return fmt.Errorf("update team join application: %w", err)
	}
	return requireTeamMutationRow(result, service.ErrPlayTeamApplicationNotFound, "update team join application")
}

func (r *playRepository) ExpirePendingTeamJoinApplications(ctx context.Context, userID int64, now time.Time) (result []service.PlayTeamJoinApplication, err error) {
	rows, err := r.sqlExec(ctx).QueryContext(ctx, `
		UPDATE play_team_join_applications
		SET status = 'expired',
		    handled_at = COALESCE(handled_at, $2),
		    updated_at = NOW()
		WHERE applicant_user_id = $1
		  AND status = 'pending'
		  AND expires_at <= $2
		RETURNING id, team_id, applicant_user_id, status, message,
		          requested_at, sla_due_at, expires_at,
		          handled_at, handled_by_user_id, decision_note`, userID, now)
	if err != nil {
		return nil, fmt.Errorf("expire team join applications: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()
	for rows.Next() {
		application, scanErr := scanTeamJoinApplication(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, *application)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate expired team join applications: %w", err)
	}
	return result, nil
}

func (r *playRepository) ListUserTeamJoinApplications(ctx context.Context, userID int64, limit int) (result []service.PlayTeamJoinApplication, err error) {
	rows, err := r.sqlExec(ctx).QueryContext(ctx, `
		SELECT id, team_id, applicant_user_id, status, message,
		       requested_at, sla_due_at, expires_at,
		       handled_at, handled_by_user_id, decision_note
		FROM play_team_join_applications
		WHERE applicant_user_id = $1
		ORDER BY requested_at DESC, id DESC
		LIMIT $2`, userID, normalizePublicTeamLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list user team join applications: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()
	for rows.Next() {
		application, scanErr := scanTeamJoinApplication(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, *application)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user team join applications: %w", err)
	}
	return result, nil
}

func (r *playRepository) ListCaptainTeamJoinApplications(ctx context.Context, teamID int64, limit int) (result []service.PlayTeamJoinApplication, err error) {
	rows, err := r.sqlExec(ctx).QueryContext(ctx, `
		SELECT a.id, a.team_id, a.applicant_user_id, a.status, a.message,
		       a.requested_at, a.sla_due_at, a.expires_at,
		       a.handled_at, a.handled_by_user_id, a.decision_note,
		       COALESCE(u.username, ''), COALESCE(u.email, '')
		FROM play_team_join_applications a
		JOIN users u ON u.id = a.applicant_user_id
		WHERE a.team_id = $1
		ORDER BY a.requested_at DESC, a.id DESC
		LIMIT $2`, teamID, normalizePublicTeamLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("list captain team join applications: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()
	for rows.Next() {
		application, scanErr := scanCaptainTeamJoinApplication(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, *application)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate captain team join applications: %w", err)
	}
	return result, nil
}

func (r *playRepository) RecordTeamJoinApplicationEvent(ctx context.Context, applicationID, teamID int64, actorUserID *int64, eventType, fromStatus, toStatus string, detail map[string]any) error {
	detailJSON, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("marshal team join application event detail: %w", err)
	}
	_, err = r.sqlExec(ctx).ExecContext(ctx, `
		INSERT INTO play_team_join_application_events (
			application_id, team_id, actor_user_id, event_type, from_status, to_status, detail
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, applicationID, teamID, actorUserID, eventType, fromStatus, toStatus, detailJSON)
	if err != nil {
		return fmt.Errorf("record team join application event: %w", err)
	}
	return nil
}

func scanTeamJoinApplicationQuery(ctx context.Context, exec sqlExecutor, query string, args ...any) (*service.PlayTeamJoinApplication, error) {
	var application service.PlayTeamJoinApplication
	var handledAt sql.NullTime
	var handledBy sql.NullInt64
	err := scanSingleRow(ctx, exec, query, args,
		&application.ID, &application.TeamID, &application.ApplicantID, &application.Status, &application.Message,
		&application.RequestedAt, &application.SLADueAt, &application.ExpiresAt,
		&handledAt, &handledBy, &application.DecisionNote,
	)
	if err != nil {
		return nil, err
	}
	if handledAt.Valid {
		value := handledAt.Time
		application.HandledAt = &value
	}
	if handledBy.Valid {
		value := handledBy.Int64
		application.HandledByID = &value
	}
	return &application, nil
}

func scanTeamJoinApplication(scanner interface{ Scan(...any) error }) (*service.PlayTeamJoinApplication, error) {
	var application service.PlayTeamJoinApplication
	var handledAt sql.NullTime
	var handledBy sql.NullInt64
	if err := scanner.Scan(
		&application.ID, &application.TeamID, &application.ApplicantID, &application.Status, &application.Message,
		&application.RequestedAt, &application.SLADueAt, &application.ExpiresAt,
		&handledAt, &handledBy, &application.DecisionNote,
	); err != nil {
		return nil, fmt.Errorf("scan team join application: %w", err)
	}
	if handledAt.Valid {
		value := handledAt.Time
		application.HandledAt = &value
	}
	if handledBy.Valid {
		value := handledBy.Int64
		application.HandledByID = &value
	}
	return &application, nil
}

func scanCaptainTeamJoinApplication(scanner interface{ Scan(...any) error }) (*service.PlayTeamJoinApplication, error) {
	var application service.PlayTeamJoinApplication
	var handledAt sql.NullTime
	var handledBy sql.NullInt64
	var username, email string
	if err := scanner.Scan(
		&application.ID, &application.TeamID, &application.ApplicantID, &application.Status, &application.Message,
		&application.RequestedAt, &application.SLADueAt, &application.ExpiresAt,
		&handledAt, &handledBy, &application.DecisionNote,
		&username, &email,
	); err != nil {
		return nil, fmt.Errorf("scan captain team join application: %w", err)
	}
	if handledAt.Valid {
		value := handledAt.Time
		application.HandledAt = &value
	}
	if handledBy.Valid {
		value := handledBy.Int64
		application.HandledByID = &value
	}
	application.ApplicantDisplayName = service.PublicPlayDisplayName(username, email, application.ApplicantID)
	return &application, nil
}

func (r *playRepository) GetTeamCompetitionSeason(ctx context.Context, periodStart time.Time) (*service.PlayTeamSeason, error) {
	season, err := getTeamCompetitionSeason(ctx, r.sqlExec(ctx), periodStart, false)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get team competition season: %w", err)
	}
	return season, nil
}

func (r *playRepository) EnsureTeamCompetitionSeason(
	ctx context.Context,
	periodStart, windowStart, windowEnd time.Time,
	rules map[string]any,
) (*service.PlayTeamSeason, error) {
	if dbent.TxFromContext(ctx) != nil {
		return r.ensureTeamCompetitionSeason(ctx, r.sqlExec(ctx), periodStart, windowStart, windowEnd, rules)
	}
	if r.client == nil {
		return nil, fmt.Errorf("team competition season: ent client missing")
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin team competition season tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	season, err := r.ensureTeamCompetitionSeason(txCtx, r.sqlExec(txCtx), periodStart, windowStart, windowEnd, rules)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit team competition season: %w", err)
	}
	return season, nil
}

func (r *playRepository) ensureTeamCompetitionSeason(
	ctx context.Context,
	exec sqlExecutor,
	periodStart, windowStart, windowEnd time.Time,
	rules map[string]any,
) (*service.PlayTeamSeason, error) {
	rulesJSON, err := json.Marshal(rules)
	if err != nil {
		return nil, fmt.Errorf("marshal team competition season rules: %w", err)
	}
	season, err := scanTeamCompetitionSeason(ctx, exec, `
		INSERT INTO play_team_seasons (
			period_start, window_start, window_end, rules_json, status, frozen_at, settled_at
		)
		VALUES ($1, $2, $3, $4, 'active', NOW(), NULL)
		ON CONFLICT (period_start) DO NOTHING
		RETURNING id, period_start, window_start, window_end,
		          rules_json::text, status, frozen_at, settled_at`,
		[]any{periodStart.Format("2006-01-02"), windowStart, windowEnd, string(rulesJSON)},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return getTeamCompetitionSeason(ctx, exec, periodStart, true)
	}
	if err != nil {
		return nil, fmt.Errorf("insert team competition season: %w", err)
	}
	return season, nil
}

func getTeamCompetitionSeason(
	ctx context.Context,
	exec sqlExecutor,
	periodStart time.Time,
	forUpdate bool,
) (*service.PlayTeamSeason, error) {
	query := `
		SELECT id, period_start, window_start, window_end,
		       rules_json::text, status, frozen_at, settled_at
		FROM play_team_seasons
		WHERE period_start = $1`
	if forUpdate {
		query += " FOR UPDATE"
	}
	return scanTeamCompetitionSeason(ctx, exec, query, []any{periodStart.Format("2006-01-02")})
}

func scanTeamCompetitionSeason(
	ctx context.Context,
	exec sqlExecutor,
	query string,
	args []any,
) (*service.PlayTeamSeason, error) {
	var id int64
	var periodStart, windowStart, windowEnd time.Time
	var rules, status string
	var frozenAt, settledAt sql.NullTime
	if err := scanSingleRow(ctx, exec, query, args,
		&id, &periodStart, &windowStart, &windowEnd, &rules, &status, &frozenAt, &settledAt,
	); err != nil {
		return nil, err
	}
	return newPublicTeamSeason(id, periodStart, windowStart, windowEnd, rules, status, frozenAt, settledAt)
}

func (r *playRepository) CreateTeamCompetitionSeasonSnapshot(
	ctx context.Context,
	periodStart, windowStart, windowEnd time.Time,
	rules map[string]any,
) (bool, error) {
	if dbent.TxFromContext(ctx) != nil {
		return r.createTeamCompetitionSeasonSnapshot(ctx, r.sqlExec(ctx), periodStart, windowStart, windowEnd, rules)
	}
	if r.client == nil {
		return false, fmt.Errorf("team competition season snapshot: ent client missing")
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return false, fmt.Errorf("begin team competition season snapshot tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	created, err := r.createTeamCompetitionSeasonSnapshot(txCtx, r.sqlExec(txCtx), periodStart, windowStart, windowEnd, rules)
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit team competition season snapshot: %w", err)
	}
	return created, nil
}

func (r *playRepository) createTeamCompetitionSeasonSnapshot(
	ctx context.Context,
	exec sqlExecutor,
	periodStart, windowStart, windowEnd time.Time,
	rules map[string]any,
) (bool, error) {
	season, err := r.ensureTeamCompetitionSeason(ctx, exec, periodStart, windowStart, windowEnd, rules)
	if err != nil {
		return false, err
	}
	if season.Status == "settled" || season.Status == "legacy" {
		return false, nil
	}
	if _, err := exec.ExecContext(ctx, `
		UPDATE play_team_seasons
		SET status = 'settling',
		    frozen_at = COALESCE(frozen_at, NOW()),
		    settled_at = NULL,
		    updated_at = NOW()
		WHERE id = $1`, season.ID); err != nil {
		return false, fmt.Errorf("mark team competition season settling: %w", err)
	}

	var incomplete bool
	if err := scanSingleRow(ctx, exec, `
		SELECT EXISTS (
			SELECT 1
			FROM play_team_settlements s
			WHERE s.period_start = $1
			  AND (
				s.status <> 'completed'
				OR EXISTS (
					SELECT 1
					FROM play_team_reward_allocations a
					WHERE a.settlement_id = s.id
					  AND a.reward_amount > 0
					  AND a.payout_status <> 'paid'
				)
			  )
		)`, []any{periodStart.Format("2006-01-02")}, &incomplete); err != nil {
		return false, fmt.Errorf("check team competition season payouts: %w", err)
	}
	if incomplete {
		return false, nil
	}

	// Settling rows are not public history yet. Rebuild them atomically so a
	// retry after an interrupted pre-publication write cannot retain stale ranks.
	if _, err := exec.ExecContext(ctx, `DELETE FROM play_team_season_rankings WHERE season_id = $1`, season.ID); err != nil {
		return false, fmt.Errorf("clear incomplete team competition season rankings: %w", err)
	}
	if _, err := exec.ExecContext(ctx, `
		WITH ranked AS (
			SELECT
				s.id AS settlement_id,
				s.team_id,
				t.name AS team_name,
				ROW_NUMBER() OVER (ORDER BY s.team_spend DESC, s.team_id ASC)::int AS rank,
				(
					SELECT COUNT(DISTINCT m.user_id)::int
					FROM play_team_members m
					WHERE m.team_id = s.team_id
					  AND m.joined_at < $3
					  AND m.reward_eligible_at < $3
					  AND (m.left_at IS NULL OR m.left_at > $2)
				) AS member_count,
				s.team_spend, s.reached_threshold, s.reward_rate, s.pool_amount,
				COALESCE(SUM(a.reward_amount) FILTER (WHERE a.payout_status = 'paid'), 0) AS paid_amount,
				s.status AS settlement_status
			FROM play_team_settlements s
			JOIN play_teams t ON t.id = s.team_id
			LEFT JOIN play_team_reward_allocations a ON a.settlement_id = s.id
			WHERE s.period_start = $1
			  AND s.status = 'completed'
			GROUP BY s.id, s.team_id, t.name, s.team_spend, s.reached_threshold,
			         s.reward_rate, s.pool_amount, s.status
		)
		INSERT INTO play_team_season_rankings (
			season_id, team_id, team_name, rank, member_count, team_spend,
			reached_threshold, reward_rate, pool_amount, paid_amount,
			settlement_id, settlement_status
		)
		SELECT $4, team_id, team_name, rank, member_count, team_spend,
		       reached_threshold, reward_rate, pool_amount, paid_amount,
		       settlement_id, settlement_status
		FROM ranked
		WHERE rank <= $5
		ORDER BY rank`, periodStart.Format("2006-01-02"), windowStart, windowEnd, season.ID, 10); err != nil {
		return false, fmt.Errorf("create team competition season rankings: %w", err)
	}
	if _, err := exec.ExecContext(ctx, `
		UPDATE play_team_seasons
		SET status = 'settled',
		    settled_at = COALESCE(settled_at, NOW()),
		    updated_at = NOW()
		WHERE id = $1
		  AND status = 'settling'`, season.ID); err != nil {
		return false, fmt.Errorf("finalize team competition season snapshot: %w", err)
	}
	return true, nil
}
