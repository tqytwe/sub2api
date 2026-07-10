package service

// ArenaBillableTokens returns usage tokens counted for arena ranking:
// input + output + cache creation, excluding cache read (cached) tokens.
func ArenaBillableTokens(input, output, cacheCreation int) int64 {
	sum := int64(input) + int64(output) + int64(cacheCreation)
	if sum < 0 {
		return 0
	}
	return sum
}

// ArenaBillableTokensFromUsageLog aggregates billable tokens from a usage log row.
func ArenaBillableTokensFromUsageLog(log UsageLog) int64 {
	return ArenaBillableTokens(log.InputTokens, log.OutputTokens, log.CacheCreationTokens)
}
