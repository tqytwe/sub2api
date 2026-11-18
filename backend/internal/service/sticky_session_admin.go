package service

import (
	"context"
	"errors"
)

type StickySessionAdminCache interface {
	CountStickySessionBindings(context.Context, int64) (map[int64]int, error)
	ClearStickySessionBindings(context.Context, int64, map[int64]struct{}) (int, error)
}

func (s *OpenAIGatewayService) StickySessionBindingCounts(ctx context.Context, groupID int64) (map[int64]int, error) {
	if s == nil || s.cache == nil {
		return nil, ErrStickySessionAdminUnsupported
	}
	cache, ok := s.cache.(StickySessionAdminCache)
	if !ok {
		return nil, ErrStickySessionAdminUnsupported
	}
	return cache.CountStickySessionBindings(ctx, groupID)
}

func (s *OpenAIGatewayService) ClearStickySessionBindings(ctx context.Context, groupID int64, accountIDs []int64) (int, error) {
	if s == nil || s.cache == nil {
		return 0, ErrStickySessionAdminUnsupported
	}
	cache, ok := s.cache.(StickySessionAdminCache)
	if !ok {
		return 0, ErrStickySessionAdminUnsupported
	}
	var filter map[int64]struct{}
	if len(accountIDs) > 0 {
		filter = make(map[int64]struct{}, len(accountIDs))
		for _, id := range accountIDs {
			if id > 0 {
				filter[id] = struct{}{}
			}
		}
		if len(filter) == 0 {
			return 0, nil
		}
	}
	return cache.ClearStickySessionBindings(ctx, groupID, filter)
}

var ErrStickySessionAdminUnsupported = errors.New("sticky session administration unsupported")
