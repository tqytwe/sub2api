package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDedupeQuizQuestionsByTemplate(t *testing.T) {
	opts := `["A","B","C","D"]`
	questions := []PlayQuizQuestionDB{
		{ID: 1, Prompt: "Stem（第1题）", OptionsJSON: opts, CorrectIndex: 0},
		{ID: 2, Prompt: "Stem（第2题）", OptionsJSON: opts, CorrectIndex: 0},
		{ID: 3, Prompt: "Other question", OptionsJSON: `["X","Y"]`, CorrectIndex: 1},
	}
	date := time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)

	deduped := dedupeQuizQuestionsByTemplate(questions, 42, date, "zh")
	require.Len(t, deduped, 2)

	seen := map[int64]struct{}{}
	for _, q := range deduped {
		seen[q.ID] = struct{}{}
	}
	require.Len(t, seen, 2)
	require.Contains(t, seen, int64(3))
}

func TestPickDailyQuizQuestionsReturnsDistinctTemplates(t *testing.T) {
	s := &PlayService{}
	pool := make([]PlayQuizQuestionDB, 0, 50)
	for i := 1; i <= 10; i++ {
		opts := fmt.Sprintf(`["A%d","B%d","C%d","D%d"]`, i, i, i, i)
		for v := 1; v <= 5; v++ {
			pool = append(pool, PlayQuizQuestionDB{
				ID:           int64(i*10 + v),
				Prompt:       fmt.Sprintf("Prompt %d variant %d", i, v),
				OptionsJSON:  opts,
				CorrectIndex: 0,
			})
		}
	}
	date := time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)
	picked := s.pickDailyQuizQuestions(pool, 5, 99, date, "zh")
	require.Len(t, picked, 5)

	keys := make(map[string]struct{}, len(picked))
	for _, q := range picked {
		keys[quizTemplateKey(q)] = struct{}{}
	}
	require.Len(t, keys, 5, "daily quiz should not repeat the same template")
}
