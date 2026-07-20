package service

import (
	"context"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type nextChatAPIKeyRepoStub struct {
	APIKeyRepository
	keys    []APIKey
	created []APIKey
	updated []APIKey
}

func (s *nextChatAPIKeyRepoStub) Create(_ context.Context, key *APIKey) error {
	if key.ID == 0 {
		key.ID = int64(1000 + len(s.created))
	}
	clone := *key
	s.created = append(s.created, clone)
	s.keys = append(s.keys, clone)
	return nil
}

func (s *nextChatAPIKeyRepoStub) GetByID(_ context.Context, id int64) (*APIKey, error) {
	for i := range s.keys {
		if s.keys[i].ID == id {
			clone := s.keys[i]
			return &clone, nil
		}
	}
	return nil, ErrAPIKeyNotFound
}

func (s *nextChatAPIKeyRepoStub) Update(_ context.Context, key *APIKey) error {
	clone := *key
	s.updated = append(s.updated, clone)
	for i := range s.keys {
		if s.keys[i].ID == key.ID {
			s.keys[i] = clone
			return nil
		}
	}
	return ErrAPIKeyNotFound
}

func (s *nextChatAPIKeyRepoStub) ListByUserID(_ context.Context, userID int64, params pagination.PaginationParams, filters APIKeyListFilters) ([]APIKey, *pagination.PaginationResult, error) {
	keys := filterNextChatAPIKeyRepoKeys(userID, s.keys, filters)
	return keys, &pagination.PaginationResult{Total: int64(len(keys)), Page: params.Page, PageSize: params.PageSize, Pages: 1}, nil
}

func (s *nextChatAPIKeyRepoStub) ListAllByUserID(_ context.Context, userID int64, filters APIKeyListFilters) ([]APIKey, error) {
	return filterNextChatAPIKeyRepoKeys(userID, s.keys, filters), nil
}

type nextChatUserRepoStub struct {
	UserRepository
	user *User
}

func (s *nextChatUserRepoStub) GetByID(context.Context, int64) (*User, error) {
	return s.user, nil
}

type nextChatGroupRepoStub struct {
	GroupRepository
	groups []Group
}

func (s *nextChatGroupRepoStub) GetByID(_ context.Context, id int64) (*Group, error) {
	for i := range s.groups {
		if s.groups[i].ID == id {
			return &s.groups[i], nil
		}
	}
	return nil, ErrGroupNotFound
}

func (s *nextChatGroupRepoStub) ListActive(context.Context) ([]Group, error) {
	return append([]Group(nil), s.groups...), nil
}

type nextChatSubscriptionRepoStub struct {
	UserSubscriptionRepository
}

func (s *nextChatSubscriptionRepoStub) ListActiveByUserID(context.Context, int64) ([]UserSubscription, error) {
	return nil, nil
}

type nextChatModelCatalogRepoStub struct {
	ModelCatalogRepository
	entries []SiteModelCatalogEntry
}

func (s *nextChatModelCatalogRepoStub) ListCatalog(_ context.Context, filter CatalogListFilter) ([]SiteModelCatalogEntry, error) {
	out := make([]SiteModelCatalogEntry, 0, len(s.entries))
	for _, entry := range s.entries {
		if filter.VisibleAuth != nil && entry.VisibleAuth != *filter.VisibleAuth {
			continue
		}
		out = append(out, entry)
	}
	return out, nil
}

type nextChatAvailableModelResolverStub struct {
	modelsByGroup map[int64][]string
}

func (s *nextChatAvailableModelResolverStub) GetAvailableModels(_ context.Context, groupID *int64, _ string) []string {
	if s == nil || groupID == nil {
		return nil
	}
	return append([]string(nil), s.modelsByGroup[*groupID]...)
}

func TestIssueNextChatManagedSessionReusesExistingManagedKey(t *testing.T) {
	repo := &nextChatAPIKeyRepoStub{keys: []APIKey{
		{ID: 1, UserID: 42, Name: "normal key", Key: "sk-normal", Status: StatusActive},
		{ID: 2, UserID: 42, Name: NextChatManagedAPIKeyName, Key: "sk-managed", Status: StatusActive},
	}}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, nil, &config.Config{})

	session, err := svc.IssueNextChatManagedSession(context.Background(), 42)

	require.NoError(t, err)
	require.Equal(t, int64(42), session.UserID)
	require.Equal(t, int64(2), session.KeyID)
	require.Equal(t, "sk-managed", session.APIKey)
	require.Empty(t, repo.created)
	require.Empty(t, repo.updated)
}

