package service

// HasObservedUsage identifies upstream-metered Responses text/image work that
// must be settled even when forwarding fails. Token-detail subsets and output
// deltas alone do not authorize an estimated charge.
func (r *OpenAIForwardResult) HasObservedUsage() bool {
	return r != nil && hasObservedOpenAIUsage(&r.Usage, r.ImageCount)
}

func hasObservedOpenAIUsage(usage *OpenAIUsage, imageCount int) bool {
	return imageCount > 0 || usage != nil && (usage.InputTokens > 0 || usage.OutputTokens > 0)
}
