package service

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func recordAccountTestSSE(ctx context.Context, protocol, model string, body []byte) {
	usage := startAccountTestUsageStream(ctx, protocol)
	if model = strings.TrimSpace(model); model != "" {
		usage.model = model
	}
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if data, ok := strings.CutPrefix(line, "data:"); ok {
			line = strings.TrimSpace(data)
		}
		if line != "" && line != "[DONE]" {
			usage.read(line)
		}
	}
}

func TestAccountTestStreamParsersCollectProviderUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name     string
		protocol string
		stream   string
		process  func(*AccountTestService, *gin.Context, string) error
	}{
		{
			name: "responses", protocol: "responses",
			stream: "data: {\"type\":\"response.completed\",\"response\":{\"model\":\"probe-model\",\"usage\":{\"input_tokens\":12,\"output_tokens\":7}}}\n",
			process: func(s *AccountTestService, c *gin.Context, body string) error {
				return s.processOpenAIStream(c, strings.NewReader(body))
			},
		},
		{
			name: "anthropic", protocol: "anthropic",
			stream: "data: {\"type\":\"message_stop\",\"message\":{\"model\":\"probe-model\",\"usage\":{\"input_tokens\":12,\"output_tokens\":7}}}\n",
			process: func(s *AccountTestService, c *gin.Context, body string) error {
				return s.processClaudeStream(c, strings.NewReader(body))
			},
		},
		{
			name: "chat", protocol: "chat",
			stream: "data: {\"choices\":[{\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":12,\"completion_tokens\":7}}\n",
			process: func(s *AccountTestService, c *gin.Context, body string) error {
				return s.processOpenAIChatCompletionsStream(c, strings.NewReader(body))
			},
		},
		{
			name: "gemini", protocol: "gemini",
			stream: "data: {\"candidates\":[{\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":12,\"candidatesTokenCount\":7}}\n",
			process: func(s *AccountTestService, c *gin.Context, body string) error {
				return s.processGeminiStream(c, strings.NewReader(body))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := &accountTestUsageCollector{model: "probe-model"}
			ctx := context.WithValue(context.Background(), accountTestUsageKey{}, collector)
			recorder := httptest.NewRecorder()
			ginContext, _ := gin.CreateTestContext(recorder)
			ginContext.Request = httptest.NewRequest("GET", "/", nil).WithContext(ctx)
			require.NoError(t, tt.process(&AccountTestService{}, ginContext, tt.stream))
			require.Len(t, collector.requests, 1)
			usage := collector.requests[0]
			require.Equal(t, tt.protocol, usage.protocol)
			require.Equal(t, 12, usage.input)
			require.Equal(t, 7, usage.output)
			require.True(t, usage.complete)
		})
	}
}

func TestAccountTestUsageCollectorSeparatesStreamsAndRequiresCompleteUsage(t *testing.T) {
	collector := &accountTestUsageCollector{model: "probe-model"}
	ctx := context.WithValue(context.Background(), accountTestUsageKey{}, collector)

	usage := startAccountTestUsageStream(ctx, "chat")
	usage.read(`{"choices":[{"finish_reason":null}],"usage":{"prompt_tokens":100,"completion_tokens":20,"prompt_tokens_details":{"cached_tokens":30}}}`)
	usage.read(`{"choices":[{"finish_reason":"stop"}]}`)

	require.Len(t, collector.requests, 1)
	usage = collector.requests[0]
	require.Equal(t, "probe-model", usage.model)
	require.Equal(t, 70, usage.tokens.InputTokens)
	require.Equal(t, 30, usage.tokens.CacheReadTokens)
	require.Equal(t, 20, usage.tokens.OutputTokens)
	require.True(t, usage.inputSeen)
	require.True(t, usage.outputSeen)
	require.True(t, usage.complete)
}

func TestAccountTestUsageCollectorReadsProviderUsageFormats(t *testing.T) {
	tests := []struct {
		name     string
		protocol string
		body     string
		input    int
		output   int
		cache    int
	}{
		{name: "responses", protocol: "responses", body: `{"type":"response.completed","response":{"model":"mapped-model","usage":{"input_tokens":12,"output_tokens":7,"input_tokens_details":{"cached_tokens":3}}}}`, input: 9, output: 7, cache: 3},
		{name: "anthropic", protocol: "anthropic", body: `{"type":"message_stop","message":{"model":"claude-probe","usage":{"input_tokens":12,"output_tokens":7,"cache_read_input_tokens":3}}}`, input: 12, output: 7, cache: 3},
		{name: "gemini", protocol: "gemini", body: `{"candidates":[{"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":12,"candidatesTokenCount":7,"thoughtsTokenCount":2,"cachedContentTokenCount":3}}`, input: 9, output: 9, cache: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := &accountTestUsageCollector{model: "configured-model"}
			ctx := context.WithValue(context.Background(), accountTestUsageKey{}, collector)
			recordAccountTestSSE(ctx, tt.protocol, "", []byte(tt.body))
			require.Len(t, collector.requests, 1)
			usage := collector.requests[0]
			require.Equal(t, tt.input, usage.tokens.InputTokens)
			require.Equal(t, tt.output, usage.tokens.OutputTokens)
			require.Equal(t, tt.cache, usage.tokens.CacheReadTokens)
			require.True(t, usage.complete)
		})
	}
}

func TestAccountTestUsageCostUnknownAndPartial(t *testing.T) {
	billing := &BillingService{fallbackPrices: map[string]*ModelPricing{
		"gpt-6-sol": {InputPricePerToken: 0.01, OutputPricePerToken: 0.02},
	}}
	unknown := &accountTestUsageCollector{model: "unpriced-model"}
	unknownCtx := context.WithValue(context.Background(), accountTestUsageKey{}, unknown)
	recordAccountTestSSE(unknownCtx, "chat", "", []byte(`{"choices":[{"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5}}`))
	cost, incomplete := unknown.cost(billing, 1)
	require.Nil(t, cost)
	require.True(t, incomplete)

	partial := &accountTestUsageCollector{model: "gpt-6-sol"}
	partialCtx := context.WithValue(context.Background(), accountTestUsageKey{}, partial)
	recordAccountTestSSE(partialCtx, "chat", "", []byte(`{"choices":[{"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5}}`))
	recordAccountTestSSE(partialCtx, "chat", "", []byte(`{"choices":[{"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2}}`))
	partial.requests[1].complete = false
	partial.requests[1].outputSeen = false
	cost, incomplete = partial.cost(billing, 0.5)
	require.NotNil(t, cost)
	require.InDelta(t, 0.135, *cost, 1e-12)
	require.True(t, incomplete)
}