func TestIssueNextChatManagedSessionCreatesHiddenKeyWithPreferredGroup(t *testing.T) {
	repo := &nextChatAPIKeyRepoStub{}
	userRepo := &nextChatUserRepoStub{user: &User{ID: 42, Status: StatusActive}}
	groupRepo := &nextChatGroupRepoStub{groups: []Group{
		{ID: 3, Platform: PlatformAnthropic, Status: StatusActive, SortOrder: 1},
		{ID: 2, Platform: PlatformOpenAI, Status: StatusActive, SortOrder: 2},
		{ID: 1, Platform: PlatformGrok, Status: StatusActive, SortOrder: 3},
	}}
	svc := NewAPIKeyService(repo, userRepo, groupRepo, &nextChatSubscriptionRepoStub{}, nil, nil, &config.Config{
		Default: config.DefaultConfig{APIKeyPrefix: "sk-test-"},
	})

	session, err := svc.IssueNextChatManagedSession(context.Background(), 42)

	require.NoError(t, err)
	require.NotEmpty(t, session.APIKey)
	require.Len(t, repo.created, 1)
	require.Equal(t, NextChatManagedAPIKeyName, repo.created[0].Name)
	require.NotNil(t, repo.created[0].GroupID)
	require.Equal(t, int64(2), *repo.created[0].GroupID)
	require.True(t, strings.HasPrefix(repo.created[0].Key, "sk-test-"))
}

func TestIssueNextChatManagedSessionCreatesHiddenKeyFromExistingUserKeyGroup(t *testing.T) {
	existingGroupID := int64(7)
	repo := &nextChatAPIKeyRepoStub{keys: []APIKey{
		{
			ID:      1,
			UserID:  42,
			Name:    "user key",
			Key:     "sk-user",
			Status:  StatusActive,
			GroupID: &existingGroupID,
			Group:   &Group{ID: existingGroupID, Platform: PlatformOpenAI, Status: StatusActive, SortOrder: 9},
		},
	}}
	userRepo := &nextChatUserRepoStub{user: &User{ID: 42, Status: StatusActive}}
	groupRepo := &nextChatGroupRepoStub{groups: []Group{
		{ID: 2, Platform: PlatformOpenAI, Status: StatusActive, SortOrder: 1},
		{ID: existingGroupID, Platform: PlatformOpenAI, Status: StatusActive, SortOrder: 9},
	}}
	svc := NewAPIKeyService(repo, userRepo, groupRepo, &nextChatSubscriptionRepoStub{}, nil, nil, &config.Config{
		Default: config.DefaultConfig{APIKeyPrefix: "sk-test-"},
	})

	session, err := svc.IssueNextChatManagedSession(context.Background(), 42)

	require.NoError(t, err)
	require.NotEmpty(t, session.APIKey)
	require.Len(t, repo.created, 1)
	require.NotNil(t, repo.created[0].GroupID)
	require.Equal(t, existingGroupID, *repo.created[0].GroupID)
}

func TestIssueNextChatManagedSessionKeepsReusableManagedKeySelectableGroup(t *testing.T) {
	currentGroupID := int64(2)
	otherGroupID := int64(7)
	repo := &nextChatAPIKeyRepoStub{keys: []APIKey{
		{
			ID:      1,
			UserID:  42,
			Name:    "openai user key",
			Key:     "sk-user-openai",
			Status:  StatusActive,
			GroupID: &otherGroupID,
			Group:   &Group{ID: otherGroupID, Platform: PlatformOpenAI, Status: StatusActive, SortOrder: 1},
		},
		{
			ID:      2,
			UserID:  42,
			Name:    "grok user key",
			Key:     "sk-user-grok",
			Status:  StatusActive,
			GroupID: &currentGroupID,
			Group:   &Group{ID: currentGroupID, Platform: PlatformGrok, Status: StatusActive, SortOrder: 2},
		},
		{
			ID:      3,
			UserID:  42,
			Name:    NextChatManagedAPIKeyName,
			Key:     "sk-managed",
			Status:  StatusActive,
			GroupID: &currentGroupID,
			Group:   &Group{ID: currentGroupID, Platform: PlatformGrok, Status: StatusActive, SortOrder: 2},
		},
	}}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, nil, &config.Config{})

	session, err := svc.IssueNextChatManagedSession(context.Background(), 42)

	require.NoError(t, err)
	require.Equal(t, int64(3), session.KeyID)
	require.Equal(t, "sk-managed", session.APIKey)
	require.Empty(t, repo.created)
	require.Empty(t, repo.updated)
}

