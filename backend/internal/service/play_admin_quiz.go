package service

import (
	"context"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	PlayQuizDifficultyEasy   = "easy"
	PlayQuizDifficultyNormal = "normal"
	PlayQuizDifficultyHard   = "hard"
)

func (s *PlayService) ListAdminQuizQuestions(ctx context.Context, filter PlayAdminQuizQuestionFilter) ([]PlayAdminQuizQuestion, int, PlayAdminQuizQuestionStats, error) {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	filter.Language = normalizeAdminQuizLanguageFilter(filter.Language)
	filter.Category = strings.TrimSpace(filter.Category)
	filter.Difficulty = normalizeAdminQuizDifficultyFilter(filter.Difficulty)
	filter.Query = strings.TrimSpace(filter.Query)
	return s.repo.ListAdminQuizQuestions(ctx, filter)
}

func (s *PlayService) CreateAdminQuizQuestion(ctx context.Context, input PlayAdminQuizQuestionInput) (*PlayAdminQuizQuestion, error) {
	normalized, err := normalizeAdminQuizQuestionInput(input)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateAdminQuizQuestion(ctx, normalized)
}

func (s *PlayService) UpdateAdminQuizQuestion(ctx context.Context, id int64, input PlayAdminQuizQuestionInput) (*PlayAdminQuizQuestion, error) {
	if id <= 0 {
		return nil, infraerrors.BadRequest("PLAY_QUIZ_QUESTION_INVALID", "题目 ID 无效")
	}
	normalized, err := normalizeAdminQuizQuestionInput(input)
	if err != nil {
		return nil, err
	}
	return s.repo.UpdateAdminQuizQuestion(ctx, id, normalized)
}

func (s *PlayService) DeleteAdminQuizQuestion(ctx context.Context, id int64) error {
	if id <= 0 {
		return infraerrors.BadRequest("PLAY_QUIZ_QUESTION_INVALID", "题目 ID 无效")
	}
	return s.repo.DeleteAdminQuizQuestion(ctx, id)
}

func normalizeAdminQuizQuestionInput(input PlayAdminQuizQuestionInput) (PlayAdminQuizQuestionInput, error) {
	input.Language = normalizeQuizLanguage(input.Language)
	input.Prompt = strings.TrimSpace(input.Prompt)
	input.Category = strings.TrimSpace(input.Category)
	input.Difficulty = normalizeAdminQuizDifficultyFilter(input.Difficulty)
	input.Explanation = strings.TrimSpace(input.Explanation)
	if input.Prompt == "" {
		return input, infraerrors.BadRequest("PLAY_QUIZ_PROMPT_REQUIRED", "题干不能为空")
	}
	if len(input.Options) != 4 {
		return input, infraerrors.BadRequest("PLAY_QUIZ_OPTIONS_INVALID", "必须填写 4 个选项")
	}
	seen := make(map[string]struct{}, len(input.Options))
	for i := range input.Options {
		input.Options[i] = strings.TrimSpace(input.Options[i])
		if input.Options[i] == "" {
			return input, infraerrors.BadRequest("PLAY_QUIZ_OPTIONS_INVALID", "选项不能为空")
		}
		key := strings.ToLower(input.Options[i])
		if _, ok := seen[key]; ok {
			return input, infraerrors.BadRequest("PLAY_QUIZ_OPTIONS_DUPLICATE", "选项不能重复")
		}
		seen[key] = struct{}{}
	}
	if input.CorrectIndex < 0 || input.CorrectIndex >= len(input.Options) {
		return input, infraerrors.BadRequest("PLAY_QUIZ_CORRECT_INDEX_INVALID", "正确答案无效")
	}
	if input.Category == "" {
		input.Category = "平台知识"
	}
	if input.Difficulty == "" {
		input.Difficulty = PlayQuizDifficultyNormal
	}
	return input, nil
}

func normalizeAdminQuizLanguageFilter(language string) string {
	lang := strings.ToLower(strings.TrimSpace(language))
	switch lang {
	case "", "all":
		return ""
	case "zh", "en":
		return lang
	default:
		return normalizeQuizLanguage(lang)
	}
}

func normalizeAdminQuizDifficultyFilter(difficulty string) string {
	switch strings.ToLower(strings.TrimSpace(difficulty)) {
	case "", "all":
		return ""
	case PlayQuizDifficultyEasy:
		return PlayQuizDifficultyEasy
	case PlayQuizDifficultyHard:
		return PlayQuizDifficultyHard
	default:
		return PlayQuizDifficultyNormal
	}
}
