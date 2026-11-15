package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/gin-gonic/gin"
)

const (
	pelicanErrEmptyOutput   = "Model returned empty output"
	pelicanErrCaptureLimit  = "Response exceeds 4 MiB capture limit"
	pelicanErrHistoryLimit  = "Output exceeds 2 MiB history limit"
	pelicanErrMaxTokens     = "Model output hit max_tokens before finishing"
	pelicanErrRefused       = "Model refused the request"
	pelicanDefaultPrompt    = "创建一个 HTML，内容是 SVG 绘制一个鹈鹕骑自行车的 2D 动画。只输出 HTML，不要使用 Markdown 代码块或外部依赖。"
	PelicanDeliveryContract = "所有账号使用相同交付约定：直接返回独立 HTML，不使用 Markdown 代码块或外部依赖。只输出 HTML，不要解释。"
)

type pelicanAccountTestContextKey struct{}

func normalizePelicanReasoningEffort(value string) string {
	effort := strings.ToLower(strings.TrimSpace(value))
	switch effort {
	case "none", "minimal", "low", "medium", "high", "xhigh", "max", "ultra":
		return effort
	}
	return ""
}

func createPelicanClaudePayload(modelID, prompt, reasoningEffort string) (map[string]any, error) {
	payload, err := createTestPayload(modelID)
	if err != nil {
		return nil, err
	}
	payload["max_tokens"] = 32000
	if effort := pelicanClaudeEffort(modelID, reasoningEffort); effort != "" {
		payload["output_config"] = map[string]any{"effort": effort}
	}
	if messages, ok := payload["messages"].([]map[string]any); ok && len(messages) > 0 {
		if content, ok := messages[0]["content"].([]map[string]any); ok && len(content) > 0 {
			content[0]["text"] = promptOrDefault(prompt)
		}
	}
	return payload, nil
}

func pelicanClaudeEffort(modelID, reasoningEffort string) string {
	effort := normalizePelicanReasoningEffort(reasoningEffort)
	for _, level := range claude.EffortLevelsForModel(modelID) {
		if level == effort {
			return effort
		}
	}
	return ""
}

func createPelicanOpenAIPayload(modelID string, isOAuth bool, prompt, reasoningEffort string) map[string]any {
	payload := createOpenAITestPayload(modelID, isOAuth)
	applyQualityProbePromptToOpenAIPayload(payload, prompt)
	if effort := normalizePelicanReasoningEffort(reasoningEffort); effort != "" {
		payload["reasoning"] = map[string]any{"effort": effort}
	}
	payload["max_output_tokens"] = 32000
	return payload
}

func promptOrDefault(prompt string) string {
	if value := strings.TrimSpace(prompt); value != "" {
		return value
	}
	return "hi"
}

var pelicanHTMLPattern = regexp.MustCompile(`(?i)<(?:!doctype\s+html|html|svg)[\s>]`)

func intelligenceTestPrompt(cfg *PelicanTestConfig) string {
	prompt := strings.TrimSpace(cfg.Prompt)
	if prompt == "" {
		prompt = pelicanDefaultPrompt
	}
	return prompt + "\n\n" + PelicanDeliveryContract
}

func (s *AccountTestService) TestPelicanAccountConnection(c *gin.Context, accountID int64, model, prompt, effort string) error {
	ctx := context.WithValue(c.Request.Context(), qualityProbePromptContextKey{}, prompt)
	ctx = context.WithValue(ctx, qualityProbeEffortContextKey{}, effort)
	ctx = context.WithValue(ctx, pelicanAccountTestContextKey{}, true)
	c.Request = c.Request.WithContext(ctx)
	return s.TestAccountConnection(c, accountID, model, prompt, AccountTestModeDefault)
}

// RunPelicanBackground reuses the Fork's metered quality probe transport, but
// keeps the generated document as the result used by group-test/showcase.
func (s *AccountTestService) RunPelicanBackground(ctx context.Context, accountID int64, model string, cfg *PelicanTestConfig) (*ScheduledTestResult, error) {
	if cfg == nil {
		return nil, ErrChannelMonitorV2InvalidConfig
	}
	prompt := intelligenceTestPrompt(cfg)
	started := time.Now()
	usage := &accountTestUsageCollector{model: model}
	ctx = context.WithValue(ctx, accountTestUsageKey{}, usage)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = (&http.Request{Header: make(http.Header)}).WithContext(ctx)
	testErr := s.TestPelicanAccountConnection(c, accountID, model, prompt, cfg.ReasoningEffort)
	finished := time.Now()
	account, accountErr := s.accountRepo.GetByID(ctx, accountID)
	multiplier := 0.1
	if accountErr == nil && account != nil {
		multiplier = account.CostMultiplier()
	}
	cost, costIncomplete := usage.cost(s.billingService, multiplier)
	responseText, errMsg := parseTestSSEOutput(w.Body.String())
	if testErr != nil && errMsg == "" {
		errMsg = testErr.Error()
	}
	result := &ScheduledTestResult{Status: "success", ResponseText: responseText, LatencyMs: finished.Sub(started).Milliseconds(), StartedAt: started, FinishedAt: finished, CostUSD: cost, CostIncomplete: costIncomplete || accountErr != nil}
	if errMsg != "" {
		result.Status, result.ErrorMessage = "failed", errMsg
	}
	if result.Status == "success" && strings.TrimSpace(result.ResponseText) == "" {
		result.Status = "failed"
		result.ErrorMessage = pelicanErrEmptyOutput
	}
	if result.Status == "success" && !pelicanHTMLPattern.MatchString(result.ResponseText) {
		result.Status = "failed"
		result.ErrorMessage = "Model did not return HTML or SVG"
	}
	if len(result.ResponseText) > 2<<20 {
		result.Status = "failed"
		result.ResponseText = ""
		result.ErrorMessage = pelicanErrHistoryLimit
	}
	if result.StartedAt.IsZero() {
		result.StartedAt = started
	}
	if result.FinishedAt.IsZero() {
		result.FinishedAt = time.Now()
	}
	snapshot := *cfg
	snapshot.ModelID = model
	snapshot.ActualPrompt = prompt
	result.PelicanConfig = &snapshot
	return result, nil
}
