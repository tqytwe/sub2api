package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

const mobileVideoExecutionKeyNamePrefix = NextChatManagedAPIKeyNamePrefix + " Video Execution/"

// IsMobileVideoExecutionAPIKeyName distinguishes worker-owned video keys from
// the interactive chat/image/video workspace keys. The worker never falls back
// to a chat key when this predicate fails.
func IsMobileVideoExecutionAPIKeyName(name string) bool {
	return strings.HasPrefix(strings.TrimSpace(name), mobileVideoExecutionKeyNamePrefix)
}

func mobileVideoExecutionKeyName(groupID int64) string {
	return mobileVideoExecutionKeyNamePrefix + strconv.FormatInt(groupID, 10)
}

// IssueMobileVideoExecutionSession returns a group-pinned managed key for a
// durable video job. One key per user/group prevents a job in group A from
// racing with a group switch for a job in group B, and keeps video execution
// isolated from chat and image managed sessions.
func (s *APIKeyService) IssueMobileVideoExecutionSession(ctx context.Context, userID, groupID int64) (*NextChatManagedSession, error) {
	if s == nil || s.apiKeyRepo == nil || userID <= 0 || groupID <= 0 {
		return nil, ErrInsufficientPerms
	}
	var groups []Group
	var err error
	if s.userRepo != nil && s.groupRepo != nil && s.userSubRepo != nil {
		// This is the authoritative authorization query in a fully configured
		// server.  Unlike the workspace helper it does not retain an old key
		// group when the entitlement lookup is temporarily unavailable.
		groups, err = s.GetAvailableGroups(ctx, userID)
	} else {
		// Keep the deliberately small test/embedded configuration usable while
		// still requiring an active, user-owned selectable group.
		groups, err = s.GetNextChatSelectableGroups(ctx, userID)
	}
	if err != nil {
		return nil, err
	}
	allowed := false
	for _, group := range groups {
		if group.ID == groupID {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, ErrGroupNotAllowed
	}

	name := mobileVideoExecutionKeyName(groupID)
	if lister, ok := s.apiKeyRepo.(apiKeyAllByUserIDLister); ok {
		keys, listErr := lister.ListAllByUserID(ctx, userID, APIKeyListFilters{Status: StatusActive, GroupID: &groupID})
		if listErr != nil {
			return nil, fmt.Errorf("list mobile video execution keys: %w", listErr)
		}
		for i := range keys {
			key := &keys[i]
			if key.Name == name && key.IsActive() && !key.IsExpired() && key.GroupID != nil && *key.GroupID == groupID {
				return &NextChatManagedSession{UserID: userID, APIKey: key.Key, KeyID: key.ID, Purpose: NextChatSessionPurposeVideo}, nil
			}
		}
	}

	key, err := s.Create(ctx, userID, CreateAPIKeyRequest{Name: name, GroupID: &groupID})
	if err != nil {
		return nil, fmt.Errorf("create mobile video execution key: %w", err)
	}
	return &NextChatManagedSession{UserID: userID, APIKey: key.Key, KeyID: key.ID, Purpose: NextChatSessionPurposeVideo}, nil
}

// GetMobileVideoExecutionKey resolves only a key explicitly created for a
// video worker and verifies its immutable user/group binding before a gateway
// handler sees it.
func (s *APIKeyService) GetMobileVideoExecutionKey(ctx context.Context, userID, groupID, keyID int64) (*APIKey, error) {
	if s == nil || userID <= 0 || groupID <= 0 || keyID <= 0 {
		return nil, ErrInsufficientPerms
	}
	// A durable job can outlive a user's subscription or group grant. In a fully
	// configured server, use the strict entitlement query and fail closed when it
	// cannot be read. GetNextChatSelectableGroups intentionally preserves an
	// already-bound key group during a transient lookup failure for interactive
	// workspace UX; a background billing worker must never inherit that fallback.
	var groups []Group
	var err error
	if s.userRepo != nil && s.groupRepo != nil && s.userSubRepo != nil {
		groups, err = s.GetAvailableGroups(ctx, userID)
	} else {
		groups, err = s.GetNextChatSelectableGroups(ctx, userID)
	}
	if err != nil {
		return nil, err
	}
	allowed := false
	for _, group := range groups {
		if group.ID == groupID {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, ErrGroupNotAllowed
	}
	key, err := s.GetByID(ctx, keyID)
	if err != nil {
		return nil, err
	}
	if key == nil || key.UserID != userID || key.GroupID == nil || *key.GroupID != groupID || !key.IsActive() || key.IsExpired() || !IsMobileVideoExecutionAPIKeyName(key.Name) {
		return nil, ErrInsufficientPerms
	}
	return key, nil
}
