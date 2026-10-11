package service

import (
	"bytes"
	"context"
	"slices"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/tidwall/gjson"
)

// HasSubscriptionOutputEvidence preserves unpriced observations from an admitted
// send. It never authorizes a charge or changes the legacy metered-usage rule.
func (r *OpenAIForwardResult) HasSubscriptionOutputEvidence(ctx context.Context) bool {
	return r != nil && hasSubscriptionOutputEvidence(ctx, &r.Usage)
}

func hasSubscriptionOutputEvidence(ctx context.Context, usage *OpenAIUsage) bool {
	return usage != nil && usage.UnpricedOutputUsage != nil && requestledger.CurrentSubscriptionReceipt(ctx) != nil
}

func mergeOpenAIUnpricedOutputUsage(dst *OpenAIUsage, src *OpenAIUnpricedOutputUsage) {
	if dst == nil || src == nil {
		return
	}
	if dst.UnpricedOutputUsage == nil {
		dst.UnpricedOutputUsage = &OpenAIUnpricedOutputUsage{}
	}
	d := dst.UnpricedOutputUsage
	d.InputAudioTokens = max(d.InputAudioTokens, src.InputAudioTokens)
	d.OutputAudioTokens = max(d.OutputAudioTokens, src.OutputAudioTokens)
	d.CacheReadAudioTokens = max(d.CacheReadAudioTokens, src.CacheReadAudioTokens)
	d.CacheCreationAudioTokens = max(d.CacheCreationAudioTokens, src.CacheCreationAudioTokens)
	d.SearchCalls = max(d.SearchCalls, src.SearchCalls)
	for _, kind := range src.OutputTypes {
		if !slices.Contains(d.OutputTypes, kind) {
			d.OutputTypes = append(d.OutputTypes, kind)
		}
	}
}

// Output evidence survives progressive/terminal aggregate replacement. Counts
// here are observations, not extra billable units; existing tool pricing stays
// on its original fields and deduplicating counters.
func observeOpenAIUnpricedOutput(usage *OpenAIUsage, data []byte) {
	if usage == nil || (!bytes.Contains(data, []byte(`"output"`)) && !bytes.Contains(data, []byte(`"item"`)) && !bytes.Contains(data, []byte(`"part"`))) || !gjson.ValidBytes(data) {
		return
	}
	evidence := &OpenAIUnpricedOutputUsage{SearchCalls: countGrokNativeSearchCallsInSSEData(data)}
	add := func(kind string) {
		if kind == "" {
			kind = "unknown"
		}
		if !slices.Contains(evidence.OutputTypes, kind) {
			evidence.OutputTypes = append(evidence.OutputTypes, kind)
		}
	}
	contentPart := func(v gjson.Result) {
		kind := strings.TrimSpace(v.Get("type").String())
		if kind != "output_text" && kind != "refusal" {
			add(kind)
		}
	}
	item := func(v gjson.Result) {
		switch kind := strings.TrimSpace(v.Get("type").String()); kind {
		case "reasoning", "compaction":
		case "message":
			for _, part := range v.Get("content").Array() {
				contentPart(part)
			}
		default:
			add(kind)
		}
	}
	for _, path := range []string{"output", "response.output", "data.output", "data.response.output"} {
		for _, v := range gjson.GetBytes(data, path).Array() {
			item(v)
		}
	}
	if v := gjson.GetBytes(data, "item"); v.IsObject() {
		item(v)
	}
	if eventType := gjson.GetBytes(data, "type").String(); eventType == "response.content_part.added" || eventType == "response.content_part.done" {
		if part := gjson.GetBytes(data, "part"); part.IsObject() {
			contentPart(part)
		}
	}
	if evidence.SearchCalls > 0 || len(evidence.OutputTypes) > 0 {
		mergeOpenAIUnpricedOutputUsage(usage, evidence)
	}
}