func TestIssueNextChatManagedSessionRealignsReusableManagedKeyInvalidGroup(t *testing.T) {
	staleGroupID := int64(2)
	desiredGroupID := int64(7)
	repo := &nextChatAPIKeyRepoStub{keys: []APIKey{
		{
			ID:      1,
			UserID:  42,
			Name:    "user key",
			Key:     "sk-user",
			Status:  StatusActive,
			GroupID: &desiredGroupID,
			Group:   &Group{ID: desiredGroupID, Platform: PlatformOpenAI, Status: StatusActive, SortOrder: 9},
		},
		{
			ID:      2,
			UserID:  42,
			Name:    NextChatManagedAPIKeyName,
			Key:     "sk-managed",
			Status:  StatusActive,
			GroupID: &staleGroupID,
			Group:   &Group{ID: staleGroupID, Platform: PlatformOpenAI, Status: StatusActive, SortOrder: 1},
		},
	}}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, nil, &config.Config{})

	session, err := svc.IssueNextChatManagedSession(context.Background(), 42)

	require.NoError(t, err)
	require.Equal(t, int64(2), session.KeyID)
	require.Equal(t, "sk-managed", session.APIKey)
	require.Empty(t, repo.created)
	require.Len(t, repo.updated, 1)
	require.NotNil(t, repo.updated[0].GroupID)
	require.Equal(t, desiredGroupID, *repo.updated[0].GroupID)
}

func TestIssueNextChatManagedSessionClearsReusableManagedKeyWhenNoSelectableGroups(t *testing.T) {
	staleGroupID := int64(2)
	repo := &nextChatAPIKeyRepoStub{keys: []APIKey{
		{
			ID:      2,
			UserID:  42,
			Name:    NextChatManagedAPIKeyName,
			Key:     "sk-managed",
			Status:  StatusActive,
			GroupID: &staleGroupID,
			Group:   &Group{ID: staleGroupID, Platform: PlatformOpenAI, Status: StatusActive},
		},
	}}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, nil, &config.Config{})

	session, err := svc.IssueNextChatManagedSession(context.Background(), 42)

	require.NoError(t, err)
	require.Equal(t, int64(2), session.KeyID)
	require.Len(t, repo.updated, 1)
	require.Nil(t, repo.updated[0].GroupID)
}

func TestGetNextChatWorkspaceIdentityReturnsUserAndManagedKeySummary(t *testing.T) {
	groupID := int64(7)
	repo := &nextChatAPIKeyRepoStub{keys: []APIKey{
		{
			ID:      2,
			UserID:  42,
			Name:    NextChatManagedAPIKeyName,
			Key:     "sk-managed",
			Status:  StatusActive,
			GroupID: &groupID,
		},
	}}
	userRepo := &nextChatUserRepoStub{user: &User{
		ID:            42,
		Username:      "tester",
		Email:         "tester@example.com",
		AvatarURL:     "/avatar.png",
		Balance:       12.5,
		FrozenBalance: 1.5,
		Status:        StatusActive,
	}}
	groupRepo := &nextChatGroupRepoStub{groups: []Group{
		{ID: groupID, Name: "OpenAI main", Platform: PlatformOpenAI, Status: StatusActive},
	}}
	svc := NewAPIKeyService(repo, userRepo, groupRepo, nil, nil, nil, &config.Config{})

	identity, err := svc.GetNextChatWorkspaceIdentity(context.Background(), 42, 2)

	require.NoError(t, err)
	require.Equal(t, int64(42), identity.User.ID)
	require.Equal(t, "tester", identity.User.Username)
	require.Equal(t, 12.5, identity.User.Balance)
	require.Equal(t, int64(2), identity.APIKey.ID)
	require.Equal(t, NextChatManagedAPIKeyName, identity.APIKey.Name)
	require.NotNil(t, identity.APIKey.GroupID)
	require.Equal(t, groupID, *identity.APIKey.GroupID)
	require.Equal(t, "OpenAI main", identity.APIKey.GroupName)
	require.Equal(t, PlatformOpenAI, identity.APIKey.GroupPlatform)
}

func TestGetNextChatWorkspaceIdentityRejectsNonManagedKey(t *testing.T) {
	repo := &nextChatAPIKeyRepoStub{keys: []APIKey{
		{ID: 2, UserID: 42, Name: "normal key", Key: "sk-user", Status: StatusActive},
	}}
	userRepo := &nextChatUserRepoStub{user: &User{ID: 42, Status: StatusActive}}
	svc := NewAPIKeyService(repo, userRepo, nil, nil, nil, nil, &config.Config{})

	identity, err := svc.GetNextChatWorkspaceIdentity(context.Background(), 42, 2)

	require.Nil(t, identity)
	require.ErrorIs(t, err, ErrInsufficientPerms)
}

