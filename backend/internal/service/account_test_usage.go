package service

import (
	"context"
	"math"
	"strings"

	"github.com/tidwall/gjson"
)

type accountTestUsageKey struct{}

type accountTestUsageCollector struct {
	model    string
	requests []*accountTestUsage
}

type accountTestUsage struct {
	protocol    string
	model       string
	serviceTier string
	tokens      UsageTokens
	input       int
	output      int
	thoughts    int
	seen        bool
	inputSeen   bool
	outputSeen  bool
	complete    bool
}

func startAccountTestUsageStream(ctx context.Context, protocol string) *accountTestUsage {
	collector, _ := ctx.Value(accountTestUsageKey{}).(*accountTestUsageCollector)
	if collector == nil {
		return nil
	}
	usage := &accountTestUsage{protocol: protocol, model: collector.model}
	collector.requests = append(collector.requests, usage)
	return usage
}

func (u *accountTestUsage) read(raw string) {
	if u == nil {
		return
	}
	data := gjson.Parse(raw)
	eventType := data.Get("type").String()
	if eventType == "message_stop" || eventType == "response.completed" || eventType == "response.done" || eventType == "response.failed" || eventType == "response.incomplete" {
		u.complete = true
	}
	if response := data.Get("response"); response.IsObject() {
		data = response
	} else if message := data.Get("message"); message.IsObject() {
		data = message
	}
	protocol := u.protocol
	if protocol == "antigravity" {
		switch {
		case data.Get("usageMetadata").IsObject() || len(data.Get("candidates").Array()) > 0:
			protocol = "gemini"
		case data.Get("usage").IsObject() || strings.HasPrefix(eventType, "message_"):
			protocol = "anthropic"
		}
		if protocol != "antigravity" {
			u.protocol = protocol
		}
	}
	if model := strings.TrimSpace(data.Get("model").String()); model != "" {
		u.model = model
	}
	if tier := data.Get("service_tier").String(); tier != "" {
		u.serviceTier = tier
	}
	usage := data.Get("usage")
	if protocol == "gemini" {
		usage = data.Get("usageMetadata")
		for _, candidate := range data.Get("candidates").Array() {
			if candidate.Get("finishReason").String() != "" {
				u.complete = true
			}
		}
	}
	for _, choice := range data.Get("choices").Array() {
		if choice.Get("finish_reason").String() != "" && choice.Get("finish_reason").Type != gjson.Null {
			u.complete = true
		}
	}
	if !usage.IsObject() {
		return
	}
	set := func(field string, target *int) {
		value := usage.Get(field)
		if value.Type != gjson.Number || value.Float() < 0 {
			return
		}
		*target = int(value.Int())
		u.seen = true
		if target == &u.input {
			u.inputSeen = true
		}
		if target == &u.output {
			u.outputSeen = true
		}
	}
	switch protocol {
	case "anthropic":
		set("input_tokens", &u.input)
		set("output_tokens", &u.output)
		set("cache_read_input_tokens", &u.tokens.CacheReadTokens)
		set("cache_creation_input_tokens", &u.tokens.CacheCreationTokens)
		set("cache_creation.ephemeral_5m_input_tokens", &u.tokens.CacheCreation5mTokens)
		set("cache_creation.ephemeral_1h_input_tokens", &u.tokens.CacheCreation1hTokens)
	case "gemini":
		set("promptTokenCount", &u.input)
		set("candidatesTokenCount", &u.output)
		set("thoughtsTokenCount", &u.thoughts)
		set("cachedContentTokenCount", &u.tokens.CacheReadTokens)
	case "chat":
		set("prompt_tokens", &u.input)
		set("completion_tokens", &u.output)
		set("prompt_tokens_details.cached_tokens", &u.tokens.CacheReadTokens)
		set("prompt_cache_hit_tokens", &u.tokens.CacheReadTokens)
	default:
		set("input_tokens", &u.input)
		set("output_tokens", &u.output)
		set("input_tokens_details.cached_tokens", &u.tokens.CacheReadTokens)
		set("cache_read_input_tokens", &u.tokens.CacheReadTokens)
		set("cache_creation_input_tokens", &u.tokens.CacheCreationTokens)
	}
	u.tokens.InputTokens = u.input
	if protocol != "anthropic" {
		u.tokens.InputTokens = max(0, u.input-u.tokens.CacheReadTokens-u.tokens.CacheCreationTokens)
	}
	u.tokens.OutputTokens = u.output + u.thoughts
}

func (c *accountTestUsageCollector) cost(billing *BillingService, accountCostMultiplier float64) (*float64, bool) {
	var total float64
	priced, incomplete := false, len(c.requests) == 0
	for _, usage := range c.requests {
		if !usage.seen || billing == nil {
			incomplete = true
			continue
		}
		cost, err := billing.CalculateCostWithServiceTier(usage.model, usage.tokens, 1, usage.serviceTier)
		if err != nil || cost == nil {
			incomplete = true
			continue
		}
		amount := cost.TotalCost * accountCostMultiplier
		if math.IsNaN(amount) || math.IsInf(amount, 0) || amount < 0 {
			incomplete = true
			continue
		}
		total += amount
		priced = true
		incomplete = incomplete || !usage.complete || !usage.inputSeen || !usage.outputSeen
	}
	if !priced {
		return nil, true
	}
	return &total, incomplete
}
