package handler

import (
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIResponsesWebSocket_SelectsGroupFromFirstFrameBeforeAllowlist(t *testing.T) {
	for _, mode := range []string{service.OpenAIWSIngressModePassthrough, service.OpenAIWSIngressModeDedicated} {
		t.Run(mode, func(t *testing.T) {
			var selections int
			got := runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
				firstPayload:  `{"type":"response.create","model":"gpt-5.4","input":"first"}`,
				secondPayload: `{"type":"response.create","input":"second"}`,
				group:         wsAllowlistGroup(true, "claude-sonnet-4-6"),
				ingressMode:   mode,
				setupContext: func(c *gin.Context) {
					middleware.SetDeferredAPIKeyRequestGroup(c, func(models []string) (*service.APIKey, error) {
						selections++
						if len(models) != 1 || models[0] != "gpt-5.4" {
							return nil, errors.New("first-frame model was not supplied")
						}
						key, _ := middleware.GetAPIKeyFromContext(c)
						selected := *key
						selected.Group = wsAllowlistGroup(true, "gpt-5.4")
						selected.Group.ID = 4202
						selected.GroupID = &selected.Group.ID
						c.Set(string(middleware.ContextKeyAPIKey), &selected)
						return &selected, nil
					})
				},
			})
			require.Equal(t, 1, selections, "subsequent turns must retain the selected billing group")
			require.Len(t, got.upstreamPayloads, 2)
			for _, log := range got.logs {
				require.NotNil(t, log.GroupID)
				require.Equal(t, int64(4202), *log.GroupID)
			}
		})
	}
}

func TestOpenAIResponsesWebSocket_GroupSelectionErrorNeverReachesUpstream(t *testing.T) {
	var models []string
	runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
		firstPayload:            `{"type":"response.create","model":"gpt-5.4","model":"blocked-model"}`,
		group:                   wsAllowlistGroup(false),
		firstFrameCloseExpected: true,
		closeReason:             "No selected group is available for this request",
		setupContext: func(c *gin.Context) {
			middleware.SetDeferredAPIKeyRequestGroup(c, func(candidates []string) (*service.APIKey, error) {
				models = append([]string(nil), candidates...)
				return nil, errors.New("no permitted group")
			})
		},
	})
	require.Equal(t, []string{"gpt-5.4", "blocked-model"}, models, "selection must inspect every model that a downstream decoder could use")
}

func TestOpenAIResponsesWebSocket_GroupSelectionSeesExplicitImageIntent(t *testing.T) {
	var imageIntent bool
	runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
		firstPayload:            `{"type":"response.create","model":"gpt-5.4","tools":[{"type":"image_generation"}]}`,
		firstFrameCloseExpected: true,
		closeReason:             "No selected group is available for this request",
		setupContext: func(c *gin.Context) {
			middleware.SetDeferredAPIKeyRequestGroup(c, func([]string) (*service.APIKey, error) {
				imageIntent = service.OpenAIImageGenerationIntentFromContext(c.Request.Context())
				return nil, errors.New("no group permits image generation")
			})
		},
	})
	require.True(t, imageIntent, "selection must apply image permission before choosing a billing group")
}
