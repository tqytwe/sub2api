package service

// ArenaBillableTokens returns all usage token categories counted by Play.
func ArenaBillableTokens(input, output, cacheCreation, cacheRead int) int64 {
	sum := int64(input) + int64(output) + int64(cacheCreation) + int64(cacheRead)
	if sum < 0 {
		return 0
	}
	return sum
}

// ArenaBillableTokensFromUsageLog aggregates billable tokens from a usage log row.
func ArenaBillableTokensFromUsageLog(log UsageLog) int64 {
	return ArenaBillableTokens(log.InputTokens, log.OutputTokens, log.CacheCreationTokens, log.CacheReadTokens)
}
