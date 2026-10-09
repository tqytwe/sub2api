package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGatewayBillingIdentitySurvivesDetachedUsageCallbacks(t *testing.T) {
	parent := context.WithValue(context.Background(), ctxkey.UsageBillingRequestID, "server-request")
	parent, cancel := context.WithCancel(parent)
	cancel()
	for _, submit := range []func(context.Context, service.UsageRecordTask){
		(&GatewayHandler{}).submitUsageRecordTask,
		(&OpenAIGatewayHandler{}).submitUsageRecordTask,
	} {
		for range 2 {
			submit(parent, func(ctx context.Context) {
				require.NoError(t, ctx.Err())
				require.Equal(t, "server-request", ctx.Value(ctxkey.UsageBillingRequestID))
			})
		}
	}
}

func TestAsyncImageWorkerClearsInheritedGatewayBillingIdentity(t *testing.T) {
	store := &asyncImageMemoryStore{tasks: map[string]*service.ImageTaskRecord{}}
	tasks, _ := newAsyncImageQueuedTestService(store)
	h := &AsyncImageHandler{tasks: tasks}
	parent := context.WithValue(context.Background(), ctxkey.UsageBillingRequestID, "submission-ingress")
	key := &service.APIKey{ID: 9, UserID: 7, User: &service.User{ID: 7}}
	for _, taskID := range []string{"task-a", "task-b", "task-a"} {
		c, _, cancel, err := h.newWorkerImageContext(parent, taskID, key,
			&service.ImageTaskRequestEnvelope{Method: http.MethodPost, Path: "/v1/images/generations"}, "application/json", []byte(`{}`))
		require.NoError(t, err)
		require.Empty(t, c.Request.Context().Value(ctxkey.UsageBillingRequestID))
		require.Equal(t, taskID, c.Request.Context().Value(ctxkey.ClientRequestID))
		cancel()
	}
	require.Equal(t, "submission-ingress", parent.Value(ctxkey.UsageBillingRequestID))
}

func TestImageStudioItemsClearInheritedGatewayBillingIdentity(t *testing.T) {
	parent := context.WithValue(context.Background(), ctxkey.UsageBillingRequestID, "shared-job-ingress")
	for _, itemID := range []string{"item-a", "item-b", "item-a"} {
		expected := "image-studio:job-1:" + itemID
		ctx := imageWorkerBillingContext(parent, expected)
		require.Empty(t, ctx.Value(ctxkey.UsageBillingRequestID))
		require.Equal(t, expected, ctx.Value(ctxkey.ClientRequestID))
		// Clearing is explicit even if the worker pool base carries an ingress ID.
		detached := usageRecordContext(ctx, parent)
		require.Empty(t, detached.Value(ctxkey.UsageBillingRequestID))
		require.Equal(t, expected, detached.Value(ctxkey.ClientRequestID))
	}
	require.Equal(t, "shared-job-ingress", parent.Value(ctxkey.UsageBillingRequestID))
}
