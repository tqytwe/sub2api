package service

import "testing"

func TestArenaBillableTokens(t *testing.T) {
	tests := []struct {
		name          string
		input         int
		output        int
		cacheCreation int
		want          int64
	}{
		{
			name:   "excludes cache read by design",
			input:  100,
			output: 50,
			want:   150,
		},
		{
			name:          "includes cache creation",
			input:         10,
			output:        20,
			cacheCreation: 5,
			want:          35,
		},
		{
			name:   "negative sum clamped to zero",
			input:  -10,
			output: -5,
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ArenaBillableTokens(tt.input, tt.output, tt.cacheCreation)
			if got != tt.want {
				t.Fatalf("ArenaBillableTokens() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestArenaBillableTokensFromUsageLog(t *testing.T) {
	log := UsageLog{
		InputTokens:         100,
		OutputTokens:        200,
		CacheCreationTokens: 30,
		CacheReadTokens:     999,
	}
	if got := ArenaBillableTokensFromUsageLog(log); got != 330 {
		t.Fatalf("ArenaBillableTokensFromUsageLog() = %d, want 330", got)
	}
}
