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

type NextChatWorkspaceUser struct {
	ID            int64   `json:"id"`
	Username      string  `json:"username,omitempty"`
	Email         string  `json:"email,omitempty"`
	AvatarURL     string  `json:"avatar_url,omitempty"`
	Balance       float64 `json:"balance"`
	FrozenBalance float64 `json:"frozen_balance"`
}

type NextChatWorkspaceAPIKey struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	GroupID       *int64 `json:"group_id,omitempty"`
	GroupName     string `json:"group_name,omitempty"`
	GroupPlatform string `json:"group_platform,omitempty"`
}

type NextChatWorkspaceIdentity struct {
	User   NextChatWorkspaceUser   `json:"user"`
	APIKey NextChatWorkspaceAPIKey `json:"managed_api_key"`
}

type NextChatPrompt struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Content     string `json:"content"`
	Category    string `json:"category,omitempty"`
}

type NextChatPromptCatalog struct {
	ChatPrompts    []NextChatPrompt   `json:"chat_prompts"`
	ImageTemplates ImageStudioCatalog `json:"image_templates"`
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
	groupID, groupErr := s.pickNextChatGroupID(ctx, userID)
	if groupErr != nil {
		return nil, groupErr
	}
	if key != nil {
		key, err = s.realignNextChatManagedKeyGroup(ctx, key, userID, groupID)
		if err != nil {
			return nil, err
		}
	}
	if key == nil {
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

func (s *APIKeyService) GetNextChatWorkspaceIdentity(ctx context.Context, userID, apiKeyID int64) (*NextChatWorkspaceIdentity, error) {
	if s == nil || s.userRepo == nil || s.apiKeyRepo == nil {
		return nil, fmt.Errorf("nextchat workspace identity service is not configured")
	}
	if userID <= 0 || apiKeyID <= 0 {
		return nil, ErrInsufficientPerms
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get nextchat user: %w", err)
	}
	key, err := s.GetByID(ctx, apiKeyID)
	if err != nil {
		return nil, fmt.Errorf("get nextchat managed api key: %w", err)
	}
	if key.UserID != userID || !IsNextChatManagedAPIKeyName(key.Name) || !key.IsActive() || key.IsExpired() {
		return nil, ErrInsufficientPerms
	}

	group := key.Group
	if group == nil && key.GroupID != nil && *key.GroupID > 0 && s.groupRepo != nil {
		if got, groupErr := s.groupRepo.GetByID(ctx, *key.GroupID); groupErr == nil {
			group = got
		}
	}

	out := &NextChatWorkspaceIdentity{
		User: NextChatWorkspaceUser{
			ID:            user.ID,
			Username:      user.Username,
			Email:         user.Email,
			AvatarURL:     user.AvatarURL,
			Balance:       user.Balance,
			FrozenBalance: user.FrozenBalance,
		},
		APIKey: NextChatWorkspaceAPIKey{
			ID:      key.ID,
			Name:    key.Name,
			GroupID: key.GroupID,
		},
	}
	if group != nil {
		out.APIKey.GroupName = group.Name
		out.APIKey.GroupPlatform = group.Platform
	}
	return out, nil
}

func BuildNextChatPromptCatalog() NextChatPromptCatalog {
	return NextChatPromptCatalog{
		ChatPrompts: []NextChatPrompt{
			{
				ID:          "general-assistant",
				Title:       "通用助手",
				Description: "日常问答、写作、分析和总结",
				Content:     "你是极速蹬 AI 工作台里的专业助手。请用清晰、准确、可执行的方式回答用户问题。",
				Category:    "chat",
			},
			{
				ID:          "ecommerce-copy",
				Title:       "电商文案",
				Description: "商品卖点、标题、详情页和投放文案",
				Content:     "请根据用户给出的商品信息，输出适合电商场景的标题、核心卖点、详情页结构和可直接使用的营销文案。",
				Category:    "ecommerce",
			},
		},
		ImageTemplates: defaultImageStudioCatalog(),
	}
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
	groups, err := s.getNextChatUserOwnedKeyGroups(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(groups) == 0 && s.userRepo != nil && s.groupRepo != nil && s.userSubRepo != nil {
		groups, err = s.GetAvailableGroups(ctx, userID)
		if err != nil {
			return nil, err
		}
	}
	return pickPreferredNextChatGroupID(groups), nil
}

func (s *APIKeyService) getNextChatUserOwnedKeyGroups(ctx context.Context, userID int64) ([]Group, error) {
	lister, ok := s.apiKeyRepo.(apiKeyAllByUserIDLister)
	if !ok {
		return nil, nil
	}
	keys, err := lister.ListAllByUserID(ctx, userID, APIKeyListFilters{Status: StatusActive})
	if err != nil {
		return nil, fmt.Errorf("list user api key groups: %w", err)
	}
	groupByID := make(map[int64]Group)
	missingGroupIDs := make(map[int64]struct{})
	for i := range keys {
		key := &keys[i]
		if IsNextChatManagedAPIKeyName(key.Name) || key.GroupID == nil || *key.GroupID <= 0 {
			continue
		}
		if key.Group != nil && key.Group.ID == *key.GroupID && key.Group.IsActive() {
			groupByID[*key.GroupID] = *key.Group
			continue
		}
		missingGroupIDs[*key.GroupID] = struct{}{}
	}
	if len(missingGroupIDs) > 0 && s.groupRepo != nil {
		groups, listErr := s.groupRepo.ListActive(ctx)
		if listErr != nil {
			return nil, fmt.Errorf("list active groups: %w", listErr)
		}
		for _, group := range groups {
			if _, needed := missingGroupIDs[group.ID]; needed {
				groupByID[group.ID] = group
			}
		}
	}
	out := make([]Group, 0, len(groupByID))
	for _, group := range groupByID {
		out = append(out, group)
	}
	return out, nil
}

func (s *APIKeyService) realignNextChatManagedKeyGroup(ctx context.Context, key *APIKey, userID int64, groupID *int64) (*APIKey, error) {
	if key == nil || groupID == nil || sameAPIKeyGroupID(key.GroupID, groupID) {
		return key, nil
	}
	if key.UserID != userID {
		return nil, ErrInsufficientPerms
	}

	updated := *key
	updated.GroupID = groupID
	if err := s.apiKeyRepo.Update(ctx, &updated); err != nil {
		return nil, fmt.Errorf("realign nextchat managed api key group: %w", err)
	}
	s.InvalidateAuthCacheByKey(ctx, updated.Key)
	s.compileAPIKeyIPRules(&updated)
	return &updated, nil
}

func pickPreferredNextChatGroupID(groups []Group) *int64 {
	if len(groups) == 0 {
		return nil
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
			return &id
		}
	}
	id := groups[0].ID
	return &id
}

func sameAPIKeyGroupID(a, b *int64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
