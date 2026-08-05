package handler

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSubmitUsageRecordTaskCopiesRequestContext(t *testing.T) {
	parent := context.WithValue(context.Background(), ctxkey.ClientRequestID, "client-request-123")
	parent = context.WithValue(parent, ctxkey.RequestID, "request-456")

	var gotClientRequestID string
	var gotRequestID string
	h := &GatewayHandler{}
	h.submitUsageRecordTask(parent, func(ctx context.Context) {
		gotClientRequestID, _ = ctx.Value(ctxkey.ClientRequestID).(string)
		gotRequestID, _ = ctx.Value(ctxkey.RequestID).(string)
	})

	require.Equal(t, "client-request-123", gotClientRequestID)
	require.Equal(t, "request-456", gotRequestID)
}

func TestOpenAISubmitUsageRecordTaskCopiesRequestContext(t *testing.T) {
	parent := context.WithValue(context.Background(), ctxkey.ClientRequestID, "openai-client-request-123")
	parent = context.WithValue(parent, ctxkey.RequestID, "openai-request-456")
	capture := service.NewImageStudioBillingCapture()
	parent = service.WithImageStudioBillingCapture(parent, capture)
	parent = service.WithImageStudioBillingActualCostCap(parent, 0.125)

	var gotClientRequestID string
	var gotRequestID string
	var gotManagedBilling bool
	var gotCapture *service.ImageStudioBillingCapture
	var gotActualCostCap float64
	var gotActualCostCapOK bool
	h := &OpenAIGatewayHandler{}
	h.submitUsageRecordTask(parent, func(ctx context.Context) {
		gotClientRequestID, _ = ctx.Value(ctxkey.ClientRequestID).(string)
		gotRequestID, _ = ctx.Value(ctxkey.RequestID).(string)
		gotManagedBilling, _ = ctx.Value(ctxkey.ImageStudioManagedBilling).(bool)
		gotCapture = service.ImageStudioBillingCaptureFromContext(ctx)
		gotActualCostCap, gotActualCostCapOK = ctx.Value(ctxkey.ImageStudioBillingActualCostCap).(float64)
	})

	require.Equal(t, "openai-client-request-123", gotClientRequestID)
	require.Equal(t, "openai-request-456", gotRequestID)
	require.True(t, gotManagedBilling)
	require.Same(t, capture, gotCapture)
	require.True(t, gotActualCostCapOK)
	require.InDelta(t, 0.125, gotActualCostCap, 0.000001)
}
