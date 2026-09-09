package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type nextChatAPIKeyRepoStub struct {
	APIKeyRepository
	keys          []APIKey
	created       []APIKey
	updated       []APIKey
	updatedFields []APIKeyUpdateFields
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

func (s *nextChatAPIKeyRepoStub) Update(_ context.Context, key *APIKey, fields APIKeyUpdateFields) error {
	clone := *key
	s.updated = append(s.updated, clone)
	s.updatedFields = append(s.updatedFields, fields)
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
	modelsByGroup                 map[int64][]string
	modelsByGroupAndPlatform      map[int64]map[string][]string
	schedulablePlatformsByGroupID map[int64]map[string]struct{}
}

// nextChatAvailableModelsOnlyStub deliberately models an older integration
// that can list models but cannot prove a concrete schedulable platform.
type nextChatAvailableModelsOnlyStub struct {
	modelsByGroup map[int64][]string
}

func newNextChatWorkspaceMediaCatalogService(
	group Group,
	entries []SiteModelCatalogEntry,
	models []string,
) *ModelCatalogService {
	group.Status = StatusActive
	groupID := group.ID
	apiKeyService := NewAPIKeyService(
		&nextChatAPIKeyRepoStub{keys: []APIKey{
			{
				ID:      1,
				UserID:  42,
				Name:    "workspace group key",
				Key:     "sk-workspace-group",
				Status:  StatusActive,
				GroupID: &groupID,
				Group:   &group,
			},
			{
				ID:      2,
				UserID:  42,
				Name:    NextChatManagedAPIKeyName,
				Key:     "sk-managed-workspace",
				Status:  StatusActive,
				GroupID: &groupID,
			},
		}},
		&nextChatUserRepoStub{user: &User{ID: 42, Status: StatusActive}},
		&nextChatGroupRepoStub{groups: []Group{group}},
		&nextChatSubscriptionRepoStub{},
		nil,
		nil,
		&config.Config{},
	)
	return NewModelCatalogService(
		&nextChatModelCatalogRepoStub{entries: entries},
		nil,
		nil,
		nil,
		nil,
		apiKeyService,
		&nextChatAvailableModelResolverStub{
			modelsByGroup: map[int64][]string{groupID: models},
			schedulablePlatformsByGroupID: map[int64]map[string]struct{}{
				groupID: {group.Platform: {}},
			},
		},
	)
}

func (s *nextChatAvailableModelResolverStub) GetAvailableModels(_ context.Context, groupID *int64, platform string) []string {
	if s == nil || groupID == nil {
		return nil
	}
	if byPlatform, ok := s.modelsByGroupAndPlatform[*groupID]; ok {
		return append([]string(nil), byPlatform[platform]...)
	}
	return append([]string(nil), s.modelsByGroup[*groupID]...)
}

func (s *nextChatAvailableModelResolverStub) GetSchedulablePlatforms(_ context.Context, groupID *int64) map[string]struct{} {
	if s == nil || groupID == nil {
		return nil
	}
	platforms := s.schedulablePlatformsByGroupID[*groupID]
	result := make(map[string]struct{}, len(platforms))
	for platform := range platforms {
		result[platform] = struct{}{}
	}
	return result
}

func (s *nextChatAvailableModelsOnlyStub) GetAvailableModels(_ context.Context, groupID *int64, _ string) []string {
	if s == nil || groupID == nil {
		return nil
	}
	return append([]string(nil), s.modelsByGroup[*groupID]...)
}

func TestGetNextChatSelectableGroupsMergesAvailableGroupsWithOwnedKeyGroups(t *testing.T) {
	domesticGroupID := int64(11)
	videoGroupID := int64(22)
	svc := NewAPIKeyService(
		&nextChatAPIKeyRepoStub{keys: []APIKey{{ID: 1, UserID: 42, Name: "ordinary key", Key: "sk-ordinary", GroupID: &domesticGroupID, Status: StatusActive}}},
		&nextChatUserRepoStub{user: &User{ID: 42}},
		&nextChatGroupRepoStub{groups: []Group{
			{ID: domesticGroupID, Name: "国产分组", Status: StatusActive},
			{ID: videoGroupID, Name: "video 分组", Status: StatusActive},
		}},
		&nextChatSubscriptionRepoStub{},
		nil,
		nil,
		&config.Config{},
	)

	groups, err := svc.GetNextChatSelectableGroups(context.Background(), 42)
	require.NoError(t, err)
	require.Len(t, groups, 2)
	require.ElementsMatch(t, []int64{domesticGroupID, videoGroupID}, []int64{groups[0].ID, groups[1].ID})
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

func TestIssueNextChatManagedSessionsKeepsPurposesIndependent(t *testing.T) {
	chatGroupID := int64(7)
	imageGroupID := int64(8)
	videoGroupID := int64(9)
	repo := &nextChatAPIKeyRepoStub{keys: []APIKey{
		{ID: 2, UserID: 42, Name: NextChatManagedAPIKeyName, Key: "sk-chat", Status: StatusActive, GroupID: &chatGroupID},
		{ID: 3, UserID: 42, Name: NextChatManagedImageAPIKeyName, Key: "sk-image", Status: StatusActive, GroupID: &imageGroupID},
		{ID: 4, UserID: 42, Name: NextChatManagedVideoAPIKeyName, Key: "sk-video", Status: StatusActive, GroupID: &videoGroupID},
	}}
	userRepo := &nextChatUserRepoStub{user: &User{ID: 42, Status: StatusActive}}
	groupRepo := &nextChatGroupRepoStub{groups: []Group{
		{ID: chatGroupID, Platform: PlatformOpenAI, Status: StatusActive},
		{ID: imageGroupID, Platform: PlatformGrok, Status: StatusActive},
		{ID: videoGroupID, Platform: PlatformOpenAI, Status: StatusActive},
	}}
	svc := NewAPIKeyService(repo, userRepo, groupRepo, &nextChatSubscriptionRepoStub{}, nil, nil, &config.Config{})

	sessions, err := svc.IssueNextChatManagedSessions(context.Background(), 42)
	require.NoError(t, err)
	require.Equal(t, int64(2), sessions.Chat.KeyID)
	require.Equal(t, int64(3), sessions.Image.KeyID)
	require.Equal(t, int64(4), sessions.Video.KeyID)
	require.Equal(t, NextChatSessionPurposeChat, sessions.Chat.Purpose)
	require.Equal(t, NextChatSessionPurposeImage, sessions.Image.Purpose)
	require.Equal(t, NextChatSessionPurposeVideo, sessions.Video.Purpose)

	_, err = svc.SetNextChatManagedSessionGroup(context.Background(), 42, NextChatSessionPurposeChat, imageGroupID)
	require.NoError(t, err)
	require.Equal(t, imageGroupID, *repo.keys[0].GroupID)
	require.Equal(t, imageGroupID, *repo.keys[1].GroupID, "image session must not be modified by chat switch")
	require.Equal(t, videoGroupID, *repo.keys[2].GroupID, "video session must not be modified by chat switch")

	_, err = svc.SetNextChatManagedSessionGroup(context.Background(), 42, NextChatSessionPurposeImage, chatGroupID)
	require.NoError(t, err)
	require.Equal(t, imageGroupID, *repo.keys[0].GroupID, "chat session must not be modified by image switch")
	require.Equal(t, chatGroupID, *repo.keys[1].GroupID)
	require.Equal(t, videoGroupID, *repo.keys[2].GroupID, "video session must not be modified by image switch")

	_, err = svc.SetNextChatManagedSessionGroup(context.Background(), 42, NextChatSessionPurposeVideo, chatGroupID)
	require.NoError(t, err)
	require.Equal(t, imageGroupID, *repo.keys[0].GroupID, "chat session must not be modified by video switch")
	require.Equal(t, chatGroupID, *repo.keys[1].GroupID, "image session must not be modified by video switch")
	require.Equal(t, chatGroupID, *repo.keys[2].GroupID)
}

func TestIssueNextChatManagedSessionForVideoDoesNotReuseChatKey(t *testing.T) {
	groupID := int64(7)
	repo := &nextChatAPIKeyRepoStub{keys: []APIKey{
		{ID: 2, UserID: 42, Name: NextChatManagedAPIKeyName, Key: "sk-chat", Status: StatusActive, GroupID: &groupID},
	}}
	userRepo := &nextChatUserRepoStub{user: &User{ID: 42, Status: StatusActive}}
	groupRepo := &nextChatGroupRepoStub{groups: []Group{{ID: groupID, Platform: PlatformOpenAI, Status: StatusActive}}}
	svc := NewAPIKeyService(repo, userRepo, groupRepo, &nextChatSubscriptionRepoStub{}, nil, nil, &config.Config{Default: config.DefaultConfig{APIKeyPrefix: "sk-test-"}})

	video, err := svc.IssueNextChatManagedSessionForPurpose(context.Background(), 42, NextChatSessionPurposeVideo)
	require.NoError(t, err)
	require.NotEqual(t, int64(2), video.KeyID)
	require.Equal(t, NextChatManagedVideoAPIKeyName+"/7", repo.created[0].Name)
	require.Equal(t, NextChatGroupPinnedSessionBinding, video.Binding)
}

func TestIssueNextChatManagedSessionForImageDoesNotReuseChatKey(t *testing.T) {
	groupID := int64(7)
	repo := &nextChatAPIKeyRepoStub{keys: []APIKey{
		{ID: 2, UserID: 42, Name: NextChatManagedAPIKeyName, Key: "sk-chat", Status: StatusActive, GroupID: &groupID},
	}}
	userRepo := &nextChatUserRepoStub{user: &User{ID: 42, Status: StatusActive}}
	groupRepo := &nextChatGroupRepoStub{groups: []Group{{ID: groupID, Platform: PlatformOpenAI, Status: StatusActive}}}
	svc := NewAPIKeyService(repo, userRepo, groupRepo, &nextChatSubscriptionRepoStub{}, nil, nil, &config.Config{Default: config.DefaultConfig{APIKeyPrefix: "sk-test-"}})

	image, err := svc.IssueNextChatManagedSessionForPurpose(context.Background(), 42, NextChatSessionPurposeImage)
	require.NoError(t, err)
	require.NotEqual(t, int64(2), image.KeyID)
	require.Equal(t, NextChatManagedImageAPIKeyName+"/7", repo.created[0].Name)
	require.Equal(t, NextChatGroupPinnedSessionBinding, image.Binding)
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
	require.Equal(t, NextChatManagedChatAPIKeyName+"/2", repo.created[0].Name)
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

func TestIssueNextChatManagedSessionSkipsUnauthorizedExistingUserKeyGroup(t *testing.T) {
	unauthorizedGroupID := int64(62)
	publicGroupID := int64(31)
	repo := &nextChatAPIKeyRepoStub{keys: []APIKey{
		{
			ID:      1,
			UserID:  42,
			Name:    "expired subscription key",
			Key:     "sk-expired",
			Status:  StatusActive,
			GroupID: &unauthorizedGroupID,
			Group:   &Group{ID: unauthorizedGroupID, Platform: PlatformOpenAI, IsExclusive: true, Status: StatusActive, SortOrder: 1},
		},
	}}
	userRepo := &nextChatUserRepoStub{user: &User{ID: 42, Status: StatusActive}}
	groupRepo := &nextChatGroupRepoStub{groups: []Group{
		{ID: unauthorizedGroupID, Platform: PlatformOpenAI, IsExclusive: true, Status: StatusActive, SortOrder: 1},
		{ID: publicGroupID, Platform: PlatformOpenAI, Status: StatusActive, SortOrder: 2},
	}}
	svc := NewAPIKeyService(repo, userRepo, groupRepo, &nextChatSubscriptionRepoStub{}, nil, nil, &config.Config{
		Default: config.DefaultConfig{APIKeyPrefix: "sk-test-"},
	})

	session, err := svc.IssueNextChatManagedSession(context.Background(), 42)

	require.NoError(t, err)
	require.NotNil(t, session)
	require.Len(t, repo.created, 1)
	require.NotNil(t, repo.created[0].GroupID)
	require.Equal(t, publicGroupID, *repo.created[0].GroupID)
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

func TestIssueNextChatManagedSessionReplacesInvalidLegacyGroupWithoutMutation(t *testing.T) {
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
	svc := NewAPIKeyService(
		repo,
		&nextChatUserRepoStub{user: &User{ID: 42, Status: StatusActive}},
		&nextChatGroupRepoStub{groups: []Group{{ID: desiredGroupID, Platform: PlatformOpenAI, Status: StatusActive}}},
		&nextChatSubscriptionRepoStub{}, nil, nil,
		&config.Config{Default: config.DefaultConfig{APIKeyPrefix: "sk-test-"}},
	)

	session, err := svc.IssueNextChatManagedSession(context.Background(), 42)

	require.NoError(t, err)
	require.NotEqual(t, int64(2), session.KeyID)
	require.NotNil(t, session.GroupID)
	require.Equal(t, desiredGroupID, *session.GroupID)
	require.Equal(t, NextChatGroupPinnedSessionBinding, session.Binding)
	require.Len(t, repo.created, 1)
	require.Empty(t, repo.updated)
	require.NotNil(t, repo.keys[1].GroupID)
	require.Equal(t, staleGroupID, *repo.keys[1].GroupID)
}

func TestIssueNextChatManagedSessionDoesNotClearLegacyKeyWhenNoSelectableGroups(t *testing.T) {
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
	svc := NewAPIKeyService(
		repo,
		&nextChatUserRepoStub{user: &User{ID: 42, Status: StatusActive}},
		nil, nil, nil, nil,
		&config.Config{Default: config.DefaultConfig{APIKeyPrefix: "sk-test-"}},
	)

	session, err := svc.IssueNextChatManagedSession(context.Background(), 42)

	require.NoError(t, err)
	require.NotEqual(t, int64(2), session.KeyID)
	require.Nil(t, session.GroupID)
	require.Len(t, repo.created, 1)
	require.Empty(t, repo.updated)
	require.NotNil(t, repo.keys[0].GroupID)
	require.Equal(t, staleGroupID, *repo.keys[0].GroupID)
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
		Role:          RoleAdmin,
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
	require.Equal(t, RoleAdmin, identity.User.Role)
	require.True(t, identity.User.IsAdmin)
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
	require.ErrorIs(t, err, ErrGroupNotAllowed)
	require.Empty(t, repo.updated)
}

func TestSwitchNextChatManagedSessionGroupIssuesImmutableReplacement(t *testing.T) {
	chatGroupID := int64(7)
	imageGroupID := int64(8)
	legacyGroupID := chatGroupID
	repo := &nextChatAPIKeyRepoStub{keys: []APIKey{
		{
			ID:      1,
			UserID:  42,
			Name:    "ordinary chat key",
			Key:     "sk-user-chat",
			Status:  StatusActive,
			GroupID: &chatGroupID,
			Group:   &Group{ID: chatGroupID, Name: "Chat", Platform: PlatformOpenAI, Status: StatusActive},
		},
		{
			ID:      2,
			UserID:  42,
			Name:    "ordinary image key",
			Key:     "sk-user-image",
			Status:  StatusActive,
			GroupID: &imageGroupID,
			Group:   &Group{ID: imageGroupID, Name: "Image", Platform: PlatformGrok, Status: StatusActive},
		},
		{
			ID:      3,
			UserID:  42,
			Name:    NextChatManagedAPIKeyName,
			Key:     "sk-managed-legacy",
			Status:  StatusActive,
			GroupID: &legacyGroupID,
		},
	}}
	svc := NewAPIKeyService(
		repo,
		&nextChatUserRepoStub{user: &User{ID: 42, Status: StatusActive}},
		&nextChatGroupRepoStub{groups: []Group{
			{ID: chatGroupID, Name: "Chat", Platform: PlatformOpenAI, Status: StatusActive},
			{ID: imageGroupID, Name: "Image", Platform: PlatformGrok, Status: StatusActive},
		}},
		&nextChatSubscriptionRepoStub{}, nil, nil,
		&config.Config{Default: config.DefaultConfig{APIKeyPrefix: "sk-test-"}},
	)

	first, firstIdentity, err := svc.SwitchNextChatManagedSessionGroup(
		context.Background(), 42, 3, NextChatSessionPurposeChat, imageGroupID,
	)
	require.NoError(t, err)
	require.NotNil(t, first)
	require.NotEqual(t, int64(3), first.KeyID)
	require.NotNil(t, first.GroupID)
	require.Equal(t, imageGroupID, *first.GroupID)
	require.Equal(t, NextChatGroupPinnedSessionBinding, first.Binding)
	require.Equal(t, first.KeyID, firstIdentity.APIKey.ID)
	require.Len(t, repo.created, 1)
	require.Empty(t, repo.updated, "a group switch must never mutate the presented key")
	require.NotNil(t, repo.keys[2].GroupID)
	require.Equal(t, chatGroupID, *repo.keys[2].GroupID, "the old session remains safe for an in-flight request")

	second, secondIdentity, err := svc.SwitchNextChatManagedSessionGroup(
		context.Background(), 42, 3, NextChatSessionPurposeChat, imageGroupID,
	)
	require.NoError(t, err)
	require.Equal(t, first.KeyID, second.KeyID, "the same user/purpose/group reuses one pinned session")
	require.Equal(t, firstIdentity.APIKey.ID, secondIdentity.APIKey.ID)
	require.Len(t, repo.created, 1)
	require.Empty(t, repo.updated)
}

func TestSetNextChatManagedKeyGroupRejectsPinnedSession(t *testing.T) {
	groupID := int64(7)
	pinnedName, err := nextChatManagedScopedAPIKeyName(NextChatSessionPurposeImage, groupID)
	require.NoError(t, err)
	repo := &nextChatAPIKeyRepoStub{keys: []APIKey{{
		ID: 2, UserID: 42, Name: pinnedName, Key: "sk-pinned", Status: StatusActive, GroupID: &groupID,
	}}}
	svc := NewAPIKeyService(
		repo,
		&nextChatUserRepoStub{user: &User{ID: 42, Status: StatusActive}},
		&nextChatGroupRepoStub{groups: []Group{{ID: groupID, Status: StatusActive}}},
		&nextChatSubscriptionRepoStub{}, nil, nil, &config.Config{},
	)

	identity, err := svc.SetNextChatManagedKeyGroup(context.Background(), 42, 2, groupID+1)
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
			Group:   &Group{ID: openAIGroupID, Name: "OpenAI main", Platform: PlatformOpenAI, Status: StatusActive, SortOrder: 1, AllowLive: true},
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
		{ID: openAIGroupID, Name: "OpenAI main", Platform: PlatformOpenAI, Status: StatusActive, SortOrder: 1, AllowLive: true},
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
	require.True(t, models.Groups[0].LiveAvailable)
	require.Equal(t, []string{"gpt-4o-mini"}, collectNextChatModelNames(models.Groups[0].Models))
	require.Equal(t, grokGroupID, models.Groups[1].ID)
	require.False(t, models.Groups[1].LiveAvailable)
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
				ModelAllowlist: GroupModelAllowlist{
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
			ModelAllowlist: GroupModelAllowlist{
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
				ModelAllowlist: GroupModelAllowlist{
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
			ModelAllowlist: GroupModelAllowlist{
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

func TestGetNextChatWorkspaceModelsRequiresDeclaredMediaContractForNewImageNamedModels(t *testing.T) {
	groupID := int64(2_608_269)
	imagePrice := 0.01
	svc := newNextChatWorkspaceMediaCatalogService(
		Group{ID: groupID, Name: "OpenAI image", Platform: PlatformOpenAI, AllowImageGeneration: true, ImagePrice1K: &imagePrice},
		[]SiteModelCatalogEntry{
			{
				ModelName:   "foo-image-model",
				Platform:    PlatformOpenAI,
				VisibleAuth: true,
				GroupIDs:    []int64{groupID},
			},
			{
				ModelName:   "declared-image-model",
				Platform:    PlatformOpenAI,
				VisibleAuth: true,
				GroupIDs:    []int64{groupID},
				MediaCapabilities: json.RawMessage(`{
					"version":"2026-08-26.1",
					"adapter":"openai_images",
					"modalities":["image"],
					"image":{"operations":["create"]}
				}`),
			},
		},
		[]string{"foo-image-model", "declared-image-model", "gpt-image-1"},
	)

	workspace, err := svc.GetNextChatWorkspaceModels(context.Background(), 42, 2)

	require.NoError(t, err)
	require.Len(t, workspace.Groups, 1)
	byName := make(map[string]NextChatWorkspaceModel, len(workspace.Groups[0].Models))
	for _, model := range workspace.Groups[0].Models {
		byName[model.Name] = model
	}

	unknown := byName["foo-image-model"]
	require.Empty(t, unknown.Modalities)
	require.Nil(t, unknown.ImageCapabilities)

	declared := byName["declared-image-model"]
	require.Equal(t, []string{"image"}, declared.Modalities)
	require.NotNil(t, declared.ImageCapabilities)
	require.Equal(t, []string{"create"}, declared.ImageCapabilities.Operations)

	legacy := byName["gpt-image-1"]
	require.Equal(t, []string{"image"}, legacy.Modalities)
	require.NotNil(t, legacy.ImageCapabilities)
	require.Equal(t, []string{"create", "edit"}, legacy.ImageCapabilities.Operations)
}

func TestGetNextChatWorkspaceModelsAppliesExactCatalogMediaContracts(t *testing.T) {
	groupID := int64(2_608_262)
	imagePrice := 0.01
	svc := newNextChatWorkspaceMediaCatalogService(
		Group{ID: groupID, Name: "OpenAI image", Platform: PlatformOpenAI, AllowImageGeneration: true, ImagePrice1K: &imagePrice},
		[]SiteModelCatalogEntry{
			{
				ModelName:   "sensenova-u1-fast",
				Platform:    PlatformOpenAI,
				VisibleAuth: true,
				GroupIDs:    []int64{groupID},
				MediaCapabilities: json.RawMessage(`{
					"version":"2026-08-26.1",
					"adapter":"sensenova",
					"modalities":["image"],
					"image":{
						"operations":["create"],
						"sizing_kind":"fixed",
						"supported_sizes":["2048x2048"]
					}
				}`),
			},
			{
				// This explicit chat-only declaration must clear the old static
				// image compatibility profile for this group.
				ModelName:   "gpt-image-2",
				Platform:    PlatformOpenAI,
				VisibleAuth: true,
				GroupIDs:    []int64{groupID},
				MediaCapabilities: json.RawMessage(`{
					"version":"2026-08-26.1",
					"adapter":"chat-only",
					"modalities":["chat"]
				}`),
			},
			{
				// A same-named model declaration in another group must never
				// grant a media mode to this workspace.
				ModelName:   "other-group-video",
				Platform:    PlatformOpenAI,
				VisibleAuth: true,
				GroupIDs:    []int64{groupID + 1},
				MediaCapabilities: json.RawMessage(`{
					"version":"2026-08-26.1",
					"adapter":"grok_video",
					"modalities":["video"],
					"video":{"operations":["create"]}
				}`),
			},
			{
				// A reviewed but unsupported declaration must fail closed; only
				// rows without a declaration may use the legacy profile.
				ModelName:   "sensenova-u1.5-lite",
				Platform:    PlatformOpenAI,
				VisibleAuth: true,
				GroupIDs:    []int64{groupID},
				MediaCapabilities: json.RawMessage(`{
					"version":"2026-08-26.1",
					"adapter":"not-an-executable-adapter",
					"modalities":["image"],
					"image":{"operations":["create"]}
				}`),
			},
		},
		[]string{"sensenova-u1-fast", "gpt-image-2", "other-group-video", "sensenova-u1.5-lite"},
	)

	models, err := svc.GetNextChatWorkspaceModels(context.Background(), 42, 2)

	require.NoError(t, err)
	require.Equal(t, NextChatVideoCapabilitiesVersion, models.VideoCapabilitiesVersion)
	require.Len(t, models.Groups, 1)
	require.False(t, models.Groups[0].VideoAvailable)
	byName := make(map[string]NextChatWorkspaceModel, len(models.Groups[0].Models))
	for _, model := range models.Groups[0].Models {
		byName[model.Name] = model
	}

	fast := byName["sensenova-u1-fast"]
	require.Equal(t, []string{"image"}, fast.Modalities)
	require.Equal(t, "sensenova", fast.Adapter)
	require.Equal(t, "2026-08-26.1", fast.CapabilityVersion)
	require.NotNil(t, fast.ImageCapabilities)
	require.Equal(t, []string{"create"}, fast.ImageCapabilities.Operations)
	require.Empty(t, fast.ImageCapabilities.SupportedRatios)
	require.Empty(t, fast.ImageCapabilities.SupportedFormats)
	require.Nil(t, fast.VideoCapabilities)
	encodedImage, err := json.Marshal(fast.ImageCapabilities)
	require.NoError(t, err)
	var imageFields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(encodedImage, &imageFields))
	require.NotContains(t, imageFields, "supported_ratios")
	require.NotContains(t, imageFields, "supported_formats")
	require.NotContains(t, imageFields, "supported_aspect_ratios")
	require.NotContains(t, imageFields, "supported_output_formats")

	chatOnly := byName["gpt-image-2"]
	require.Equal(t, []string{"chat"}, chatOnly.Modalities)
	require.Equal(t, "chat-only", chatOnly.Adapter)
	require.Nil(t, chatOnly.ImageCapabilities)
	require.Nil(t, chatOnly.VideoCapabilities)

	otherGroup := byName["other-group-video"]
	require.Empty(t, otherGroup.Modalities)
	require.Empty(t, otherGroup.Adapter)
	require.Nil(t, otherGroup.ImageCapabilities)
	require.Nil(t, otherGroup.VideoCapabilities)

	unsupported := byName["sensenova-u1.5-lite"]
	require.Empty(t, unsupported.Modalities)
	require.Empty(t, unsupported.Adapter)
	require.Empty(t, unsupported.CapabilityVersion)
	require.Nil(t, unsupported.ImageCapabilities)
	require.Nil(t, unsupported.VideoCapabilities)
}

func TestGetNextChatWorkspaceModelsMarksExecutableDeclaredVideoModels(t *testing.T) {
	groupID := int64(2_608_263)
	price := 0.14
	svc := newNextChatWorkspaceMediaCatalogService(
		Group{
			ID: groupID, Name: "Grok Heavy", Platform: PlatformGrok,
			AllowImageGeneration: true, VideoPrice720P: &price,
		},
		[]SiteModelCatalogEntry{{
			ModelName:   "grok-imagine-video",
			Platform:    PlatformGrok,
			VisibleAuth: true,
			GroupIDs:    []int64{groupID},
			MediaCapabilities: json.RawMessage(`{
				"version":"2026-08-26.1",
				"adapter":"grok_video",
				"modalities":["video"],
				"video":{
					"operations":["generate"],
					"supported_resolutions":["720p"],
					"supported_aspect_ratios":["16:9"],
					"durations_seconds":[6]
				}
			}`),
		}},
		[]string{"grok-imagine-video"},
	)

	models, err := svc.GetNextChatWorkspaceModels(context.Background(), 42, 2)

	require.NoError(t, err)
	require.Len(t, models.Groups, 1)
	require.True(t, models.Groups[0].VideoAvailable)
	require.Len(t, models.Groups[0].Models, 1)
	video := models.Groups[0].Models[0]
	require.Equal(t, []string{"video"}, video.Modalities)
	require.Equal(t, MobileVideoAdapterGrok, video.Adapter)
	require.NotNil(t, video.VideoCapabilities)
	require.Equal(t, []string{"generate"}, video.VideoCapabilities.Operations)
	require.Equal(t, []string{"720p"}, video.VideoCapabilities.SupportedResolutions)
	require.Equal(t, []string{"16:9"}, video.VideoCapabilities.SupportedRatios)
	require.Equal(t, []int{6}, video.VideoCapabilities.SupportedDurations)
	encoded, err := json.Marshal(video.VideoCapabilities)
	require.NoError(t, err)
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(encoded, &fields))
	require.Contains(t, fields, "supported_resolutions")
	require.Contains(t, fields, "supported_ratios")
	require.Contains(t, fields, "supported_durations")
	require.NotContains(t, fields, "supported_aspect_ratios")
	require.NotContains(t, fields, "durations_seconds")
	require.Nil(t, video.ImageCapabilities)
}

func TestGetNextChatWorkspaceModelsFailsClosedForVideoWithoutGroupPrice(t *testing.T) {
	groupID := int64(2_608_264)
	svc := newNextChatWorkspaceMediaCatalogService(
		Group{ID: groupID, Name: "Grok Heavy", Platform: PlatformGrok, AllowImageGeneration: true},
		[]SiteModelCatalogEntry{{
			ModelName: "grok-imagine-video", Platform: PlatformGrok, VisibleAuth: true, GroupIDs: []int64{groupID},
			MediaCapabilities: json.RawMessage(`{
				"version":"2026-08-26.1", "adapter":"grok_video", "modalities":["video"],
				"video":{"operations":["generate"],"supported_resolutions":["720p"],"supported_aspect_ratios":["16:9"],"durations_seconds":[6]}
			}`),
		}},
		[]string{"grok-imagine-video"},
	)

	models, err := svc.GetNextChatWorkspaceModels(context.Background(), 42, 2)

	require.NoError(t, err)
	require.Len(t, models.Groups, 1)
	require.False(t, models.Groups[0].VideoAvailable)
	require.Len(t, models.Groups[0].Models, 1)
	require.Empty(t, models.Groups[0].Models[0].Modalities)
	require.Nil(t, models.Groups[0].Models[0].VideoCapabilities)
}

func TestGetNextChatWorkspaceModelsFailsClosedForVideoWhenSchedulerCapabilitiesAreUnavailable(t *testing.T) {
	groupID := int64(2_608_265)
	price := 0.14
	group := Group{
		ID: groupID, Name: "OpenAI media", Platform: PlatformOpenAI,
		AllowImageGeneration: true, VideoPrice720P: &price, Status: StatusActive,
	}
	apiKeyService := NewAPIKeyService(
		&nextChatAPIKeyRepoStub{keys: []APIKey{
			{ID: 1, UserID: 42, Name: "workspace group key", Key: "sk-workspace-group", Status: StatusActive, GroupID: &groupID, Group: &group},
			{ID: 2, UserID: 42, Name: NextChatManagedAPIKeyName, Key: "sk-managed-workspace", Status: StatusActive, GroupID: &groupID},
		}},
		&nextChatUserRepoStub{user: &User{ID: 42, Status: StatusActive}},
		&nextChatGroupRepoStub{groups: []Group{group}},
		&nextChatSubscriptionRepoStub{}, nil, nil, &config.Config{},
	)
	svc := NewModelCatalogService(
		&nextChatModelCatalogRepoStub{entries: []SiteModelCatalogEntry{{
			ModelName: "agnes-video-v2.0", Platform: PlatformOpenAI, VisibleAuth: true,
			GroupIDs: []int64{groupID}, MediaCapabilities: json.RawMessage(`{
				"version":"2026-08-26.1", "adapter":"agnes_video", "modalities":["video"],
				"video":{"operations":["generate"],"supported_resolutions":["720p"],"supported_aspect_ratios":["16:9"],"durations_seconds":[6]}
			}`),
		}}},
		nil, nil, nil, nil, apiKeyService,
		&nextChatAvailableModelsOnlyStub{modelsByGroup: map[int64][]string{
			groupID: {"gpt-image-1", "agnes-video-v2.0"},
		}},
	)

	models, err := svc.GetNextChatWorkspaceModels(context.Background(), 42, 2)

	require.NoError(t, err)
	require.Len(t, models.Groups, 1)
	require.False(t, models.Groups[0].VideoAvailable)
	byName := make(map[string]NextChatWorkspaceModel, len(models.Groups[0].Models))
	for _, model := range models.Groups[0].Models {
		byName[model.Name] = model
	}
	require.Empty(t, byName["agnes-video-v2.0"].Modalities)
	require.Nil(t, byName["agnes-video-v2.0"].VideoCapabilities)
	require.Equal(t, []string{"image"}, byName["gpt-image-1"].Modalities)
	require.NotNil(t, byName["gpt-image-1"].ImageCapabilities)
}

func TestGetNextChatWorkspaceModelsFailsClosedForCompositeVideoWithoutAdapterPlatform(t *testing.T) {
	groupID := int64(2_608_266)
	price := 0.14
	group := Group{
		ID: groupID, Name: "Composite media", Platform: PlatformComposite,
		AllowImageGeneration: true, VideoPrice720P: &price, Status: StatusActive,
	}
	apiKeyService := NewAPIKeyService(
		&nextChatAPIKeyRepoStub{keys: []APIKey{
			{ID: 1, UserID: 42, Name: "workspace group key", Key: "sk-workspace-group", Status: StatusActive, GroupID: &groupID, Group: &group},
			{ID: 2, UserID: 42, Name: NextChatManagedAPIKeyName, Key: "sk-managed-workspace", Status: StatusActive, GroupID: &groupID},
		}},
		&nextChatUserRepoStub{user: &User{ID: 42, Status: StatusActive}},
		&nextChatGroupRepoStub{groups: []Group{group}},
		&nextChatSubscriptionRepoStub{}, nil, nil, &config.Config{},
	)
	videoDeclaration := json.RawMessage(`{
		"version":"2026-08-26.1", "adapter":"grok_video", "modalities":["video"],
		"video":{"operations":["generate"],"supported_resolutions":["720p"],"supported_aspect_ratios":["16:9"],"durations_seconds":[6]}
	}`)
	resolver := &nextChatAvailableModelResolverStub{
		// The legacy workspace lookup can see this stale composite mapping.
		modelsByGroupAndPlatform: map[int64]map[string][]string{groupID: {
			PlatformComposite: {"grok-imagine-video"},
			PlatformGrok:      {"grok-imagine-video"},
		}},
		// There is no schedulable Grok account. The canonical mobile video
		// resolver must therefore suppress the mapping despite the group price.
		schedulablePlatformsByGroupID: map[int64]map[string]struct{}{groupID: {
			PlatformOpenAI: {},
		}},
	}
	svc := NewModelCatalogService(
		&nextChatModelCatalogRepoStub{entries: []SiteModelCatalogEntry{{
			// A generic catalog entry is valid for a composite group, but it
			// still needs a schedulable account for the declared adapter platform.
			ModelName: "grok-imagine-video", VisibleAuth: true,
			GroupIDs: []int64{groupID}, MediaCapabilities: videoDeclaration,
		}}},
		nil, nil, nil, nil, apiKeyService, resolver,
	)

	models, err := svc.GetNextChatWorkspaceModels(context.Background(), 42, 2)

	require.NoError(t, err)
	require.Len(t, models.Groups, 1)
	// A stale aggregate mapping must not make an unavailable concrete provider
	// appear in a managed workspace. Only schedulable concrete platforms are
	// eligible for the composite model list.
	require.Empty(t, models.Groups[0].Models)
	require.False(t, models.Groups[0].VideoAvailable)
}

func TestGetNextChatWorkspaceModelsResolvesCompositeModelsByConcretePlatform(t *testing.T) {
	groupID := int64(2_608_267)
	price := 0.14
	imagePrice := 0.01
	group := Group{
		ID: groupID, Name: "Composite media", Platform: PlatformComposite,
		AllowImageGeneration: true, ImagePrice1K: &imagePrice, Status: StatusActive,
		VideoModelPrices: map[string]map[string]float64{
			"grok-imagine-video": {"720p": price},
		},
	}
	apiKeyService := NewAPIKeyService(
		&nextChatAPIKeyRepoStub{keys: []APIKey{
			{ID: 1, UserID: 42, Name: "workspace group key", Key: "sk-workspace-group", Status: StatusActive, GroupID: &groupID, Group: &group},
			{ID: 2, UserID: 42, Name: NextChatManagedAPIKeyName, Key: "sk-managed-workspace", Status: StatusActive, GroupID: &groupID},
		}},
		&nextChatUserRepoStub{user: &User{ID: 42, Status: StatusActive}},
		&nextChatGroupRepoStub{groups: []Group{group}},
		&nextChatSubscriptionRepoStub{}, nil, nil, &config.Config{},
	)
	fastDeclaration := json.RawMessage(`{
		"version":"2026-08-26.1", "adapter":"sensenova", "modalities":["image"],
		"image":{"operations":["create"],"sizing_kind":"fixed",
		"supported_sizes":["1664x2496","2496x1664","1760x2368","2368x1760","1824x2272","2272x1824","2048x2048","2752x1536","1536x2752","3072x1376","1344x3136"],
		"max_reference_images":0}
	}`)
	videoDeclaration := json.RawMessage(`{
		"version":"2026-08-26.1", "adapter":"grok_video", "modalities":["video"],
		"video":{"operations":["generate"],"supported_resolutions":["720p"],
		"supported_aspect_ratios":["16:9"],"durations_seconds":[8]}
	}`)
	resolver := &nextChatAvailableModelResolverStub{
		modelsByGroupAndPlatform: map[int64]map[string][]string{groupID: {
			PlatformComposite: {"stale-composite-model"},
			PlatformOpenAI:    {"sensenova-u1-fast"},
			PlatformGrok:      {"grok-imagine-video"},
			"new-provider":    {"new-provider-chat"},
		}},
		schedulablePlatformsByGroupID: map[int64]map[string]struct{}{groupID: {
			PlatformOpenAI: {},
			PlatformGrok:   {},
			"new-provider": {},
		}},
	}
	svc := NewModelCatalogService(
		&nextChatModelCatalogRepoStub{entries: []SiteModelCatalogEntry{
			{ModelName: "sensenova-u1-fast", Platform: PlatformOpenAI, VisibleAuth: true, GroupIDs: []int64{groupID}, MediaCapabilities: fastDeclaration},
			{ModelName: "grok-imagine-video", Platform: PlatformGrok, VisibleAuth: true, GroupIDs: []int64{groupID}, MediaCapabilities: videoDeclaration},
		}},
		nil, nil, nil, nil, apiKeyService, resolver,
	)

	models, err := svc.GetNextChatWorkspaceModels(context.Background(), 42, 2)

	require.NoError(t, err)
	require.Len(t, models.Groups, 1)
	require.True(t, models.Groups[0].VideoAvailable)
	byName := make(map[string]NextChatWorkspaceModel, len(models.Groups[0].Models))
	for _, model := range models.Groups[0].Models {
		byName[model.Name] = model
	}
	require.NotContains(t, byName, "stale-composite-model")

	fast := byName["sensenova-u1-fast"]
	require.Equal(t, PlatformOpenAI, fast.Platform)
	require.Equal(t, []string{"image"}, fast.Modalities)
	require.Equal(t, "sensenova", fast.Adapter)
	require.NotNil(t, fast.ImageCapabilities)
	require.Nil(t, fast.VideoCapabilities)

	newProvider := byName["new-provider-chat"]
	require.Equal(t, "new-provider", newProvider.Platform)
	require.Empty(t, newProvider.Modalities)

	video := byName["grok-imagine-video"]
	require.Equal(t, PlatformGrok, video.Platform)
	require.Equal(t, []string{"video"}, video.Modalities)
	require.Equal(t, MobileVideoAdapterGrok, video.Adapter)
	require.Nil(t, video.ImageCapabilities)
	require.NotNil(t, video.VideoCapabilities)
	require.Equal(t, []string{"720p"}, video.VideoCapabilities.SupportedResolutions)
}

func TestGetNextChatWorkspaceModelsUsesMobileVideoSelectedContractForDuplicateCompositeModel(t *testing.T) {
	groupID := int64(2_608_271)
	price := 0.14
	group := Group{
		ID: groupID, Name: "Composite duplicate video", Platform: PlatformComposite,
		Status: StatusActive,
		VideoModelPrices: map[string]map[string]float64{
			"shared-video": {"480p": price, "720p": price},
		},
	}
	apiKeyService := NewAPIKeyService(
		&nextChatAPIKeyRepoStub{keys: []APIKey{
			{ID: 1, UserID: 42, Name: "workspace group key", Key: "sk-workspace-group", Status: StatusActive, GroupID: &groupID, Group: &group},
			{ID: 2, UserID: 42, Name: NextChatManagedAPIKeyName, Key: "sk-managed-workspace", Status: StatusActive, GroupID: &groupID},
		}},
		&nextChatUserRepoStub{user: &User{ID: 42, Status: StatusActive}},
		&nextChatGroupRepoStub{groups: []Group{group}},
		&nextChatSubscriptionRepoStub{}, nil, nil, &config.Config{},
	)
	// OpenAI has higher ordinary composite display precedence, while the video
	// resolver's deterministic executable selection chooses Grok. This is the
	// exact situation that used to splice the OpenAI adapter onto Grok limits.
	agnesDeclaration := json.RawMessage(`{
		"version":"openai-v1", "adapter":"agnes_video", "modalities":["video"],
		"video":{"operations":["generate"],"supported_resolutions":["480p"],"supported_aspect_ratios":["16:9"],"durations_seconds":[3]}
	}`)
	grokDeclaration := json.RawMessage(`{
		"version":"grok-v1", "adapter":"grok_video", "modalities":["video"],
		"video":{"operations":["generate"],"supported_resolutions":["720p"],"supported_aspect_ratios":["16:9"],"durations_seconds":[8]}
	}`)
	resolver := &nextChatAvailableModelResolverStub{
		modelsByGroupAndPlatform: map[int64]map[string][]string{groupID: {
			PlatformOpenAI: {"shared-video"},
			PlatformGrok:   {"shared-video"},
		}},
		schedulablePlatformsByGroupID: map[int64]map[string]struct{}{groupID: {
			PlatformOpenAI: {},
			PlatformGrok:   {},
		}},
	}
	svc := NewModelCatalogService(
		&nextChatModelCatalogRepoStub{entries: []SiteModelCatalogEntry{
			{ModelName: "shared-video", Platform: PlatformOpenAI, VisibleAuth: true, GroupIDs: []int64{groupID}, MediaCapabilities: agnesDeclaration},
			{ModelName: "shared-video", Platform: PlatformGrok, VisibleAuth: true, GroupIDs: []int64{groupID}, MediaCapabilities: grokDeclaration},
		}},
		nil, nil, nil, nil, apiKeyService, resolver,
	)

	workspace, err := svc.GetNextChatWorkspaceModels(context.Background(), 42, 2)
	require.NoError(t, err)
	require.Len(t, workspace.Groups, 1)
	require.Len(t, workspace.Groups[0].Models, 1)
	model := workspace.Groups[0].Models[0]
	require.True(t, workspace.Groups[0].VideoAvailable)
	require.Equal(t, PlatformGrok, model.Platform)
	require.Equal(t, MobileVideoAdapterGrok, model.Adapter)
	require.Equal(t, MobileVideoCapabilitiesVersion, model.CapabilityVersion)
	require.Equal(t, []string{"video"}, model.Modalities)
	require.NotNil(t, model.VideoCapabilities)
	require.Equal(t, []string{"720p"}, model.VideoCapabilities.SupportedResolutions)
	require.Equal(t, []string{"16:9"}, model.VideoCapabilities.SupportedRatios)
	require.Equal(t, []int{8}, model.VideoCapabilities.SupportedDurations)
}

func TestNextChatWorkspaceModelMetadataDoesNotBorrowBillingDataAcrossGroups(t *testing.T) {
	openAIGroupID := int64(7)
	grokGroupID := int64(8)
	borrowedInputPrice := 0.42
	metadata := buildNextChatWorkspaceModelMetadata([]NextChatDisplayModel{
		{
			Name:                "gpt-4o-mini",
			Platform:            PlatformOpenAI,
			Channel:             "billing-channel-from-other-group",
			EffectiveInputPrice: &borrowedInputPrice,
			SortOrder:           9,
			Groups:              []NextChatDisplayGroup{{ID: openAIGroupID, Name: "OpenAI main"}},
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

func TestBuildNextChatPromptCatalogFromPublicPromptsSkipsImagePrompts(t *testing.T) {
	catalog := BuildNextChatPromptCatalogFromPublicPrompts([]PublicPrompt{
		{
			ID:         89,
			Title:      "白底商品主图",
			Purpose:    "image",
			Version:    1,
			PromptText: "生成白底商品主图。",
			Models:     []string{"gpt-image-2"},
			Sizes:      []string{"1024x1024"},
		},
	})

	titles := make([]string, 0, len(catalog.ChatPrompts))
	for _, prompt := range catalog.ChatPrompts {
		titles = append(titles, prompt.Title)
	}
	require.NotContains(t, titles, "白底商品主图")
	require.NotEmpty(t, catalog.ImageTemplates.Intents)
}

func collectNextChatModelNames(models []NextChatWorkspaceModel) []string {
	names := make([]string, 0, len(models))
	for _, model := range models {
		names = append(names, model.Name)
	}
	return names
}

func TestBuildNextChatWorkspaceModelPublishesServerImageCapabilities(t *testing.T) {
	editModel := buildNextChatWorkspaceModel(PlatformOpenAI, "gpt-image-1", nextChatWorkspaceModelMeta{})
	require.NotNil(t, editModel.ImageCapabilities)
	require.Contains(t, editModel.ImageCapabilities.Operations, "edit")
	require.Greater(t, editModel.ImageCapabilities.MaxReferenceImages, 0)

	privateModel := buildNextChatWorkspaceModel(PlatformOpenAI, "agnes-image-2.1-flash", nextChatWorkspaceModelMeta{})
	require.NotNil(t, privateModel.ImageCapabilities)
	require.NotContains(t, privateModel.ImageCapabilities.Operations, "edit")
	require.Zero(t, privateModel.ImageCapabilities.MaxReferenceImages)
}

func TestResolveNextChatModelToolCapabilitiesPrefersExplicitCatalogDeclaration(t *testing.T) {
	allow := true
	disallow := false
	upstream := ModelToolCapabilities{
		FunctionCalling: true,
		ToolChoice:      true,
		WebSearch:       true,
	}

	resolved := resolveNextChatModelToolCapabilities(
		ModelToolCapabilityOverrides{
			FunctionCalling: &disallow,
			ToolChoice:      &allow,
			WebSearch:       &disallow,
			Live:            &allow,
		},
		upstream,
	)

	require.Equal(t, ModelToolCapabilities{
		FunctionCalling: false,
		ToolChoice:      true,
		WebSearch:       false,
		Live:            true,
	}, resolved)
}

func TestResolveNextChatModelToolCapabilitiesFallsBackOnlyToExactUpstreamMetadata(t *testing.T) {
	allow := true
	upstream := ModelToolCapabilities{FunctionCalling: true}

	// A catalog row may deliberately leave individual fields unset. Those
	// fields may use exact LiteLLM metadata, but never a model-name fallback.
	resolved := resolveNextChatModelToolCapabilities(
		ModelToolCapabilityOverrides{WebSearch: &allow},
		upstream,
	)
	require.Equal(t, ModelToolCapabilities{
		FunctionCalling: true,
		ToolChoice:      false,
		WebSearch:       true,
	}, resolved)

	require.Equal(t, ModelToolCapabilities{}, resolveNextChatModelToolCapabilities(
		ModelToolCapabilityOverrides{},
		ModelToolCapabilities{},
	))
}

func TestResolveNextChatModelToolCapabilitiesEnablesPlatformSearchForFunctionCapableModels(t *testing.T) {
	// A provider's native web-search flag is not the contract for the
	// platform-owned Exa/DuckDuckGo function tool. Any model with verified
	// function calling can request the platform tool unless an administrator
	// explicitly denies it in the catalog.
	resolved := resolveNextChatModelToolCapabilities(
		ModelToolCapabilityOverrides{},
		ModelToolCapabilities{
			FunctionCalling: true,
			ToolChoice:      true,
			WebSearch:       false,
		},
	)
	require.Equal(t, ModelToolCapabilities{
		FunctionCalling: true,
		ToolChoice:      true,
		WebSearch:       true,
	}, resolved)

	disallow := false
	denied := resolveNextChatModelToolCapabilities(
		ModelToolCapabilityOverrides{WebSearch: &disallow},
		ModelToolCapabilities{FunctionCalling: true, ToolChoice: true},
	)
	require.False(t, denied.WebSearch)
}

func TestResolveNextChatWorkspaceModelToolCapabilitiesHonorsCatalogAliasAndGroupScope(t *testing.T) {
	allow := true
	group := Group{ID: 42, Platform: PlatformOpenAI}
	entries := []SiteModelCatalogEntry{
		{
			ModelName:   "private-search-alias",
			Platform:    PlatformOpenAI,
			VisibleAuth: true,
			GroupIDs:    []int64{group.ID},
			ToolCapabilities: ModelToolCapabilityOverrides{
				FunctionCalling: &allow,
				ToolChoice:      &allow,
				WebSearch:       &allow,
				Live:            &allow,
			},
		},
	}

	// The private alias is not present in LiteLLM metadata. The explicit,
	// group-scoped catalog declaration is therefore its sole authority.
	require.Equal(t, ModelToolCapabilities{
		FunctionCalling: true,
		ToolChoice:      true,
		WebSearch:       true,
		Live:            true,
	}, resolveNextChatWorkspaceModelToolCapabilities(entries, group, "private-search-alias", ModelToolCapabilities{}))

	otherGroup := Group{ID: 99, Platform: PlatformOpenAI}
	require.Equal(t, ModelToolCapabilities{}, resolveNextChatWorkspaceModelToolCapabilities(entries, otherGroup, "private-search-alias", ModelToolCapabilities{}))

	// A model does not need a duplicated catalog row merely to receive the
	// platform-owned search tool. Exact server metadata already verifies its
	// function capability; the resolver must apply the same web-search default
	// as it does for a catalog entry with no explicit override.
	require.Equal(t, ModelToolCapabilities{
		FunctionCalling: true,
		ToolChoice:      true,
		WebSearch:       true,
	}, resolveNextChatWorkspaceModelToolCapabilities(
		nil,
		group,
		"verified-function-model",
		ModelToolCapabilities{FunctionCalling: true, ToolChoice: true},
	))
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
