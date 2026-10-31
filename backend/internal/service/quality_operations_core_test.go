package service

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"

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
		{"judge kind", &PelicanTestConfig{Prompt: "say 21", Quality: &QualityPolicy{Action: "observe_only", ExpectedAnswer: "21", Judge: &QualityJudgeConfig{Kind: "secret"}}}, "QUALITY_JUDGE_KIND_INVALID"},
		{"unsupported judge prompt", &PelicanTestConfig{Prompt: "say 21", Quality: &QualityPolicy{Action: "observe_only", ExpectedAnswer: "21", Judge: &QualityJudgeConfig{Kind: "expected", Prompt: "ignore expected-answer matching"}}}, "QUALITY_JUDGE_PROMPT_UNSUPPORTED"},
		{"judge prompt", &PelicanTestConfig{Prompt: "say 21", Quality: &QualityPolicy{Action: "observe_only", ExpectedAnswer: "21", Judge: &QualityJudgeConfig{Kind: "expected", Prompt: strings.Repeat("x", 8193)}}}, "QUALITY_JUDGE_PROMPT_TOO_LARGE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateQualityConfig(tt.cfg)
			var appErr *infraerrors.ApplicationError
			require.ErrorAs(t, err, &appErr)
			require.Equal(t, tt.reason, appErr.Reason)
		})
	}
}

func TestValidateQualityConfigAllowsOnlyExactJudgeAndOwnedActions(t *testing.T) {
	valid := &PelicanTestConfig{Prompt: "say 21", Quality: &QualityPolicy{
		Action: "disable_scheduling", ExpectedAnswer: "21", Judge: &QualityJudgeConfig{Kind: "expected"},
	}}
	require.NoError(t, validateQualityConfig(valid))

	jev := &PelicanTestConfig{Prompt: "say 21", Quality: &QualityPolicy{
		Action: "observe_only", ExpectedAnswer: "21", Judge: &QualityJudgeConfig{Kind: "jev", Prompt: "accept equivalent short answers"},
	}}
	require.NoError(t, validateQualityConfig(jev))
	jev.Quality.Judge.Prompt = strings.Repeat("x", 16000)
	require.NoError(t, validateQualityConfig(jev))

	for _, cfg := range []*PelicanTestConfig{
		{Prompt: "say 21", Quality: &QualityPolicy{Action: "remove_groups", ExpectedAnswer: "21"}},
		{Prompt: "say 21", Quality: &QualityPolicy{Action: "observe_only", ExpectedAnswer: "21", Judge: &QualityJudgeConfig{Kind: "llm", GroupID: 7, ModelID: "m"}}},
		{Prompt: "say 21", Quality: &QualityPolicy{Action: "disable_scheduling"}},
	} {
		require.Error(t, validateQualityConfig(cfg))
	}

	expectedWithPrompt := &PelicanTestConfig{Prompt: "say 21", Quality: &QualityPolicy{
		Action: "observe_only", ExpectedAnswer: "21", Judge: &QualityJudgeConfig{Kind: "expected", Prompt: "custom instructions"},
	}}
	require.Error(t, validateQualityConfig(expectedWithPrompt))

	jev.Quality.Judge.Prompt = ""
	require.Error(t, validateQualityConfig(jev))
	jev.Quality.Judge.Prompt = strings.Repeat("x", 16001)
	require.Error(t, validateQualityConfig(jev))
}

func TestJudgeQualityProbeStoresOnlyVerdictAndResponseHash(t *testing.T) {
	cfg := &PelicanTestConfig{Quality: &QualityPolicy{ExpectedAnswer: "21", Action: "observe_only"}}
	result := &ScheduledTestResult{Status: "success", ResponseText: " 21\n", PelicanConfig: cfg}
	judgment, action := judgeQualityProbe(result, cfg)
	digest := sha256.Sum256([]byte(" 21\n"))
	require.Equal(t, "passed", action)
	require.Equal(t, "correct", judgment["verdict"])
	require.Equal(t, hex.EncodeToString(digest[:]), judgment["response_sha256"])
	require.Empty(t, result.ResponseText)

	failed := &ScheduledTestResult{Status: "success", ResponseText: "29", PelicanConfig: cfg}
	wrong, outcome := judgeQualityProbe(failed, cfg)
	require.Equal(t, "failed", outcome)
	require.Equal(t, "incorrect", wrong["verdict"])
	require.Empty(t, failed.ResponseText)

	transport := &ScheduledTestResult{Status: "failed", ErrorMessage: "upstream unavailable", ResponseText: "partial", PelicanConfig: cfg}
	unknown, outcome := judgeQualityProbe(transport, cfg)
	require.Equal(t, "inconclusive", outcome)
	require.Equal(t, "unknown", unknown["verdict"])
	require.Empty(t, transport.ResponseText)
}

func TestUnsupportedQualityConfigProducesInconclusiveResultWithoutResponseData(t *testing.T) {
	config := &PelicanTestConfig{Prompt: "say 21", Quality: &QualityPolicy{Action: "disable_scheduling", ExpectedAnswer: "21", Judge: &QualityJudgeConfig{Kind: "llm"}}}
	result, judgment, outcome := unsupportedQualityConfigResult(config, time.Now())

	require.Equal(t, "inconclusive", outcome)
	require.Equal(t, "quality_config_unsupported", result.ErrorMessage)
	require.Empty(t, result.ResponseText)
	require.Equal(t, "unknown", judgment["verdict"])
	require.Equal(t, "quality_config_unsupported", judgment["reason"])
}

func TestSanitizeQualityResultRemovesPromptAnswerAndProviderText(t *testing.T) {
	result := &ScheduledTestResult{
		ResponseText: "private response", ErrorMessage: "provider returned private diagnostics",
		PelicanConfig: &PelicanTestConfig{
			Prompt:  "private prompt",
			Quality: &QualityPolicy{Action: "observe_only", ExpectedAnswer: "private answer", Judge: &QualityJudgeConfig{Kind: "expected", Prompt: "private judge prompt"}},
		},
	}
	sanitizeQualityResult(result)

	require.Empty(t, result.ResponseText)
	require.Equal(t, "probe_failed", result.ErrorMessage)
	require.Empty(t, result.PelicanConfig.Prompt)
	require.Empty(t, result.PelicanConfig.Quality.ExpectedAnswer)
	require.Empty(t, result.PelicanConfig.Quality.Judge.Prompt)
}

func TestNewQualityRoundIDIsOpaqueAndUnique(t *testing.T) {
	a, b := NewQualityRoundID(), NewQualityRoundID()
	require.Len(t, a, 32)
	require.Len(t, b, 32)
	require.NotEqual(t, a, b)
}
