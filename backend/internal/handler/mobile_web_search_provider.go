package handler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/websearch"
)

const (
	mobileWebSearchPrimaryTimeout  = 5 * time.Second
	mobileWebSearchCircuitCooldown = 30 * time.Second
)

// mobileWebSearchFallbackProvider keeps Exa as the primary provider while
// retaining a keyless DuckDuckGo fallback. The circuit breaker avoids making
// every user wait for a known-failing Exa provider during a short outage.
// One handler search invocation owns the complete primary/fallback sequence,
// therefore the handler reserves its user quota exactly once.
type mobileWebSearchFallbackProvider struct {
	primary        mobileWebSearchProvider
	fallback       mobileWebSearchProvider
	primaryName    string
	fallbackName   string
	primaryTimeout time.Duration
	cooldown       time.Duration
	now            func() time.Time

	mu               sync.Mutex
	primaryOpenUntil time.Time
}

func newMobileWebSearchFallbackProvider(primary, fallback mobileWebSearchProvider, primaryName, fallbackName string) *mobileWebSearchFallbackProvider {
	return &mobileWebSearchFallbackProvider{
		primary:        primary,
		fallback:       fallback,
		primaryName:    strings.TrimSpace(primaryName),
		fallbackName:   strings.TrimSpace(fallbackName),
		primaryTimeout: mobileWebSearchPrimaryTimeout,
		cooldown:       mobileWebSearchCircuitCooldown,
		now:            time.Now,
	}
}

func (p *mobileWebSearchFallbackProvider) Name() string {
	if p == nil {
		return ""
	}
	return p.primaryName
}

func (p *mobileWebSearchFallbackProvider) Search(ctx context.Context, request websearch.SearchRequest) (*websearch.SearchResponse, error) {
	if p == nil {
		return nil, fmt.Errorf("web search provider unavailable")
	}
	var primaryErr error
	if p.primary != nil && !p.primaryCircuitOpen() {
		primaryCtx, cancel := context.WithTimeout(ctx, p.primaryRequestTimeout())
		response, err := p.primary.Search(primaryCtx, request)
		cancel()
		if err == nil && response != nil {
			p.closePrimaryCircuit()
			return withMobileWebSearchProvider(response, p.primaryName), nil
		}
		if err == nil {
			err = fmt.Errorf("primary web search returned no response")
		}
		if errors.Is(err, context.Canceled) && ctx.Err() != nil {
			return nil, err
		}
		primaryErr = err
		p.openPrimaryCircuit()
	}

	if p.fallback == nil {
		if primaryErr != nil {
			return nil, primaryErr
		}
		return nil, fmt.Errorf("web search provider unavailable")
	}
	response, fallbackErr := p.fallback.Search(ctx, request)
	if fallbackErr == nil && response != nil {
		return withMobileWebSearchProvider(response, p.fallbackName), nil
	}
	if fallbackErr == nil {
		fallbackErr = fmt.Errorf("fallback web search returned no response")
	}
	if primaryErr != nil {
		return nil, fmt.Errorf("web search providers failed: %w", fallbackErr)
	}
	return nil, fallbackErr
}

func withMobileWebSearchProvider(response *websearch.SearchResponse, fallbackName string) *websearch.SearchResponse {
	if response != nil && strings.TrimSpace(response.Provider) == "" {
		response.Provider = fallbackName
	}
	return response
}

func (p *mobileWebSearchFallbackProvider) primaryRequestTimeout() time.Duration {
	if p.primaryTimeout > 0 {
		return p.primaryTimeout
	}
	return mobileWebSearchPrimaryTimeout
}

func (p *mobileWebSearchFallbackProvider) primaryCircuitOpen() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.primaryOpenUntil.After(p.now())
}

func (p *mobileWebSearchFallbackProvider) openPrimaryCircuit() {
	p.mu.Lock()
	defer p.mu.Unlock()
	cooldown := p.cooldown
	if cooldown <= 0 {
		cooldown = mobileWebSearchCircuitCooldown
	}
	p.primaryOpenUntil = p.now().Add(cooldown)
}

func (p *mobileWebSearchFallbackProvider) closePrimaryCircuit() {
	p.mu.Lock()
	p.primaryOpenUntil = time.Time{}
	p.mu.Unlock()
}
