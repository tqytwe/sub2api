package service

import (
	"context"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type adminQuizRepo struct {
	PlayRepository
	created *PlayAdminQuizQuestionInput
	listed  PlayAdminQuizQuestionFilter
}

func (r *adminQuizRepo) ListAdminQuizQuestions(_ context.Context, filter PlayAdminQuizQuestionFilter) ([]PlayAdminQuizQuestion, int, PlayAdminQuizQuestionStats, error) {
	r.listed = filter
	now := time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)
	return []PlayAdminQuizQuestion{{
		ID: 1, Language: "zh", Prompt: "什么是 API 网关？", Options: []string{"鉴权和路由", "训练模型", "删除数据库", "生成发票"},
		CorrectIndex: 0, Category: "API 基础", Difficulty: PlayQuizDifficultyEasy, Active: true, CreatedAt: now, UpdatedAt: now,
	}}, 1, PlayAdminQuizQuestionStats{Total: 1, Active: 1, ZhActive: 1, Categories: []string{"API 基础"}, Difficulties: []string{PlayQuizDifficultyEasy}}, nil
}

func (r *adminQuizRepo) CreateAdminQuizQuestion(_ context.Context, input PlayAdminQuizQuestionInput) (*PlayAdminQuizQuestion, error) {
	r.created = &input
	return &PlayAdminQuizQuestion{
		ID:           9,
		Language:     input.Language,
		Prompt:       input.Prompt,
		Options:      input.Options,
		CorrectIndex: input.CorrectIndex,
		Category:     input.Category,
		Difficulty:   input.Difficulty,
		Explanation:  input.Explanation,
		SortOrder:    input.SortOrder,
		Active:       input.Active,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}, nil
}

func (r *adminQuizRepo) UpdateAdminQuizQuestion(_ context.Context, id int64, input PlayAdminQuizQuestionInput) (*PlayAdminQuizQuestion, error) {
	item, err := r.CreateAdminQuizQuestion(context.Background(), input)
	if err != nil {
		return nil, err
	}
	item.ID = id
	return item, nil
}

func (r *adminQuizRepo) DeleteAdminQuizQuestion(context.Context, int64) error { return nil }

func TestPlayAdminQuizQuestionNormalizesAndCreatesQuestion(t *testing.T) {
	repo := &adminQuizRepo{}
	svc := &PlayService{repo: repo}

	created, err := svc.CreateAdminQuizQuestion(context.Background(), PlayAdminQuizQuestionInput{
		Language:     "zh-CN",
		Prompt:       " 充值优惠券应该在哪里使用？ ",
		Options:      []string{" 充值订单 ", "公开代码", "浏览器主题", "随机日志"},
		CorrectIndex: 0,
		Category:     "",
		Difficulty:   "",
		Active:       true,
	})

	require.NoError(t, err)
	require.Equal(t, "zh", created.Language)
	require.Equal(t, "充值订单", created.Options[0])
	require.Equal(t, "平台知识", created.Category)
	require.Equal(t, PlayQuizDifficultyNormal, created.Difficulty)
	require.NotNil(t, repo.created)
}

func TestPlayAdminQuizQuestionRejectsInvalidOptions(t *testing.T) {
	svc := &PlayService{repo: &adminQuizRepo{}}

	_, err := svc.CreateAdminQuizQuestion(context.Background(), PlayAdminQuizQuestionInput{
		Language:     "zh",
		Prompt:       "哪项不能重复？",
		Options:      []string{"A", "A", "B", "C"},
		CorrectIndex: 0,
	})

	require.Error(t, err)
	require.True(t, infraerrors.IsBadRequest(err))
}

func TestPlayAdminQuizQuestionListCapsPaginationAndFilters(t *testing.T) {
	repo := &adminQuizRepo{}
	svc := &PlayService{repo: repo}

	_, total, stats, err := svc.ListAdminQuizQuestions(context.Background(), PlayAdminQuizQuestionFilter{
		Language: "all", Difficulty: "easy", Page: -1, PageSize: 500, Query: " 网关 ",
	})

	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, 1, stats.Active)
	require.Equal(t, "", repo.listed.Language)
	require.Equal(t, 1, repo.listed.Page)
	require.Equal(t, 100, repo.listed.PageSize)
	require.Equal(t, "网关", repo.listed.Query)
	require.Equal(t, PlayQuizDifficultyEasy, repo.listed.Difficulty)
}