func TestSetNextChatManagedKeyGroupUpdatesToSelectableGroup(t *testing.T) {
	currentGroupID := int64(7)
	targetGroupID := int64(8)
	repo := &nextChatAPIKeyRepoStub{keys: []APIKey{
		{
			ID:      1,
			UserID:  42,
			Name:    "user key",
			Key:     "sk-user",
			Status:  StatusActive,
			GroupID: &targetGroupID,
			Group:   &Group{ID: targetGroupID, Name: "Grok backup", Platform: PlatformGrok, Status: StatusActive},
		},
		{
			ID:      2,
			UserID:  42,
			Name:    NextChatManagedAPIKeyName,
			Key:     "sk-managed",
			Status:  StatusActive,
			GroupID: &currentGroupID,
		},
	}}
	userRepo := &nextChatUserRepoStub{user: &User{ID: 42, Status: StatusActive}}
	groupRepo := &nextChatGroupRepoStub{groups: []Group{
		{ID: currentGroupID, Name: "OpenAI main", Platform: PlatformOpenAI, Status: StatusActive},
		{ID: targetGroupID, Name: "Grok backup", Platform: PlatformGrok, Status: StatusActive},
	}}
	svc := NewAPIKeyService(repo, userRepo, groupRepo, nil, nil, nil, &config.Config{})

	identity, err := svc.SetNextChatManagedKeyGroup(context.Background(), 42, 2, targetGroupID)

	require.NoError(t, err)
	require.NotNil(t, identity.APIKey.GroupID)
	require.Equal(t, targetGroupID, *identity.APIKey.GroupID)
	require.Len(t, repo.updated, 1)
	require.Equal(t, targetGroupID, *repo.updated[0].GroupID)
}

func TestSetNextChatManagedKeyGroupRejectsUnselectableGroup(t *testing.T) {
	currentGroupID := int64(7)
	repo := &nextChatAPIKeyRepoStub{keys: []APIKey{
		{ID: 2, UserID: 42, Name: NextChatManagedAPIKeyName, Key: "sk-managed", Status: StatusActive, GroupID: &currentGroupID},
	}}
	userRepo := &nextChatUserRepoStub{user: &User{ID: 42, Status: StatusActive}}
	groupRepo := &nextChatGroupRepoStub{groups: []Group{
		{ID: currentGroupID, Name: "OpenAI main", Platform: PlatformOpenAI, Status: StatusActive},
		{ID: 99, Name: "Other user group", Platform: PlatformGrok, Status: StatusActive},
	}}
	svc := NewAPIKeyService(repo, userRepo, groupRepo, nil, nil, nil, &config.Config{})

	identity, err := svc.SetNextChatManagedKeyGroup(context.Background(), 42, 2, 99)

	require.Nil(t, identity)
	require.ErrorIs(t, err, ErrInsufficientPerms)
	require.Empty(t, repo.updated)
}

