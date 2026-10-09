package dto

import "github.com/Wei-Shaw/sub2api/internal/service"

// usageOutputTPS follows ranxi2001/sub2api v2.10.3's per-record denominator.
// Output can include reasoning before the first visible token, or buffered tool
// output delivered at completion, so subtracting FirstTokenMs would misstate
// generation speed. Streaming and non-streaming use the same recorded duration.
// Derive only at the API boundary: historical records and charges are untouched.
func usageOutputTPS(log *service.UsageLog) *float64 {
	if log == nil || log.OutputTokens < 2 || log.DurationMs == nil || *log.DurationMs <= 0 {
		return nil
	}
	if log.ImageCount > 0 || log.ImageOutputTokens > 0 || log.VideoCount > 0 {
		return nil
	}
	for _, kind := range []*string{log.BillingMode, log.MediaType} {
		if kind != nil && (*kind == "image" || *kind == "video") {
			return nil
		}
	}
	tps := float64(log.OutputTokens) / (float64(*log.DurationMs) / 1000)
	return &tps
}
