package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type excelBPSRecoveryTestRepository struct {
	AccountRepository
	claimed  bool
	restored *Account
}

func (r *excelBPSRecoveryTestRepository) ClaimExcelBPS403Probe(context.Context, *Account, time.Time) (bool, error) {
	return r.claimed, nil
}

func (r *excelBPSRecoveryTestRepository) RestoreExcelBPSAfter403(_ context.Context, account *Account) (bool, error) {
	r.restored = account
	return true, nil
}

type excelBPSRecoveryTestUpstream struct {
	HTTPUpstream
	send func(*http.Request) (*http.Response, error)
}

func (u *excelBPSRecoveryTestUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	return u.send(req)
}

func TestExcelBPS403RecoveryRestoresOnlyAfterClaimAndSuccessfulProbe(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		claim, correct, restored bool
		calls                    int
	}{
		{"healthy", true, true, true, 1},
		{"wrong response", true, false, false, 1},
		{"claimed by another worker", false, true, false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now().UTC()
			account := excelAccount()
			account.Status, account.Schedulable = StatusActive, true
			account.Extra = map[string]any{
				"openai_excel_bps": false, "openai_excel_bps_auto_disable_on_403": true,
				"openai_excel_bps_models": []string{"gpt-5"}, ExcelBPSAutoRecoverOn403Key: true,
				ExcelBPS403DisabledAtKey: now.Add(-2 * time.Hour).Format(time.RFC3339Nano),
			}
			calls := 0
			upstream := &excelBPSRecoveryTestUpstream{send: func(req *http.Request) (*http.Response, error) {
				calls++
				require.Equal(t, "Bearer excel-only-access", req.Header.Get("Authorization"))
				require.Equal(t, "/basispoints/api/responses", req.URL.Path)
				raw, err := io.ReadAll(req.Body)
				require.NoError(t, err)
				match := regexp.MustCompile(`Reply with exactly this nonce and nothing else: ([a-f0-9-]{36})`).FindSubmatch(raw)
				require.Len(t, match, 2)
				text := string(match[1])
				if !tc.correct {
					text = "wrong"
				}
				wire := fmt.Sprintf("data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":%q}]}]}}\n\n", text)
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(wire))}, nil
			}}
			svc := newExcelBPSRuntime(t, account, `{"allowed":true}`, true, upstream)
			repo := &excelBPSRecoveryTestRepository{claimed: tc.claim}
			svc.accountRepo = repo
			svc.recoverExcelBPS403Account(t.Context(), account, now)
			require.Equal(t, tc.calls, calls)
			require.Equal(t, tc.restored, repo.restored != nil)
			require.False(t, account.IsExcelBPSEnabled())
			require.NotContains(t, account.Extra, ExcelBPS403LastProbeAtKey)
			if repo.restored != nil {
				require.Equal(t, now.Format(time.RFC3339Nano), repo.restored.Extra[ExcelBPS403LastProbeAtKey])
			}
		})
	}
}
