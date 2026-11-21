package admin

import (
	"context"
	"errors"
	"net/http"
	"time"
)

func matureProviderRetry(ctx context.Context, request func() error) error {
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := request()
		if err == nil || !matureProviderRateLimited(err) || attempt >= 2 {
			return err
		}
		delay := 250 * time.Millisecond * time.Duration(1<<attempt)
		var remote *providerRemoteError
		if errors.As(err, &remote) && remote.retryAfter > delay {
			delay = remote.retryAfter
		}
		var api *providerAPIError
		if errors.As(err, &api) && api.retryAfter > delay {
			delay = api.retryAfter
		}
		if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= delay {
			return err
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func matureProviderRateLimited(err error) bool {
	var remote *providerRemoteError
	if errors.As(err, &remote) && remote.status == http.StatusTooManyRequests {
		return true
	}
	var api *providerAPIError
	return errors.As(err, &api) && api.status == http.StatusTooManyRequests
}
