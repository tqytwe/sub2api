package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func TestWithPrioritySchedulingObservationKeepsPublicIdentitySeparate(t *testing.T) {
	group := int64(42)
	ctx := WithPrioritySchedulingObservation(context.Background(), &group, " public-model ", " /v1/responses ")

	model, ok := RequestedPublicModelFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, "public-model", model)
	require.Equal(t, "/v1/responses", ctx.Value(ctxkey.InboundEndpoint))
	gotGroup, ok := priorityObservationGroupFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, group, gotGroup)
}

func TestPrioritySchedulingQueryCarriesInboundEndpoint(t *testing.T) {
	query := PrioritySchedulingQuery{Model: "public-model", InboundEndpoint: "/v1/chat/completions"}
	require.Equal(t, "public-model", query.Model)
	require.Equal(t, "/v1/chat/completions", query.InboundEndpoint)
}
