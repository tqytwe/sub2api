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

func TestIssueNextChatManagedSessionRealignsReusableManagedKeyGroup(t *testing.T) {
	oldGroupID := int64(2)
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
			GroupID: &oldGroupID,
			Group:   &Group{ID: oldGroupID, Platform: PlatformOpenAI, Status: StatusActive, SortOrder: 1},
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
