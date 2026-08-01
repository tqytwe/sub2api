package service

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
)

func TestPlayRequestCacheSharesConcurrentMembershipLookup(t *testing.T) {
	ctx := withPlayRequestCache(context.Background())
	cache := playRequestCacheFromContext(ctx)
	if cache == nil {
		t.Fatal("expected a request cache")
	}

	var calls atomic.Int32
	const workers = 12
	values := make(chan float64, workers)
	errs := make(chan error, workers)
	var group sync.WaitGroup
	for i := 0; i < workers; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			value, err := cache.getMembershipTotal(ctx, 42, func(context.Context, int64) (float64, error) {
				calls.Add(1)
				return 128.5, nil
			})
			values <- value
			errs <- err
		}()
	}
	group.Wait()
	close(values)
	close(errs)

	if got := calls.Load(); got != 1 {
		t.Fatalf("membership loader calls = %d, want 1", got)
	}
	for err := range errs {
		if err != nil {
			t.Fatalf("unexpected cache error: %v", err)
		}
	}
	for value := range values {
		if value != 128.5 {
			t.Fatalf("membership total = %v, want 128.5", value)
		}
	}
}

func TestPlayRequestCacheCampaignResultsAreCopied(t *testing.T) {
	ctx := withPlayRequestCache(context.Background())
	cache := playRequestCacheFromContext(ctx)
	var calls atomic.Int32
	first, err := cache.getCampaigns(ctx, 9, func(context.Context, int64) ([]PlayCampaign, error) {
		calls.Add(1)
		return []PlayCampaign{{ID: 1, Name: "August"}}, nil
	})
	if err != nil {
		t.Fatalf("first campaigns: %v", err)
	}
	first[0].Name = "mutated"
	second, err := cache.getCampaigns(ctx, 9, func(context.Context, int64) ([]PlayCampaign, error) {
		calls.Add(1)
		return nil, nil
	})
	if err != nil {
		t.Fatalf("second campaigns: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("campaign loader calls = %d, want 1", calls.Load())
	}
	if len(second) != 1 || second[0].Name != "August" {
		t.Fatalf("cached campaigns = %#v, want independent copy", second)
	}
}
