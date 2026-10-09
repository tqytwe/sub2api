package dto

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUsageOutputTPSContract(t *testing.T) {
	intp := func(v int) *int { return &v }
	strp := func(v string) *string { return &v }
	for _, tc := range []struct {
		name string
		log  service.UsageLog
		want any
	}{
		{"stream includes first token wait", service.UsageLog{OutputTokens: 200, DurationMs: intp(10000), FirstTokenMs: intp(9000), Stream: true}, float64(20)},
		{"nonstream same duration", service.UsageLog{OutputTokens: 200, DurationMs: intp(10000)}, float64(20)},
		{"buffered first token at completion", service.UsageLog{OutputTokens: 200, DurationMs: intp(10000), FirstTokenMs: intp(10000), Stream: true}, float64(20)},
		{"first token later than duration", service.UsageLog{OutputTokens: 200, DurationMs: intp(10000), FirstTokenMs: intp(11000), Stream: true}, float64(20)},
		{"historical missing duration", service.UsageLog{OutputTokens: 200}, nil},
		{"zero duration", service.UsageLog{OutputTokens: 200, DurationMs: intp(0)}, nil},
		{"negative duration", service.UsageLog{OutputTokens: 200, DurationMs: intp(-1)}, nil},
		{"no output", service.UsageLog{DurationMs: intp(10000)}, nil},
		{"negative output", service.UsageLog{OutputTokens: -1, DurationMs: intp(10000)}, nil},
		{"single token insufficient sample", service.UsageLog{OutputTokens: 1, DurationMs: intp(10000)}, nil},
		{"image count", service.UsageLog{OutputTokens: 4000, DurationMs: intp(10000), ImageCount: 1}, nil},
		{"image output", service.UsageLog{OutputTokens: 4000, DurationMs: intp(10000), ImageOutputTokens: 4000}, nil},
		{"image mode", service.UsageLog{OutputTokens: 4000, DurationMs: intp(10000), BillingMode: strp("image")}, nil},
		{"video mode", service.UsageLog{OutputTokens: 200, DurationMs: intp(10000), BillingMode: strp("video")}, nil},
		{"video count", service.UsageLog{OutputTokens: 200, DurationMs: intp(10000), VideoCount: 1}, nil},
		{"media image", service.UsageLog{OutputTokens: 4000, DurationMs: intp(10000), MediaType: strp("image")}, nil},
		{"media video", service.UsageLog{OutputTokens: 200, DurationMs: intp(10000), MediaType: strp("video")}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.log.ActualCost = 1.25
			for _, mapped := range []any{UsageLogFromService(&tc.log), UsageLogFromServiceAdmin(&tc.log)} {
				body, err := json.Marshal(mapped)
				require.NoError(t, err)
				var fields map[string]any
				require.NoError(t, json.Unmarshal(body, &fields))
				require.Contains(t, fields, "output_tps", "missing values must be explicit JSON null")
				require.Equal(t, tc.want, fields["output_tps"])
				require.Equal(t, 1.25, fields["actual_cost"], "TPS must not change billing")
			}
		})
	}
}
