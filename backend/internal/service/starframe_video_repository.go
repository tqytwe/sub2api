package service

import "context"

// StarframeVideoRepository is the permanent submission ledger. Claims must not
// expire or be reclaimed: a missing upstream result does not prove rejection.
type StarframeVideoRepository interface {
	Claim(context.Context, *StarframeVideoTask) (bool, error)
	Complete(context.Context, *StarframeVideoTask) error
	Get(context.Context, string, StarframeVideoOwner) (*StarframeVideoTask, error)
}