func TestGetNextChatWorkspaceModelsGroupsModelsBySelectableGroup(t *testing.T) {
	openAIGroupID := int64(7)
	grokGroupID := int64(8)
	repo := &nextChatAPIKeyRepoStub{keys: []APIKey{
		{
			ID:      1,
			UserID:  42,
			Name:    "openai user key",
			Key:     "sk-user-openai",
			Status:  StatusActive,
			GroupID: &openAIGroupID,
			Group:   &Group{ID: openAIGroupID, Name: "OpenAI main", Platform: PlatformOpenAI, Status: StatusActive, SortOrder: 1},
		},
		{
			ID:      2,
			UserID:  42,
			Name:    "grok user key",
			Key:     "sk-user-grok",
			Status:  StatusActive,
			GroupID: &grokGroupID,
			Group:   &Group{ID: grokGroupID, Name: "Grok backup", Platform: PlatformGrok, Status: StatusActive, SortOrder: 2},
		},
		{
			ID:      3,
			UserID:  42,
			Name:    NextChatManagedAPIKeyName,
			Key:     "sk-managed",
			Status:  StatusActive,
			GroupID: &openAIGroupID,
		},
	}}
	userRepo := &nextChatUserRepoStub{user: &User{ID: 42, Status: StatusActive}}
	groupRepo := &nextChatGroupRepoStub{groups: []Group{
		{ID: openAIGroupID, Name: "OpenAI main", Platform: PlatformOpenAI, Status: StatusActive, SortOrder: 1},
		{ID: grokGroupID, Name: "Grok backup", Platform: PlatformGrok, Status: StatusActive, SortOrder: 2},
	}}
	apiKeySvc := NewAPIKeyService(repo, userRepo, groupRepo, &nextChatSubscriptionRepoStub{}, nil, nil, &config.Config{})
	catalogSvc := NewModelCatalogService(&nextChatModelCatalogRepoStub{entries: []SiteModelCatalogEntry{
		{ModelName: "gpt-4o-mini", Platform: PlatformOpenAI, VisibleAuth: true, SortOrder: 1},
		{ModelName: "grok-4-fast", Platform: PlatformGrok, VisibleAuth: true, SortOrder: 2},
		{ModelName: "claude-fable-5", Platform: PlatformAnthropic, VisibleAuth: true, SortOrder: 3, GroupIDs: []int64{grokGroupID}},
	}}, nil, nil, nil, nil, apiKeySvc, &nextChatAvailableModelResolverStub{modelsByGroup: map[int64][]string{
		openAIGroupID: []string{"gpt-4o-mini"},
		grokGroupID:   []string{"grok-4-fast"},
	}})

	models, err := catalogSvc.GetNextChatWorkspaceModels(context.Background(), 42, 3)

	require.NoError(t, err)
	require.Equal(t, "/v1/models", models.Source)
	require.NotNil(t, models.SelectedGroupID)
	require.Equal(t, openAIGroupID, *models.SelectedGroupID)
	require.Len(t, models.Groups, 2)
	require.Equal(t, openAIGroupID, models.Groups[0].ID)
	require.True(t, models.Groups[0].IsCurrent)
	require.Equal(t, []string{"gpt-4o-mini"}, collectNextChatModelNames(models.Groups[0].Models))
	require.Equal(t, grokGroupID, models.Groups[1].ID)
	require.Equal(t, []string{"grok-4-fast"}, collectNextChatModelNames(models.Groups[1].Models))
	require.NotContains(t, collectNextChatModelNames(models.Groups[1].Models), "claude-fable-5")
	require.Equal(t, "gpt-4o-mini", models.DefaultModel)
}

func TestGetNextChatWorkspaceModelsUsesCurrentGroupDefaultModel(t *testing.T) {
	openAIGroupID := int64(7)
	grokGroupID := int64(8)
	repo := &nextChatAPIKeyRepoStub{keys: []APIKey{
		{
			ID:      1,
			UserID:  42,
			Name:    "openai user key",
			Key:     "sk-user-openai",
			Status:  StatusActive,
			GroupID: &openAIGroupID,
			Group:   &Group{ID: openAIGroupID, Name: "OpenAI main", Platform: PlatformOpenAI, Status: StatusActive, SortOrder: 1},
		},
		{
			ID:      2,
			UserID:  42,
			Name:    "grok user key",
			Key:     "sk-user-grok",
			Status:  StatusActive,
			GroupID: &grokGroupID,
			Group:   &Group{ID: grokGroupID, Name: "Grok backup", Platform: PlatformGrok, Status: StatusActive, SortOrder: 2},
		},
		{
			ID:      3,
			UserID:  42,
			Name:    NextChatManagedAPIKeyName,
			Key:     "sk-managed",
			Status:  StatusActive,
			GroupID: &grokGroupID,
		},
	}}
	userRepo := &nextChatUserRepoStub{user: &User{ID: 42, Status: StatusActive}}
	groupRepo := &nextChatGroupRepoStub{groups: []Group{
		{ID: openAIGroupID, Name: "OpenAI main", Platform: PlatformOpenAI, Status: StatusActive, SortOrder: 1},
		{ID: grokGroupID, Name: "Grok backup", Platform: PlatformGrok, Status: StatusActive, SortOrder: 2},
	}}
	apiKeySvc := NewAPIKeyService(repo, userRepo, groupRepo, &nextChatSubscriptionRepoStub{}, nil, nil, &config.Config{})
	catalogSvc := NewModelCatalogService(&nextChatModelCatalogRepoStub{entries: []SiteModelCatalogEntry{
		{ModelName: "gpt-4o-mini", Platform: PlatformOpenAI, VisibleAuth: true, SortOrder: 1},
		{ModelName: "grok-4-fast", Platform: PlatformGrok, VisibleAuth: true, SortOrder: 2},
	}}, nil, nil, nil, nil, apiKeySvc, &nextChatAvailableModelResolverStub{modelsByGroup: map[int64][]string{
		openAIGroupID: []string{"gpt-4o-mini"},
		grokGroupID:   []string{"grok-4-fast"},
	}})

	models, err := catalogSvc.GetNextChatWorkspaceModels(context.Background(), 42, 3)

	require.NoError(t, err)
	require.Equal(t, "grok-4-fast", models.DefaultModel)
	require.False(t, models.Groups[0].IsCurrent)
	require.True(t, models.Groups[1].IsCurrent)
	require.Equal(t, PlatformGrok, models.Groups[1].Models[0].Platform)
}

