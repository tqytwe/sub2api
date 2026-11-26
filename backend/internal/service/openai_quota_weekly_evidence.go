package service

import (
	"encoding/json"
	"time"
)

// A malformed week invalidates older evidence. Malformed unrelated windows
// do not affect independently identified weekly observations.
func openAIQuotaWeekly429Evidence(body []byte, expectedAccount string, observed, received time.Time) map[string]any {
	var payload struct {
		AccountID string `json:"account_id"`
		RateLimit *struct {
			Primary   json.RawMessage `json:"primary_window"`
			Secondary json.RawMessage `json:"secondary_window"`
		} `json:"rate_limit"`
	}
	if !openAIEncryptedJSONUnambiguous(body) || json.Unmarshal(body, &payload) != nil || payload.RateLimit == nil || identityValuesConflict(expectedAccount, payload.AccountID) {
		return nil
	}
	var result map[string]any
	for _, raw := range []json.RawMessage{payload.RateLimit.Primary, payload.RateLimit.Secondary} {
		var fields map[string]json.RawMessage
		var seconds int64
		if json.Unmarshal(raw, &fields) != nil || json.Unmarshal(fields["limit_window_seconds"], &seconds) != nil || seconds != 7*24*60*60 {
			continue
		}
		invalid := openAI429Evidence(0, observed, time.Time{}, false)
		var used *float64
		if result != nil || json.Unmarshal(fields["used_percent"], &used) != nil || used == nil || !validOpenAI429Percent(*used) {
			return invalid
		}
		var reset time.Time
		if raw, exists := fields["reset_after_seconds"]; exists {
			var relative int64
			if json.Unmarshal(raw, &relative) != nil || relative <= 0 || relative > seconds {
				return invalid
			}
			reset = received.Add(time.Duration(relative) * time.Second)
		}
		if raw, exists := fields["reset_at"]; exists {
			var absolute int64
			if json.Unmarshal(raw, &absolute) != nil || !time.Unix(absolute, 0).After(received) {
				return invalid
			}
			reset = time.Unix(absolute, 0)
		}
		result = openAI429Evidence(*used, observed, reset, true)
	}
	return result
}
