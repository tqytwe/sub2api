//go:build unit

package handler

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type apiKeyPolicyCapturingUpstream struct {
	service.HTTPUpstream
	calls            int
	body             []byte
	original, mapped string
}

func (u *apiKeyPolicyCapturingUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.calls++
	u.body, _ = io.ReadAll(req.Body)
	u.original, _ = service.RequestedPublicModelFromContext(req.Context())
	u.mapped = service.APIKeyMappedModelFromContext(req.Context())
	return &http.Response{StatusCode: http.StatusBadRequest, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"error":{"type":"invalid_request_error","message":"synthetic stop after captured send"}}`))}, nil
}

func TestAPIKeyPolicyGatewayAppliesMappingEffortAndAdmission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name              string
		allowed           []string
		limit             int
		input             string
		wantCalls, status int
	}{
		{"mapped send retains group ceiling", []string{"public-model", "gpt-5.1"}, 0, "hello", 1, http.StatusBadRequest},
		{"mapped target denied", []string{"public-model"}, 0, "hello", 0, http.StatusNotFound},
		{"explicit ceiling admits no send", []string{"public-model", "gpt-5.1"}, 10, strings.Repeat(" context", 100), 0, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := &apiKeyPolicyCapturingUpstream{}
			h := newOpenAIResponsesFailoverTestHandler(t, upstream)
			gid := int64(3131)
			key := &service.APIKey{ID: 99, UserID: 100, GroupID: &gid, AllowLongContext: true, ContextTokenLimit: tc.limit, ModelMapping: map[string]string{"public-model": "gpt-5.1"}, ReasoningEffort: "high", User: &service.User{ID: 100}, Group: &service.Group{ID: gid, Platform: service.PlatformOpenAI, MaxReasoningEffort: "medium", ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: tc.allowed}, ReasoningEffortMappings: []service.ReasoningEffortMapping{{From: "low", To: "ultra"}}}}
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set(string(middleware2.ContextKeyAPIKey), key)
				c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 100})
				c.Next()
			})
			router.POST("/v1/responses", middleware2.GroupModelAllowlist(), middleware2.APIKeyModelPolicy(), middleware2.APIKeyLongContextPolicy(nil), h.Responses)
			rec := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewBufferString(`{"model":"public-model","stream":false,"reasoning":{"effort":"low"},"input":"`+tc.input+`"}`))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(rec, request)
			require.Equal(t, tc.status, rec.Code, rec.Body.String())
			require.Equal(t, tc.wantCalls, upstream.calls, rec.Body.String())
			if upstream.calls > 0 {
				// The existing OAuth forwarder normalizes gpt-5.1 to gpt-5.4.
				require.Equal(t, "gpt-5.4", gjson.GetBytes(upstream.body, "model").String())
				require.Equal(t, "medium", gjson.GetBytes(upstream.body, "reasoning.effort").String())
				require.Equal(t, "public-model", upstream.original)
				require.Equal(t, "gpt-5.1", upstream.mapped)
			}
		})
	}
}
