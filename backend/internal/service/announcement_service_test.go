package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type announcementRepoStub struct {
	item *Announcement
}

func (s *announcementRepoStub) Create(_ context.Context, a *Announcement) error {
	s.item = a
	return nil
}

func (s *announcementRepoStub) GetByID(_ context.Context, _ int64) (*Announcement, error) {
	if s.item == nil {
		return nil, ErrAnnouncementNotFound
	}
	return s.item, nil
}

func (s *announcementRepoStub) Update(_ context.Context, a *Announcement) error {
	s.item = a
	return nil
}

func (*announcementRepoStub) Delete(context.Context, int64) error {
	return nil
}

func (*announcementRepoStub) List(context.Context, pagination.PaginationParams, AnnouncementListFilters) ([]Announcement, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (*announcementRepoStub) ListActive(context.Context, time.Time) ([]Announcement, error) {
	return nil, nil
}

type announcementVisibilityRepoStub struct {
	items []Announcement
}

func (*announcementVisibilityRepoStub) Create(context.Context, *Announcement) error { return nil }
func (s *announcementVisibilityRepoStub) GetByID(_ context.Context, id int64) (*Announcement, error) {
	for i := range s.items {
		if s.items[i].ID == id {
			item := s.items[i]
			return &item, nil
		}
	}
	return nil, ErrAnnouncementNotFound
}
func (*announcementVisibilityRepoStub) Update(context.Context, *Announcement) error { return nil }
func (*announcementVisibilityRepoStub) Delete(context.Context, int64) error         { return nil }
func (*announcementVisibilityRepoStub) List(context.Context, pagination.PaginationParams, AnnouncementListFilters) ([]Announcement, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (s *announcementVisibilityRepoStub) ListActive(context.Context, time.Time) ([]Announcement, error) {
	return append([]Announcement(nil), s.items...), nil
}

type announcementVisibilityUserRepo struct {
	UserRepository
	users map[int64]*User
}

func (s *announcementVisibilityUserRepo) GetByID(_ context.Context, id int64) (*User, error) {
	user, ok := s.users[id]
	if !ok {
		return nil, ErrUserNotFound
	}
	copy := *user
	return &copy, nil
}
func (s *announcementVisibilityUserRepo) ListWithFilters(context.Context, pagination.PaginationParams, UserListFilters) ([]User, *pagination.PaginationResult, error) {
	users := make([]User, 0, len(s.users))
	for _, user := range s.users {
		users = append(users, *user)
	}
	return users, nil, nil
}

type announcementVisibilitySubscriptionRepo struct{ UserSubscriptionRepository }

func (*announcementVisibilitySubscriptionRepo) ListActiveByUserID(context.Context, int64) ([]UserSubscription, error) {
	return nil, nil
}

type announcementVisibilityReadRepo struct {
	AnnouncementReadRepository
	marked []int64
}

func (s *announcementVisibilityReadRepo) MarkRead(_ context.Context, announcementID, _ int64, _ time.Time) error {
	s.marked = append(s.marked, announcementID)
	return nil
}
func (*announcementVisibilityReadRepo) GetReadMapByUser(context.Context, int64, []int64) (map[int64]time.Time, error) {
	return map[int64]time.Time{}, nil
}
func (*announcementVisibilityReadRepo) GetReadMapByUsers(context.Context, int64, []int64) (map[int64]time.Time, error) {
	return map[int64]time.Time{}, nil
}
func (*announcementVisibilityReadRepo) CountByAnnouncementID(context.Context, int64) (int64, error) {
	return 0, nil
}

func TestAnnouncementServiceCreateRejectsEqualStartEndTimes(t *testing.T) {
	repo := &announcementRepoStub{}
	svc := NewAnnouncementService(repo, nil, nil, nil)
	now := time.Unix(1776790020, 0)

	_, err := svc.Create(context.Background(), &CreateAnnouncementInput{
		Title:      "公告",
		Content:    "内容",
		Status:     AnnouncementStatusActive,
		NotifyMode: AnnouncementNotifyModePopup,
		StartsAt:   &now,
		EndsAt:     &now,
	})
	require.ErrorIs(t, err, ErrAnnouncementInvalidSchedule)
}

func TestAnnouncementServiceCreateAcceptsPlatformHostedAnnouncementImage(t *testing.T) {
	repo := &announcementRepoStub{}
	svc := NewAnnouncementService(repo, nil, nil, nil)

	created, err := svc.Create(context.Background(), &CreateAnnouncementInput{
		Title:   "公告",
		Content: "请看图：![海报](/api/v1/announcement-assets/announcements/banner.png)\n\n==重点==",
		Status:  AnnouncementStatusDraft,
	})

	require.NoError(t, err)
	require.Contains(t, created.Content, "/api/v1/announcement-assets/announcements/banner.png")
}

func TestAnnouncementServiceCreateRejectsUnsafeMarkdownImagesAndHTML(t *testing.T) {
	svc := NewAnnouncementService(&announcementRepoStub{}, nil, nil, nil)

	for _, content := range []string{
		`![external](https://example.com/banner.png)`,
		`![external-route](https://evil.example/api/v1/announcement-assets/announcements/banner.png)`,
		`![inline](data:image/png;base64,AAAA)`,
		`<span style="color:red">red</span>`,
	} {
		_, err := svc.Create(context.Background(), &CreateAnnouncementInput{
			Title:   "公告",
			Content: content,
			Status:  AnnouncementStatusDraft,
		})
		require.ErrorIs(t, err, ErrAnnouncementContentUnsafe)
	}
}

func TestAnnouncementServiceUpdateRejectsEqualStartEndTimes(t *testing.T) {
	repo := &announcementRepoStub{
		item: &Announcement{
			ID:         1,
			Title:      "公告",
			Content:    "内容",
			Status:     AnnouncementStatusActive,
			NotifyMode: AnnouncementNotifyModePopup,
		},
	}
	svc := NewAnnouncementService(repo, nil, nil, nil)
	now := time.Unix(1776790020, 0)
	startsAt := &now
	endsAt := &now

	_, err := svc.Update(context.Background(), 1, &UpdateAnnouncementInput{
		StartsAt: &startsAt,
		EndsAt:   &endsAt,
	})
	require.ErrorIs(t, err, ErrAnnouncementInvalidSchedule)
}

func TestAnnouncementServicePlayMembershipTargetingIsRealtimeAcrossReadPaths(t *testing.T) {
	announcement := Announcement{
		ID:         77,
		Title:      "仅普通用户",
		Content:    "充值后可解锁 VIP",
		Status:     AnnouncementStatusActive,
		NotifyMode: AnnouncementNotifyModePopup,
		Targeting: AnnouncementTargeting{AnyOf: []AnnouncementConditionGroup{{
			AllOf: []AnnouncementCondition{{
				Type:           AnnouncementConditionTypePlayMembership,
				Operator:       AnnouncementOperatorIn,
				PlayMembership: AnnouncementPlayMembershipOrdinary,
			}},
		}}},
	}
	readRepo := &announcementVisibilityReadRepo{}
	service := NewAnnouncementService(
		&announcementVisibilityRepoStub{items: []Announcement{announcement}},
		readRepo,
		&announcementVisibilityUserRepo{users: map[int64]*User{9: {ID: 9, Email: "user@example.com", Username: "user"}}},
		&announcementVisibilitySubscriptionRepo{},
	)
	membership := AnnouncementPlayMembershipOrdinary
	service.SetPlayMembershipResolver(func(context.Context, int64) (string, error) {
		return membership, nil
	})

	visible, err := service.ListForUser(context.Background(), 9, false)
	require.NoError(t, err)
	require.Len(t, visible, 1)
	require.NoError(t, service.MarkRead(context.Background(), 9, announcement.ID))
	require.Equal(t, []int64{announcement.ID}, readRepo.marked)

	// The resolver represents the existing cumulative Play qualification; no
	// announcement record is changed when the user reaches the first VIP tier.
	membership = AnnouncementPlayMembershipMember
	visible, err = service.ListForUser(context.Background(), 9, false)
	require.NoError(t, err)
	require.Empty(t, visible)
	require.ErrorIs(t, service.MarkRead(context.Background(), 9, announcement.ID), ErrAnnouncementNotFound)
	require.Equal(t, []int64{announcement.ID}, readRepo.marked)

	statuses, _, err := service.ListUserReadStatus(context.Background(), announcement.ID, pagination.PaginationParams{}, "")
	require.NoError(t, err)
	require.Len(t, statuses, 1)
	require.False(t, statuses[0].Eligible)

	membership = AnnouncementPlayMembershipOrdinary
	statuses, _, err = service.ListUserReadStatus(context.Background(), announcement.ID, pagination.PaginationParams{}, "")
	require.NoError(t, err)
	require.True(t, statuses[0].Eligible)
}
