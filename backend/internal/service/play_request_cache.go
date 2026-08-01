package service

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"golang.org/x/sync/singleflight"
)

// playRequestCache is deliberately scoped to one aggregated Play request. It
// avoids repeatedly resolving the same user, paid total, campaigns, and boost
// while keeping all non-Hub paths fully live and independent.
type playRequestCache struct {
	group singleflight.Group

	mu               sync.RWMutex
	users            map[int64]*User
	membershipTotals map[int64]float64
	campaigns        map[int64][]PlayCampaign
	boosts           map[int64]PlayRechargeBoostStatus
}

type playRequestCacheKey struct{}

func withPlayRequestCache(ctx context.Context) context.Context {
	if playRequestCacheFromContext(ctx) != nil {
		return ctx
	}
	return context.WithValue(ctx, playRequestCacheKey{}, &playRequestCache{
		users:            make(map[int64]*User),
		membershipTotals: make(map[int64]float64),
		campaigns:        make(map[int64][]PlayCampaign),
		boosts:           make(map[int64]PlayRechargeBoostStatus),
	})
}

func playRequestCacheFromContext(ctx context.Context) *playRequestCache {
	if ctx == nil {
		return nil
	}
	cache, _ := ctx.Value(playRequestCacheKey{}).(*playRequestCache)
	return cache
}

func (c *playRequestCache) getUser(ctx context.Context, userID int64, load func(context.Context, int64) (*User, error)) (*User, error) {
	c.mu.RLock()
	user, ok := c.users[userID]
	c.mu.RUnlock()
	if ok {
		return user, nil
	}
	value, err, _ := c.group.Do("play-user:"+formatPlayCacheID(userID), func() (any, error) {
		c.mu.RLock()
		cached, found := c.users[userID]
		c.mu.RUnlock()
		if found {
			return cached, nil
		}
		loaded, loadErr := load(ctx, userID)
		if loadErr != nil {
			return nil, loadErr
		}
		c.mu.Lock()
		c.users[userID] = loaded
		c.mu.Unlock()
		return loaded, nil
	})
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, nil
	}
	cachedUser, typeOK := value.(*User)
	if !typeOK {
		return nil, fmt.Errorf("play user cache returned %T", value)
	}
	return cachedUser, nil
}

func (c *playRequestCache) getMembershipTotal(ctx context.Context, userID int64, load func(context.Context, int64) (float64, error)) (float64, error) {
	c.mu.RLock()
	total, ok := c.membershipTotals[userID]
	c.mu.RUnlock()
	if ok {
		return total, nil
	}
	value, err, _ := c.group.Do("play-membership:"+formatPlayCacheID(userID), func() (any, error) {
		c.mu.RLock()
		cached, found := c.membershipTotals[userID]
		c.mu.RUnlock()
		if found {
			return cached, nil
		}
		loaded, loadErr := load(ctx, userID)
		if loadErr != nil {
			return nil, loadErr
		}
		c.mu.Lock()
		c.membershipTotals[userID] = loaded
		c.mu.Unlock()
		return loaded, nil
	})
	if err != nil {
		return 0, err
	}
	cachedTotal, typeOK := value.(float64)
	if !typeOK {
		return 0, fmt.Errorf("play membership cache returned %T", value)
	}
	return cachedTotal, nil
}

func (c *playRequestCache) getCampaigns(ctx context.Context, userID int64, load func(context.Context, int64) ([]PlayCampaign, error)) ([]PlayCampaign, error) {
	c.mu.RLock()
	cached, ok := c.campaigns[userID]
	c.mu.RUnlock()
	if ok {
		return clonePlayCampaigns(cached), nil
	}
	value, err, _ := c.group.Do("play-campaigns:"+formatPlayCacheID(userID), func() (any, error) {
		c.mu.RLock()
		cachedCampaigns, found := c.campaigns[userID]
		c.mu.RUnlock()
		if found {
			return cachedCampaigns, nil
		}
		loaded, loadErr := load(ctx, userID)
		if loadErr != nil {
			return nil, loadErr
		}
		stored := clonePlayCampaigns(loaded)
		c.mu.Lock()
		c.campaigns[userID] = stored
		c.mu.Unlock()
		return stored, nil
	})
	if err != nil {
		return nil, err
	}
	cachedCampaigns, typeOK := value.([]PlayCampaign)
	if !typeOK {
		return nil, fmt.Errorf("play campaign cache returned %T", value)
	}
	return clonePlayCampaigns(cachedCampaigns), nil
}

func (c *playRequestCache) getRechargeBoost(ctx context.Context, userID int64, load func(context.Context, int64) (PlayRechargeBoostStatus, error)) (PlayRechargeBoostStatus, error) {
	c.mu.RLock()
	boost, ok := c.boosts[userID]
	c.mu.RUnlock()
	if ok {
		return boost, nil
	}
	value, err, _ := c.group.Do("play-recharge-boost:"+formatPlayCacheID(userID), func() (any, error) {
		c.mu.RLock()
		cached, found := c.boosts[userID]
		c.mu.RUnlock()
		if found {
			return cached, nil
		}
		loaded, loadErr := load(ctx, userID)
		if loadErr != nil {
			return nil, loadErr
		}
		c.mu.Lock()
		c.boosts[userID] = loaded
		c.mu.Unlock()
		return loaded, nil
	})
	if err != nil {
		return PlayRechargeBoostStatus{}, err
	}
	cachedBoost, typeOK := value.(PlayRechargeBoostStatus)
	if !typeOK {
		return PlayRechargeBoostStatus{}, fmt.Errorf("play recharge boost cache returned %T", value)
	}
	return cachedBoost, nil
}

func clonePlayCampaigns(in []PlayCampaign) []PlayCampaign {
	if len(in) == 0 {
		return nil
	}
	return append([]PlayCampaign(nil), in...)
}

func formatPlayCacheID(id int64) string {
	// This only formats an internal singleflight key; it never leaves the process.
	return strconv.FormatInt(id, 10)
}
