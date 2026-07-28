package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

func (r *playRepository) ListAdminQuizQuestions(ctx context.Context, filter service.PlayAdminQuizQuestionFilter) ([]service.PlayAdminQuizQuestion, int, service.PlayAdminQuizQuestionStats, error) {
	exec := r.sqlExec(ctx)
	where, args := adminQuizQuestionWhere(filter)

	var total int
	if err := scanSingleRow(ctx, exec, `SELECT COUNT(*)::int FROM play_quiz_questions`+where, args, &total); err != nil {
		return nil, 0, service.PlayAdminQuizQuestionStats{}, fmt.Errorf("count admin quiz questions: %w", err)
	}

	stats, err := r.getAdminQuizQuestionStats(ctx)
	if err != nil {
		return nil, 0, service.PlayAdminQuizQuestionStats{}, err
	}

	limit := filter.PageSize
	offset := (filter.Page - 1) * filter.PageSize
	queryArgs := append(append([]any{}, args...), limit, offset)
	rows, err := exec.QueryContext(ctx, `
		SELECT id, language, prompt, options, correct_index,
		       COALESCE(category, ''), COALESCE(difficulty, 'normal'), COALESCE(explanation, ''),
		       sort_order, active, created_at, COALESCE(updated_at, created_at)
		FROM play_quiz_questions`+where+`
		ORDER BY sort_order ASC, id ASC
		LIMIT $`+fmt.Sprint(len(args)+1)+` OFFSET $`+fmt.Sprint(len(args)+2), queryArgs...)
	if err != nil {
		return nil, 0, service.PlayAdminQuizQuestionStats{}, fmt.Errorf("list admin quiz questions: %w", err)
	}
	defer rows.Close()

	items := make([]service.PlayAdminQuizQuestion, 0, limit)
	for rows.Next() {
		item, scanErr := scanAdminQuizQuestion(rows)
		if scanErr != nil {
			return nil, 0, service.PlayAdminQuizQuestionStats{}, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, service.PlayAdminQuizQuestionStats{}, fmt.Errorf("iterate admin quiz questions: %w", err)
	}
	return items, total, stats, nil
}

func (r *playRepository) getAdminQuizQuestionStats(ctx context.Context) (service.PlayAdminQuizQuestionStats, error) {
	exec := r.sqlExec(ctx)
	var stats service.PlayAdminQuizQuestionStats
	if err := scanSingleRow(ctx, exec, `
		SELECT COUNT(*)::int,
		       COUNT(*) FILTER (WHERE active = TRUE)::int,
		       COUNT(*) FILTER (WHERE active = FALSE)::int,
		       COUNT(*) FILTER (WHERE active = TRUE AND language = 'zh')::int,
		       COUNT(*) FILTER (WHERE active = TRUE AND language = 'en')::int
		FROM play_quiz_questions`, nil, &stats.Total, &stats.Active, &stats.Inactive, &stats.ZhActive, &stats.EnActive); err != nil {
		return stats, fmt.Errorf("count admin quiz question stats: %w", err)
	}
	if err := scanSingleRow(ctx, exec, `
		SELECT COALESCE(array_agg(DISTINCT category ORDER BY category) FILTER (WHERE COALESCE(category, '') <> ''), '{}')::text[]
		FROM play_quiz_questions`, nil, pq.Array(&stats.Categories)); err != nil {
		return stats, fmt.Errorf("list admin quiz categories: %w", err)
	}
	if err := scanSingleRow(ctx, exec, `
		SELECT COALESCE(array_agg(DISTINCT difficulty ORDER BY difficulty) FILTER (WHERE COALESCE(difficulty, '') <> ''), '{}')::text[]
		FROM play_quiz_questions`, nil, pq.Array(&stats.Difficulties)); err != nil {
		return stats, fmt.Errorf("list admin quiz difficulties: %w", err)
	}
	return stats, nil
}

func (r *playRepository) CreateAdminQuizQuestion(ctx context.Context, input service.PlayAdminQuizQuestionInput) (*service.PlayAdminQuizQuestion, error) {
	options, err := json.Marshal(input.Options)
	if err != nil {
		return nil, fmt.Errorf("marshal admin quiz options: %w", err)
	}
	return scanAdminQuizQuestionFromQuery(ctx, r.sqlExec(ctx), `
		INSERT INTO play_quiz_questions
		    (language, prompt, options, correct_index, category, difficulty, explanation, sort_order, active, updated_at)
		VALUES ($1, $2, $3::jsonb, $4, $5, $6, $7, $8, $9, NOW())
		RETURNING id, language, prompt, options, correct_index,
		          COALESCE(category, ''), COALESCE(difficulty, 'normal'), COALESCE(explanation, ''),
		          sort_order, active, created_at, COALESCE(updated_at, created_at)`,
		[]any{input.Language, input.Prompt, string(options), input.CorrectIndex, input.Category, input.Difficulty, input.Explanation, input.SortOrder, input.Active},
	)
}

func (r *playRepository) UpdateAdminQuizQuestion(ctx context.Context, id int64, input service.PlayAdminQuizQuestionInput) (*service.PlayAdminQuizQuestion, error) {
	options, err := json.Marshal(input.Options)
	if err != nil {
		return nil, fmt.Errorf("marshal admin quiz options: %w", err)
	}
	return scanAdminQuizQuestionFromQuery(ctx, r.sqlExec(ctx), `
		UPDATE play_quiz_questions
		SET language = $2,
		    prompt = $3,
		    options = $4::jsonb,
		    correct_index = $5,
		    category = $6,
		    difficulty = $7,
		    explanation = $8,
		    sort_order = $9,
		    active = $10,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, language, prompt, options, correct_index,
		          COALESCE(category, ''), COALESCE(difficulty, 'normal'), COALESCE(explanation, ''),
		          sort_order, active, created_at, COALESCE(updated_at, created_at)`,
		[]any{id, input.Language, input.Prompt, string(options), input.CorrectIndex, input.Category, input.Difficulty, input.Explanation, input.SortOrder, input.Active},
	)
}

func (r *playRepository) DeleteAdminQuizQuestion(ctx context.Context, id int64) error {
	result, err := r.sqlExec(ctx).ExecContext(ctx, `DELETE FROM play_quiz_questions WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete admin quiz question: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete admin quiz question rows affected: %w", err)
	}
	if rows == 0 {
		return infraerrors.BadRequest("PLAY_QUIZ_QUESTION_NOT_FOUND", "题目不存在")
	}
	return nil
}

func adminQuizQuestionWhere(filter service.PlayAdminQuizQuestionFilter) (string, []any) {
	clauses := make([]string, 0, 5)
	args := make([]any, 0, 5)
	add := func(clause string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(clause, len(args)))
	}
	if filter.Language != "" {
		add("language = $%d", filter.Language)
	}
	if filter.Active != nil {
		add("active = $%d", *filter.Active)
	}
	if filter.Category != "" {
		add("category = $%d", filter.Category)
	}
	if filter.Difficulty != "" {
		add("difficulty = $%d", filter.Difficulty)
	}
	if filter.Query != "" {
		add("(prompt ILIKE '%%' || $%[1]d || '%%' OR explanation ILIKE '%%' || $%[1]d || '%%')", filter.Query)
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func scanAdminQuizQuestionFromQuery(ctx context.Context, exec sqlQueryer, query string, args []any) (*service.PlayAdminQuizQuestion, error) {
	rows, err := exec.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("admin quiz question query: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("admin quiz question rows: %w", err)
		}
		return nil, infraerrors.BadRequest("PLAY_QUIZ_QUESTION_NOT_FOUND", "题目不存在")
	}
	item, err := scanAdminQuizQuestion(rows)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("admin quiz question rows: %w", err)
	}
	return &item, nil
}

func scanAdminQuizQuestion(scanner interface {
	Scan(dest ...any) error
}) (service.PlayAdminQuizQuestion, error) {
	var item service.PlayAdminQuizQuestion
	var optionsRaw []byte
	if err := scanner.Scan(
		&item.ID, &item.Language, &item.Prompt, &optionsRaw, &item.CorrectIndex,
		&item.Category, &item.Difficulty, &item.Explanation,
		&item.SortOrder, &item.Active, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return item, infraerrors.BadRequest("PLAY_QUIZ_QUESTION_NOT_FOUND", "题目不存在")
		}
		return item, fmt.Errorf("scan admin quiz question: %w", err)
	}
	if err := json.Unmarshal(optionsRaw, &item.Options); err != nil {
		return item, fmt.Errorf("decode admin quiz options: %w", err)
	}
	if item.Options == nil {
		item.Options = []string{}
	}
	return item, nil
}
