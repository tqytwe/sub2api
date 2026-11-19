//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service/basispoints"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type imagePolicyMemoryCache struct {
	GatewayCache
	mu       sync.Mutex
	progress map[string]string
	warning  map[string]bool
	err      error
}

func newImagePolicyMemoryCache() *imagePolicyMemoryCache {
	return &imagePolicyMemoryCache{progress: map[string]string{}, warning: map[string]bool{}}
}
func (c *imagePolicyMemoryCache) SetSessionAccountID(context.Context, int64, string, int64, time.Duration) error {
	return nil
}

func (c *imagePolicyMemoryCache) LoadBPSImageProgress(_ context.Context, k string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.progress[k], c.err
}
func (c *imagePolicyMemoryCache) SaveBPSImageProgress(_ context.Context, k, old, next string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.progress[k] != old {
		return errors.New("image history changed concurrently")
	}
	c.progress[k] = next
	return c.err
}
func (c *imagePolicyMemoryCache) ClaimBPSImageWarning(_ context.Context, k string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	first := !c.warning[k]
	c.warning[k] = true
	return first, c.err
}
func (c *imagePolicyMemoryCache) ResetBPSImageWarning(_ context.Context, k string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.warning, k)
	return c.err
}
func imagePolicyRequest(t *testing.T, h, k int) []byte {
	t.Helper()
	msg := func(n int) map[string]any {
		parts := []any{}
		for i := 0; i < n; i++ {
			parts = append(parts, map[string]any{"type": "input_image", "image_url": "data:image/png;base64,synthetic"})
		}
		return map[string]any{"role": "user", "content": parts}
	}
	b, e := json.Marshal(map[string]any{"model": "test-model", "input": []any{msg(h), map[string]any{"role": "assistant", "content": "seen"}, msg(k)}})
	require.NoError(t, e)
	return b
}
func imagePolicyContext(path string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, path, nil)
	c.Request.Header.Set("User-Agent", "codex_cli_rs/0.116.0")
	return c
}
func imagePolicySettings(policy string) ExcelBPSImageRelaySettings {
	return ExcelBPSImageRelaySettings{Enabled: true, Policy: policy, WarningRemaining: 8, CompactReserve: 3, Limits: basispoints.DefaultImageRelayLimits()}
}
func TestExcelBPSImagePolicyWarningBoundaries(t *testing.T) {
	ctx := context.Background()
	cache := newImagePolicyMemoryCache()
	s := &OpenAIGatewayService{cache: cache}
	settings := imagePolicySettings("warn")
	for _, n := range []int{11, 12, 17, 18, 20, 21} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			scope := fmt.Sprint(n)
			_, e := s.prepareExcelImagePolicy(ctx, imagePolicyContext("/v1/responses"), imagePolicyRequest(t, 0, n), settings, scope, true)
			if n < 12 {
				require.NoError(t, e)
			} else {
				require.Error(t, e)
				if n == 12 {
					require.Contains(t, e.Error(), "5 张")
					require.NotContains(t, e.Error(), "预留")
				}
			}
			_, e = s.prepareExcelImagePolicy(ctx, imagePolicyContext("/v1/responses"), imagePolicyRequest(t, 0, n), settings, scope, true)
			if n <= 17 {
				require.NoError(t, e)
			} else {
				require.Error(t, e)
			}
			compact, e := s.prepareExcelImagePolicy(ctx, imagePolicyContext("/v1/responses/compact"), imagePolicyRequest(t, 0, n), settings, scope, true)
			require.NoError(t, e)
			require.Equal(t, 20, compact.maxImages)
			compact.finish(ctx, true)
			if n == 12 {
				_, e = s.prepareExcelImagePolicy(ctx, imagePolicyContext("/v1/responses"), imagePolicyRequest(t, 0, n), settings, scope, true)
				require.Error(t, e)
			}
		})
	}
}
func TestExcelBPSImagePolicyAutomaticPartitions(t *testing.T) {
	s := &OpenAIGatewayService{cache: newImagePolicyMemoryCache()}
	settings := imagePolicySettings("auto_compact")
	for _, tt := range []struct {
		h, k, split int
		code        string
	}{{19, 1, 0, ""}, {19, 2, 2, ""}, {0, 21, 0, "basispoints_image_batch_too_large"}, {21, 1, 0, "basispoints_image_history_limit"}} {
		t.Run(fmt.Sprintf("%d/%d", tt.h, tt.k), func(t *testing.T) {
			p, e := s.prepareExcelImagePolicy(context.Background(), imagePolicyContext("/v1/responses"), imagePolicyRequest(t, tt.h, tt.k), settings, "scope", true)
			if tt.code != "" {
				var pe *excelImagePolicyError
				require.ErrorAs(t, e, &pe)
				require.Equal(t, tt.code, pe.Code)
			} else {
				require.NoError(t, e)
				require.Equal(t, tt.split, p.split)
			}
		})
	}
}
func TestExcelBPSImagePolicyStateFailureAndIsolation(t *testing.T) {
	cache := newImagePolicyMemoryCache()
	s := &OpenAIGatewayService{cache: cache}
	settings := imagePolicySettings("warn")
	ctx := context.Background()
	body := imagePolicyRequest(t, 0, 12)
	for _, scope := range []string{"account1/key1/thread1", "account1/key2/thread1", "account2/key1/thread1", "account1/key1/thread2"} {
		_, e := s.prepareExcelImagePolicy(ctx, imagePolicyContext("/v1/responses"), body, settings, scope, true)
		require.Error(t, e)
	}
	cache.err = errors.New("offline")
	_, e := s.prepareExcelImagePolicy(ctx, imagePolicyContext("/v1/responses"), body, settings, "scope", true)
	var pe *excelImagePolicyError
	require.ErrorAs(t, e, &pe)
	require.Equal(t, 503, pe.Status)
	settings.Policy = "off"
	_, e = s.prepareExcelImagePolicy(ctx, imagePolicyContext("/v1/responses"), body, settings, "", false)
	require.NoError(t, e)
}

