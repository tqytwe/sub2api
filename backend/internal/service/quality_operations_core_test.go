package service

import (
	"strings"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestValidateQualityConfigRejectsUnsafeOrUnboundedInputs(t *testing.T) {
	tests := []struct {
		name   string
		cfg    *PelicanTestConfig
		reason string
	}{
		{"prompt", &PelicanTestConfig{Prompt: strings.Repeat("x", 32769)}, "QUALITY_PROMPT_TOO_LARGE"},
		{"parallel", &PelicanTestConfig{ParallelCount: 33}, "QUALITY_PARALLEL_COUNT_INVALID"},
		{"judge kind", &PelicanTestConfig{Quality: &QualityPolicy{Judge: &QualityJudgeConfig{Kind: "secret"}}}, "QUALITY_JUDGE_KIND_INVALID"},
		{"judge prompt", &PelicanTestConfig{Quality: &QualityPolicy{Judge: &QualityJudgeConfig{Prompt: strings.Repeat("x", 8193)}}}, "QUALITY_JUDGE_PROMPT_TOO_LARGE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateQualityConfig(tt.cfg)
			require.Equal(t, tt.reason, err.(*infraerrors.ApplicationError).Reason)
		})
	}
}

func TestNewQualityRoundIDIsOpaqueAndUnique(t *testing.T) {
	a, b := NewQualityRoundID(), NewQualityRoundID()
	require.Len(t, a, 32)
	require.Len(t, b, 32)
	require.NotEqual(t, a, b)
}
