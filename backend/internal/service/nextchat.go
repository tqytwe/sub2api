package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	NextChatManagedAPIKeyNamePrefix = "[managed:nextchat]"
	NextChatManagedAPIKeyName       = NextChatManagedAPIKeyNamePrefix + " AI 创作"
)

type NextChatManagedSession struct {
	UserID int64  `json:"user_id"`
	APIKey string `json:"api_key"`
	KeyID  int64  `json:"key_id"`
}

func IsNextChatManagedAPIKeyName(name string) bool {
	return strings.HasPrefix(strings.TrimSpace(name), NextChatManagedAPIKeyNamePrefix)
}

func (s *SettingService) IsNextChatEnabled(ctx context.Context) bool {
	if s == nil || s.settingRepo == nil {
		return false
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeyNextChatEnabled)
	return err == nil && strings.EqualFold(strings.TrimSpace(value), "true")
}

func (s *APIKeyService) IssueNextChatManagedSession(ctx context.Context, userID int64) (*NextChatManagedSession, error) {
	key, err := s.findReusableNextChatManagedKey(ctx, userID)
	if err != nil {
		return nil, err
	}
	if key == nil {
		groupID, groupErr := s.pickNextChatGroupID(ctx, userID)
		if groupErr != nil {
			return nil, groupErr
		}
		key, err = s.Create(ctx, userID, CreateAPIKeyRequest{
			Name:    NextChatManagedAPIKeyName,
			GroupID: groupID,
		})
		if err != nil {
			return nil, err
		}
	}
	return &NextChatManagedSession{
		UserID: userID,
		APIKey: key.Key,
		KeyID:  key.ID,
	}, nil
}

func (s *APIKeyService) findReusableNextChatManagedKey(ctx context.Context, userID int64) (*APIKey, error) {
	if s == nil || s.apiKeyRepo == nil {
		return nil, fmt.Errorf("api key service is not configured")
	}
	if lister, ok := s.apiKeyRepo.(apiKeyAllByUserIDLister); ok {
		keys, err := lister.ListAllByUserID(ctx, userID, APIKeyListFilters{Status: StatusActive})
		if err != nil {
			return nil, fmt.Errorf("list managed api keys: %w", err)
		}
		for i := range keys {
			if IsNextChatManagedAPIKeyName(keys[i].Name) && keys[i].IsActive() && !keys[i].IsExpired() {
				return &keys[i], nil
			}
		}
		return nil, nil
	}

	keys, _, err := s.List(ctx, userID, pagination.PaginationParams{Page: 1, PageSize: 100}, APIKeyListFilters{
		Search: NextChatManagedAPIKeyNamePrefix,
		Status: StatusActive,
	})
	if err != nil {
		return nil, fmt.Errorf("search managed api keys: %w", err)
	}
	for i := range keys {
		if IsNextChatManagedAPIKeyName(keys[i].Name) && keys[i].IsActive() && !keys[i].IsExpired() {
			return &keys[i], nil
		}
	}
	return nil, nil
}

func (s *APIKeyService) pickNextChatGroupID(ctx context.Context, userID int64) (*int64, error) {
	groups, err := s.GetAvailableGroups(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(groups) == 0 {
		return nil, nil
	}
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].SortOrder != groups[j].SortOrder {
			return groups[i].SortOrder < groups[j].SortOrder
		}
		return groups[i].ID < groups[j].ID
	})
	for i := range groups {
		switch strings.ToLower(strings.TrimSpace(groups[i].Platform)) {
		case PlatformOpenAI, PlatformGrok:
			id := groups[i].ID
			return &id, nil
		}
	}
	id := groups[0].ID
	return &id, nil
}