func TestGetNextChatWorkspaceModelsDoesNotFallbackToCatalogWithoutMappedModels(t *testing.T) {
	grokGroupID := int64(8)
	repo := &nextChatAPIKeyRepoStub{keys: []APIKey{
		{
			ID:      1,
			UserID:  42,
			Name:    "grok user key",
			Key:     "sk-user-grok",
			Status:  StatusActive,
			GroupID: &grokGroupID,
			Group:   &Group{ID: grokGroupID, Name: "Grok backup", Platform: PlatformGrok, Status: StatusActive, SortOrder: 1},
		},
		{
			ID:      2,
			UserID:  42,
			Name:    NextChatManagedAPIKeyName,
			Key:     "sk-managed",
			Status:  StatusActive,
			GroupID: &grokGroupID,
		},
	}}
	userRepo := &nextChatUserRepoStub{user: &User{ID: 42, Status: StatusActive}}
	groupRepo := &nextChatGroupRepoStub{groups: []Group{
		{ID: grokGroupID, Name: "Grok backup", Platform: PlatformGrok, Status: StatusActive, SortOrder: 1},
	}}
	apiKeySvc := NewAPIKeyService(repo, userRepo, groupRepo, &nextChatSubscriptionRepoStub{}, nil, nil, &config.Config{})
	catalogSvc := NewModelCatalogService(&nextChatModelCatalogRepoStub{entries: []SiteModelCatalogEntry{
		{ModelName: "grok-4-fast", Platform: PlatformGrok, VisibleAuth: true, SortOrder: 1},
	}}, nil, nil, nil, nil, apiKeySvc, &nextChatAvailableModelResolverStub{modelsByGroup: map[int64][]string{
		grokGroupID: nil,
	}})

	models, err := catalogSvc.GetNextChatWorkspaceModels(context.Background(), 42, 2)

	require.NoError(t, err)
	require.Len(t, models.Groups, 1)
	require.Empty(t, models.Groups[0].Models)
	require.Empty(t, models.DefaultModel)
}

func TestGetNextChatWorkspaceModelsDoesNotUseOtherGroupDefaultWhenCurrentGroupEmpty(t *testing.T) {
	openAIGroupID := int64(7)
	grokGroupID := int64(8)
	repo := &nextChatAPIKeyRepoStub{keys: []APIKey{
		{
			ID:      1,
			UserID:  42,
			Name:    "openai user key",
			Key:     "sk-user-openai",
			Status:  StatusActive,
			GroupID: &openAIGroupID,
			Group:   &Group{ID: openAIGroupID, Name: "OpenAI main", Platform: PlatformOpenAI, Status: StatusActive, SortOrder: 1},
		},
		{
			ID:      2,
			UserID:  42,
			Name:    "grok user key",
			Key:     "sk-user-grok",
			Status:  StatusActive,
			GroupID: &grokGroupID,
			Group:   &Group{ID: grokGroupID, Name: "Grok empty", Platform: PlatformGrok, Status: StatusActive, SortOrder: 2},
		},
		{
			ID:      3,
			UserID:  42,
			Name:    NextChatManagedAPIKeyName,
			Key:     "sk-managed",
			Status:  StatusActive,
			GroupID: &grokGroupID,
		},
	}}
	userRepo := &nextChatUserRepoStub{user: &User{ID: 42, Status: StatusActive}}
	groupRepo := &nextChatGroupRepoStub{groups: []Group{
		{ID: openAIGroupID, Name: "OpenAI main", Platform: PlatformOpenAI, Status: StatusActive, SortOrder: 1},
		{ID: grokGroupID, Name: "Grok empty", Platform: PlatformGrok, Status: StatusActive, SortOrder: 2},
	}}
	apiKeySvc := NewAPIKeyService(repo, userRepo, groupRepo, &nextChatSubscriptionRepoStub{}, nil, nil, &config.Config{})
	catalogSvc := NewModelCatalogService(&nextChatModelCatalogRepoStub{entries: []SiteModelCatalogEntry{
		{ModelName: "gpt-4o-mini", Platform: PlatformOpenAI, VisibleAuth: true, SortOrder: 1},
	}}, nil, nil, nil, nil, apiKeySvc, &nextChatAvailableModelResolverStub{modelsByGroup: map[int64][]string{
		openAIGroupID: []string{"gpt-4o-mini"},
		grokGroupID:   nil,
	}})

	models, err := catalogSvc.GetNextChatWorkspaceModels(context.Background(), 42, 3)

	require.NoError(t, err)
	require.Empty(t, models.DefaultModel)
	require.True(t, models.Groups[1].IsCurrent)
	require.Empty(t, models.Groups[1].Models)
	require.Equal(t, []string{"gpt-4o-mini"}, collectNextChatModelNames(models.Groups[0].Models))
}

