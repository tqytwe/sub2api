package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/shopspring/decimal"
)

func (r *playRepository) CountAdminTeams(ctx context.Context) (int, int, error) {
	exec := r.sqlExec(ctx)
	var total, active int
	if err := scanSingleRow(ctx, exec, `
		SELECT COUNT(*)::int,
		       COUNT(*) FILTER (WHERE archived_at IS NULL)::int
		FROM play_teams`, nil, &total, &active); err != nil {
		return 0, 0, fmt.Errorf("count admin team statuses: %w", err)
	}
	return total, active, nil
}

func (r *playRepository) CountTeamRewardSettlementsNeedingAttention(ctx context.Context) (int, error) {
	exec := r.sqlExec(ctx)
	var count int
	if err := scanSingleRow(ctx, exec, `
		SELECT COUNT(*)::int
		FROM play_team_settlements
		WHERE status IN ('pending', 'processing', 'partial', 'failed')`, nil, &count); err != nil {
		return 0, fmt.Errorf("count team reward settlements needing attention: %w", err)
	}
	return count, nil
}

func (r *playRepository) ListAdminTeamMonthlySpends(ctx context.Context, start, end time.Time) (result []decimal.Decimal, err error) {
	exec := r.sqlExec(ctx)
	rows, err := exec.QueryContext(ctx, teamCompetitionScoreCTE+`
		SELECT COALESCE(team_scores.spend, 0)::text AS team_spend
		FROM play_teams t
		LEFT JOIN team_scores ON team_scores.team_id = t.id
		WHERE t.archived_at IS NULL`, start, end)
	if err != nil {
		return nil, fmt.Errorf("list admin team monthly spends: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()

	out := make([]decimal.Decimal, 0)
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan admin team monthly spend: %w", err)
		}
		spend, err := decimal.NewFromString(raw)
		if err != nil {
			return nil, fmt.Errorf("parse admin team monthly spend: %w", err)
		}
		out = append(out, spend.Round(8))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate admin team monthly spends: %w", err)
	}
	return out, nil
}

