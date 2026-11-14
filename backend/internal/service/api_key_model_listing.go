package service

import (
	"context"
	"errors"
	"sort"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// ModelListingGroups checks read entitlements without selecting or probing a
// billing group. Revoked, inactive and deleted bindings cannot enter the union.
func (s *APIKeyService) ModelListingGroups(ctx context.Context, key *APIKey) ([]*APIKey, error) {
	unavailable := infraerrors.ServiceUnavailable("MODEL_LISTING_UNAVAILABLE", "Model listing authorization is unavailable")
	if key == nil || key.User == nil || s.groupRepo == nil {
		return nil, unavailable
	}
	var groups []*APIKey
	for _, id := range key.ConfiguredGroupIDs() {
		group, err := s.groupRepo.GetByIDLite(ctx, id)
		if errors.Is(err, ErrGroupNotFound) {
			continue
		}
		if err != nil {
			return nil, unavailable
		}
		if group == nil || !group.IsActive() {
			continue
		}
		if group.IsSubscriptionType() {
			if s.userSubRepo == nil {
				return nil, unavailable
			}
			subscription, err := s.userSubRepo.GetActiveByUserIDAndGroupID(ctx, key.UserID, id)
			if errors.Is(err, ErrSubscriptionNotFound) || errors.Is(err, ErrSubscriptionExpired) {
				continue
			}
			if err != nil {
				return nil, unavailable
			}
			if subscription == nil || !subscription.IsActive() {
				continue
			}
		} else if !key.User.CanBindGroup(id, group.IsExclusive) {
			continue
		}
		candidate, err := s.apiKeyWithGroup(ctx, key, group)
		if err != nil {
			return nil, unavailable
		}
		groups = append(groups, candidate)
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Group.ID < groups[j].Group.ID })
	if len(groups) == 0 {
		return nil, infraerrors.Forbidden("NO_ELIGIBLE_API_KEY_GROUP", "No selected group permits model listing")
	}
	return groups, nil
}
