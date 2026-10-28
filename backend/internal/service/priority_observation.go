package service

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
)

// WithPrioritySchedulingObservation records the public request identity before
// channel routing. Scheduling history can then remain scoped to the caller's
// model and endpoint while account mapping continues to use the routed model.
// The values are metadata only; they do not alter routing, billing, or model
// capability checks.
func WithPrioritySchedulingObservation(ctx context.Context, group *int64, publicModel, endpoint string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = context.WithValue(ctx, ctxkey.RequestedPublicModel, strings.TrimSpace(publicModel))
	ctx = context.WithValue(ctx, ctxkey.InboundEndpoint, strings.TrimSpace(endpoint))
	if group != nil {
		ctx = context.WithValue(ctx, priorityObservationGroupKey{}, *group)
	}
	return ctx
}

type priorityObservationGroupKey struct{}

func priorityObservationGroupFromContext(ctx context.Context) (int64, bool) {
	group, ok := ctx.Value(priorityObservationGroupKey{}).(int64)
	return group, ok
}

func priorityGroupID(c *gin.Context) *int64 {
	id := getOpenAIGroupIDFromContext(c)
	if id <= 0 {
		return nil
	}
	return &id
}