func TestExcelBPSImagePolicyForwardCompactAndContinue(t *testing.T) {
	for _, reject := range []bool{false, true} {
		name := "success"
		if reject {
			name = "continuation_429"
		}
		t.Run(name, func(t *testing.T) {
			const tiny = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jv5sAAAAASUVORK5CYII="
			account := excelAccount()
			chunk := func(body string) *http.Response {
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
			}
			continuation := chunk("event: response.completed\n" + `data: {"type":"response.completed","response":{"id":"generation","status":"completed","model":"gpt-5","output":[],"usage":{"input_tokens":10,"output_tokens":2}}}` + "\n\n")
			if reject {
				continuation.StatusCode = http.StatusTooManyRequests
			}
			upstream := &excelBPSHTTPUpstreamStub{responses: []*http.Response{
				chunk(`{"openai_file_id":"file-policy"}`),
				chunk("event: response.completed\n" + `data: {"type":"response.completed","response":{"id":"compaction","model":"gpt-5","status":"completed","output":[{"type":"compaction","id":"cmp-policy","encrypted_content":"opaque-state"}],"usage":{"input_tokens":7,"output_tokens":3}}}` + "\n\n"),
				continuation,
			}}
			gateway := newExcelBPSRuntime(t, account, `{"allowed":true,"model_catalog":{"models":[{"id":"gpt-5","available":true}]}}`, true, upstream)
			cache := newImagePolicyMemoryCache()
			gateway.cache = cache
			gateway.settingService = &SettingService{settingRepo: &excelBPSImageSettingsRepo{values: map[string]string{
				SettingKeyExcelBPSImageLimitPolicy: "auto_compact", SettingKeyExcelBPSImageMaxImages: "2",
			}}}
			body := []byte(strings.ReplaceAll(string(imagePolicyRequest(t, 2, 1)), "data:image/png;base64,synthetic", tiny))
			body, err := sjson.SetBytes(body, "model", "gpt-5")
			require.NoError(t, err)
			c, rec := newExcelBPSContext()
			c.Request.Header.Set("Session-Id", t.Name())
			c.Request.Header.Set("User-Agent", "codex_cli_rs/0.116.0")
			result, err := gateway.forwardExcelBPS(c.Request.Context(), c, account, body, time.Now())
			require.Len(t, upstream.requests, 3)
			require.Contains(t, string(upstream.bodies[1]), "compaction_trigger")
			require.Contains(t, string(upstream.bodies[2]), "opaque-state")
			require.Equal(t, 1, strings.Count(string(upstream.bodies[2]), "file-policy"), "continuation retains only the new image")
			require.NotNil(t, result)
			if reject {
				require.Error(t, err)
				var failover *UpstreamFailoverError
				require.False(t, errors.As(err, &failover), "the spent compaction must not be replayed on another account")
				require.Equal(t, http.StatusTooManyRequests, rec.Code)
				require.Equal(t, 7, result.Usage.InputTokens)
				require.Equal(t, 3, result.Usage.OutputTokens)
				return
			}
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, rec.Code)
			require.Equal(t, 17, result.Usage.InputTokens)
			require.Equal(t, 5, result.Usage.OutputTokens)
			require.EqualValues(t, 17, gjson.Get(rec.Body.String(), "usage.input_tokens").Int())
		})
	}
}