func TestGetNextChatWorkspaceModelsAppliesGroupCustomModelsList(t *testing.T) {
	openAIGroupID := int64(7)
	repo := &nextChatAPIKeyRepoStub{keys: []APIKey{
		{
			ID:      1,
			UserID:  42,
			Name:    "openai user key",
			Key:     "sk-user-openai",
			Status:  StatusActive,
			GroupID: &openAIGroupID,
			Group: &Group{
				ID:       openAIGroupID,
				Name:     "OpenAI limited",
				Platform: PlatformOpenAI,
				Status:   StatusActive,
				ModelsListConfig: GroupModelsListConfig{
					Enabled: true,
					Models:  []string{"gpt-4o-mini"},
				},
			},
		},
		{
			ID:      2,
			UserID:  42,
			Name:    NextChatManagedAPIKeyName,
			Key:     "sk-managed",
			Status:  StatusActive,
			GroupID: &openAIGroupID,
		},
	}}
	userRepo := &nextChatUserRepoStub{user: &User{ID: 42, Status: StatusActive}}
	groupRepo := &nextChatGroupRepoStub{groups: []Group{
		{
			ID:       openAIGroupID,
			Name:     "OpenAI limited",
			Platform: PlatformOpenAI,
			Status:   StatusActive,
			ModelsListConfig: GroupModelsListConfig{
				Enabled: true,
				Models:  []string{"gpt-4o-mini"},
			},
		},
	}}
	apiKeySvc := NewAPIKeyService(repo, userRepo, groupRepo, &nextChatSubscriptionRepoStub{}, nil, nil, &config.Config{})
	catalogSvc := NewModelCatalogService(&nextChatModelCatalogRepoStub{entries: []SiteModelCatalogEntry{
		{ModelName: "gpt-4o-mini", Platform: PlatformOpenAI, VisibleAuth: true, SortOrder: 1},
		{ModelName: "gpt-5.4", Platform: PlatformOpenAI, VisibleAuth: true, SortOrder: 2},
	}}, nil, nil, nil, nil, apiKeySvc, &nextChatAvailableModelResolverStub{modelsByGroup: map[int64][]string{
		openAIGroupID: []string{"gpt-5.4", "gpt-4o-mini"},
	}})

	models, err := catalogSvc.GetNextChatWorkspaceModels(context.Background(), 42, 2)

	require.NoError(t, err)
	require.Equal(t, []string{"gpt-4o-mini"}, collectNextChatModelNames(models.Groups[0].Models))
	require.Equal(t, "gpt-4o-mini", models.DefaultModel)
	require.NotContains(t, collectNextChatModelNames(models.Groups[0].Models), "gpt-5.4")
}