func (r *playRepository) GetAdminTeamMeta(ctx context.Context, teamID int64) (*service.PlayAdminTeamListItem, error) {
	items, _, err := r.listAdminTeams(ctx, "all", "", time.Time{}, time.Time{}, 1, 0, &teamID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	return &items[0], nil
}

func (r *playRepository) ListAdminTeams(
	ctx context.Context,
	status string,
	query string,
	start time.Time,
	end time.Time,
	limit int,
	offset int,
) ([]service.PlayAdminTeamListItem, int, error) {
	return r.listAdminTeams(ctx, status, query, start, end, limit, offset, nil)
}

func (r *playRepository) listAdminTeams(
	ctx context.Context,
	status string,
	query string,
	start time.Time,
	end time.Time,
	limit int,
	offset int,
	extraTeamID *int64,
) ([]service.PlayAdminTeamListItem, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	exec := r.sqlExec(ctx)
	var total int
	countWhere, countArgs := buildAdminTeamWhereAt(status, query, 0)
	if extraTeamID != nil {
		countArgs = append(countArgs, *extraTeamID)
		countWhere = append(countWhere, fmt.Sprintf("t.id = $%d", len(countArgs)))
	}
	countWhereSQL := ""
	if len(countWhere) > 0 {
		countWhereSQL = "WHERE " + strings.Join(countWhere, " AND ")
	}
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM play_teams t
		JOIN users captain ON captain.id = t.captain_user_id
		%s`, countWhereSQL)
	if err := scanSingleRow(ctx, exec, countQuery, countArgs, &total); err != nil {
		return nil, 0, fmt.Errorf("count admin teams: %w", err)
	}

	where, filterArgs := buildAdminTeamWhereAt(status, query, 2)
	queryArgs := []any{start, end}
	queryArgs = append(queryArgs, filterArgs...)
	if extraTeamID != nil {
		queryArgs = append(queryArgs, *extraTeamID)
		where = append(where, fmt.Sprintf("t.id = $%d", len(queryArgs)))
	}
	whereSQL := ""
	if len(where) > 0 {
		whereSQL = "WHERE " + strings.Join(where, " AND ")
	}
	limitPos := len(queryArgs) + 1
	offsetPos := limitPos + 1
	queryArgs = append(queryArgs, limit, offset)
	rows, err := exec.QueryContext(ctx, fmt.Sprintf(teamCompetitionScoreCTE+`
		, team_usage AS (
			SELECT em.team_id,
			       COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens), 0)::bigint AS token_sum
			FROM eligible_members em
			LEFT JOIN usage_logs ul
			  ON ul.user_id = em.user_id
			 AND ul.actual_cost > 0
			 AND ul.created_at >= em.eligible_at
			 AND ul.created_at < em.inactive_at
			GROUP BY em.team_id
		)
		SELECT t.id, t.name, t.invite_code, t.captain_user_id,
		       COALESCE(captain.username, '') AS captain_username,
		       COALESCE(captain.email, '') AS captain_email,
		       COALESCE(NULLIF(TRIM(ua.url), ''), '') AS captain_avatar_url,
		       COALESCE(am.member_count, 0),
		       COALESCE(tu.token_sum, 0),
		       COALESCE(team_scores.spend, 0)::text AS team_spend,
		       t.created_at, t.archived_at
		FROM play_teams t
		JOIN users captain ON captain.id = t.captain_user_id
		LEFT JOIN user_avatars ua ON ua.user_id = t.captain_user_id
		LEFT JOIN active_members am ON am.team_id = t.id
		LEFT JOIN team_usage tu ON tu.team_id = t.id
		LEFT JOIN team_scores ON team_scores.team_id = t.id
		%s
		ORDER BY t.created_at DESC, t.id DESC
		LIMIT $%d OFFSET $%d`,
		whereSQL,
		limitPos,
		offsetPos,
	), queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list admin teams: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.PlayAdminTeamListItem, 0, limit)
	for rows.Next() {
		item, err := scanAdminTeamListItem(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate admin teams: %w", err)
	}
	return items, total, nil
}

func buildAdminTeamWhere(status string, query string) ([]string, []any) {
	return buildAdminTeamWhereAt(status, query, 0)
}

func buildAdminTeamWhereAt(status string, query string, placeholderOffset int) ([]string, []any) {
	where := make([]string, 0, 3)
	args := make([]any, 0, 2)
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "active", "":
		where = append(where, "t.archived_at IS NULL")
	case "archived":
		where = append(where, "t.archived_at IS NOT NULL")
	case "all":
	default:
		where = append(where, "t.archived_at IS NULL")
	}
	if q := strings.TrimSpace(query); q != "" {
		args = append(args, "%"+strings.ToLower(q)+"%")
		placeholder := placeholderOffset + len(args)
		where = append(where, fmt.Sprintf(`(
			LOWER(t.name) LIKE $%d OR
			LOWER(t.invite_code) LIKE $%d OR
			LOWER(captain.username) LIKE $%d OR
			LOWER(captain.email) LIKE $%d
		)`, placeholder, placeholder, placeholder, placeholder))
	}
	return where, args
}

func scanAdminTeamListItem(scan rowScanner) (*service.PlayAdminTeamListItem, error) {
	var item service.PlayAdminTeamListItem
	var username, email, spend string
	var archivedAt sql.NullTime
	if err := scan.Scan(
		&item.ID,
		&item.Name,
		&item.InviteCode,
		&item.CaptainID,
		&username,
		&email,
		&item.CaptainAvatarURL,
		&item.MemberCount,
		&item.TokenSum,
		&spend,
		&item.CreatedAt,
		&archivedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan admin team: %w", err)
	}
	parsed, err := decimal.NewFromString(spend)
	if err != nil {
		return nil, fmt.Errorf("parse admin team spend: %w", err)
	}
	item.TeamSpend = parsed.Round(8)
	item.CaptainDisplayName = service.AdminPlayDisplayName(username, email, item.CaptainID)
	item.CaptainEmail = strings.TrimSpace(email)
	if archivedAt.Valid {
		item.ArchivedAt = &archivedAt.Time
	}
	return &item, nil
}