func TestGetNextChatWorkspaceModelsCustomModelsListEnabledEmptyFailsClosed(t *testing.T) {
	openAIGroupID := int64(7)
	repo := &nextChatAPIKeyRepoStub{keys: []APIKey{
		{
			ID:      1,
			UserID:  42,
			Name:    "openai user key",
			Key:     "sk-user-openai",
			Status:  StatusActive,
			GroupID: &openAIGroupID,
			Group: &Group{
				ID:       openAIGroupID,
				Name:     "OpenAI empty whitelist",
				Platform: PlatformOpenAI,
				Status:   StatusActive,
				ModelsListConfig: GroupModelsListConfig{
					Enabled: true,
				},
			},
		},
		{
			ID:      2,
			UserID:  42,
			Name:    NextChatManagedAPIKeyName,
			Key:     "sk-managed",
			Status:  StatusActive,
			GroupID: &openAIGroupID,
		},
	}}
	userRepo := &nextChatUserRepoStub{user: &User{ID: 42, Status: StatusActive}}
	groupRepo := &nextChatGroupRepoStub{groups: []Group{
		{
			ID:       openAIGroupID,
			Name:     "OpenAI empty whitelist",
			Platform: PlatformOpenAI,
			Status:   StatusActive,
			ModelsListConfig: GroupModelsListConfig{
				Enabled: true,
			},
		},
	}}
	apiKeySvc := NewAPIKeyService(repo, userRepo, groupRepo, &nextChatSubscriptionRepoStub{}, nil, nil, &config.Config{})
	catalogSvc := NewModelCatalogService(&nextChatModelCatalogRepoStub{entries: []SiteModelCatalogEntry{
		{ModelName: "claude-fable-5", Platform: PlatformAnthropic, VisibleAuth: true, SortOrder: 1},
		{ModelName: "gpt-4o-mini", Platform: PlatformOpenAI, VisibleAuth: true, SortOrder: 2},
	}}, nil, nil, nil, nil, apiKeySvc, &nextChatAvailableModelResolverStub{modelsByGroup: map[int64][]string{
		openAIGroupID: []string{"gpt-4o-mini", "claude-fable-5"},
	}})

	models, err := catalogSvc.GetNextChatWorkspaceModels(context.Background(), 42, 2)

	require.NoError(t, err)
	require.Empty(t, models.Groups[0].Models)
	require.Empty(t, models.DefaultModel)
}

func TestNextChatWorkspaceModelMetadataDoesNotBorrowBillingDataAcrossGroups(t *testing.T) {
	openAIGroupID := int64(7)
	grokGroupID := int64(8)
	borrowedInputPrice := 0.42
	metadata := buildNextChatWorkspaceModelMetadata([]MyModelPricingRow{
		{
			Name:                "gpt-4o-mini",
			Platform:            PlatformOpenAI,
			Channel:             "billing-channel-from-other-group",
			EffectiveInputPrice: &borrowedInputPrice,
			SortOrder:           9,
			Groups:              []MyModelPricingGroup{{ID: openAIGroupID, Name: "OpenAI main"}},
		},
	})

	sameGroup := metadata.lookup(openAIGroupID, PlatformOpenAI, "gpt-4o-mini")
	otherGroup := metadata.lookup(grokGroupID, PlatformOpenAI, "gpt-4o-mini")

	require.Equal(t, "billing-channel-from-other-group", sameGroup.channel)
	require.NotNil(t, sameGroup.effectiveInputPrice)
	require.Empty(t, otherGroup.channel)
	require.Nil(t, otherGroup.effectiveInputPrice)
	require.Zero(t, otherGroup.sortOrder)
}

func TestBuildNextChatPromptCatalogFromPublicPromptsUsesPublishedLibraryContent(t *testing.T) {
	catalog := BuildNextChatPromptCatalogFromPublicPrompts([]PublicPrompt{
		{
			ID:          88,
			Title:       "爆款短视频脚本",
			Description: "把商品卖点改写成短视频脚本",
			Purpose:     "marketing",
			Version:     3,
			PromptText:  "请根据商品卖点输出 30 秒短视频脚本。",
		},
	})

	require.Equal(t, []NextChatPrompt{
		{
			ID:          "prompt-88-v3",
			Title:       "爆款短视频脚本",
			Description: "把商品卖点改写成短视频脚本",
			Content:     "请根据商品卖点输出 30 秒短视频脚本。",
			Category:    "marketing",
		},
	}, catalog.ChatPrompts)
	require.NotEmpty(t, catalog.ImageTemplates.Intents)
}

func collectNextChatModelNames(models []NextChatWorkspaceModel) []string {
	names := make([]string, 0, len(models))
	for _, model := range models {
		names = append(names, model.Name)
	}
	return names
}

func filterNextChatAPIKeyRepoKeys(userID int64, keys []APIKey, filters APIKeyListFilters) []APIKey {
	result := make([]APIKey, 0, len(keys))
	search := strings.ToLower(filters.Search)
	for _, key := range keys {
		if key.UserID != userID {
			continue
		}
		if filters.Status != "" && key.Status != filters.Status {
			continue
		}
		if search != "" &&
			!strings.Contains(strings.ToLower(key.Name), search) &&
			!strings.Contains(strings.ToLower(key.Key), search) {
			continue
		}
		if filters.ExcludeNamePrefix != "" && strings.HasPrefix(key.Name, filters.ExcludeNamePrefix) {
			continue
		}
		result = append(result, key)
	}
	return result
}
