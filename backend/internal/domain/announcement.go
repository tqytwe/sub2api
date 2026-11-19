package domain

import (
	"slices"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	AnnouncementStatusDraft    = "draft"
	AnnouncementStatusActive   = "active"
	AnnouncementStatusArchived = "archived"
)

const (
	AnnouncementNotifyModeSilent = "silent"
	AnnouncementNotifyModePopup  = "popup"
)

const (
	AnnouncementConditionTypeSubscription   = "subscription"
	AnnouncementConditionTypeBalance        = "balance"
	AnnouncementConditionTypeUser           = "user"
	AnnouncementConditionTypePlayMembership = "play_membership"
)

// MaxAnnouncementConditionUserIDs bounds a single explicit-user condition.
const MaxAnnouncementConditionUserIDs = 1000

const (
	AnnouncementPlayMembershipOrdinary = "ordinary"
	AnnouncementPlayMembershipMember   = "member"
)

const (
	AnnouncementOperatorIn  = "in"
	AnnouncementOperatorGT  = "gt"
	AnnouncementOperatorGTE = "gte"
	AnnouncementOperatorLT  = "lt"
	AnnouncementOperatorLTE = "lte"
	AnnouncementOperatorEQ  = "eq"
)

var (
	ErrAnnouncementNotFound      = infraerrors.NotFound("ANNOUNCEMENT_NOT_FOUND", "announcement not found")
	ErrAnnouncementInvalidTarget = infraerrors.BadRequest("ANNOUNCEMENT_INVALID_TARGET", "invalid announcement targeting rules")
)

type AnnouncementTargeting struct {
	// AnyOf 表示 OR：任意一个条件组满足即可展示。
	AnyOf []AnnouncementConditionGroup `json:"any_of,omitempty"`
}

type AnnouncementConditionGroup struct {
	// AllOf 表示 AND：组内所有条件都满足才算命中该组。
	AllOf []AnnouncementCondition `json:"all_of,omitempty"`
}

type AnnouncementCondition struct {
	// Type: subscription | balance | play_membership | user
	Type string `json:"type"`

	// Operator:
	// - subscription, play_membership, user: in
	// - balance: gt/gte/lt/lte/eq
	Operator string `json:"operator"`

	// subscription 条件：匹配的订阅套餐（group_id）
	GroupIDs []int64 `json:"group_ids,omitempty"`

	// user 条件：指定可见的用户（user_id）。
	UserIDs []int64 `json:"user_ids,omitempty"`

	// balance 条件：比较阈值
	Value float64 `json:"value,omitempty"`

	// play_membership 条件：当前 Play 会员资格，必须是 ordinary 或 member。
	PlayMembership string `json:"play_membership,omitempty"`
}

func (t AnnouncementTargeting) Matches(balance float64, activeSubscriptionGroupIDs map[int64]struct{}) bool {
	return t.MatchesForUser(0, balance, activeSubscriptionGroupIDs, "")
}

// MatchesWithPlayMembership preserves the original balance/subscription
// contract while adding a server-derived, real-time Play membership segment.
func (t AnnouncementTargeting) MatchesWithPlayMembership(balance float64, activeSubscriptionGroupIDs map[int64]struct{}, playMembership string) bool {
	return t.MatchesForUser(0, balance, activeSubscriptionGroupIDs, playMembership)
}

// MatchesForUser evaluates the complete audience using the authenticated user ID.
func (t AnnouncementTargeting) MatchesForUser(userID int64, balance float64, activeSubscriptionGroupIDs map[int64]struct{}, playMembership string) bool {
	// 空规则：展示给所有用户
	if len(t.AnyOf) == 0 {
		return true
	}

	for _, group := range t.AnyOf {
		if len(group.AllOf) == 0 {
			// 空条件组不命中（避免 OR 中出现无条件 “全命中”）
			continue
		}
		allMatched := true
		for _, cond := range group.AllOf {
			if !cond.matchesForUser(userID, balance, activeSubscriptionGroupIDs, playMembership) {
				allMatched = false
				break
			}
		}
		if allMatched {
			return true
		}
	}

	return false
}

func (c AnnouncementCondition) Matches(balance float64, activeSubscriptionGroupIDs map[int64]struct{}) bool {
	return c.MatchesWithPlayMembership(balance, activeSubscriptionGroupIDs, "")
}

func (c AnnouncementCondition) MatchesWithPlayMembership(balance float64, activeSubscriptionGroupIDs map[int64]struct{}, playMembership string) bool {
	return c.matchesForUser(0, balance, activeSubscriptionGroupIDs, playMembership)
}

func (c AnnouncementCondition) matchesForUser(userID int64, balance float64, activeSubscriptionGroupIDs map[int64]struct{}, playMembership string) bool {
	switch c.Type {
	case AnnouncementConditionTypeSubscription:
		if c.Operator != AnnouncementOperatorIn {
			return false
		}
		if len(c.GroupIDs) == 0 {
			return false
		}
		if len(activeSubscriptionGroupIDs) == 0 {
			return false
		}
		for _, gid := range c.GroupIDs {
			if _, ok := activeSubscriptionGroupIDs[gid]; ok {
				return true
			}
		}
		return false

	case AnnouncementConditionTypeUser:
		return c.Operator == AnnouncementOperatorIn && userID > 0 && slices.Contains(c.UserIDs, userID)

	case AnnouncementConditionTypeBalance:
		switch c.Operator {
		case AnnouncementOperatorGT:
			return balance > c.Value
		case AnnouncementOperatorGTE:
			return balance >= c.Value
		case AnnouncementOperatorLT:
			return balance < c.Value
		case AnnouncementOperatorLTE:
			return balance <= c.Value
		case AnnouncementOperatorEQ:
			return balance == c.Value
		default:
			return false
		}

	case AnnouncementConditionTypePlayMembership:
		return c.Operator == AnnouncementOperatorIn && c.PlayMembership == playMembership

	default:
		return false
	}
}

func (t AnnouncementTargeting) NormalizeAndValidate() (AnnouncementTargeting, error) {
	normalized := AnnouncementTargeting{AnyOf: make([]AnnouncementConditionGroup, 0, len(t.AnyOf))}

	// 允许空 targeting（展示给所有用户）
	if len(t.AnyOf) == 0 {
		return normalized, nil
	}

	if len(t.AnyOf) > 50 {
		return AnnouncementTargeting{}, ErrAnnouncementInvalidTarget
	}

	for _, g := range t.AnyOf {
		if len(g.AllOf) == 0 {
			return AnnouncementTargeting{}, ErrAnnouncementInvalidTarget
		}
		if len(g.AllOf) > 50 {
			return AnnouncementTargeting{}, ErrAnnouncementInvalidTarget
		}

		group := AnnouncementConditionGroup{AllOf: make([]AnnouncementCondition, 0, len(g.AllOf))}
		for _, c := range g.AllOf {
			cond := AnnouncementCondition{
				Type:           strings.TrimSpace(c.Type),
				Operator:       strings.TrimSpace(c.Operator),
				Value:          c.Value,
				PlayMembership: strings.TrimSpace(c.PlayMembership),
			}
			for _, gid := range c.GroupIDs {
				if gid <= 0 {
					return AnnouncementTargeting{}, ErrAnnouncementInvalidTarget
				}
				cond.GroupIDs = append(cond.GroupIDs, gid)
			}

			if cond.Type == AnnouncementConditionTypeUser {
				ids, err := normalizeAnnouncementUserIDs(c.UserIDs)
				if err != nil {
					return AnnouncementTargeting{}, err
				}
				cond.UserIDs = ids
			}

			if err := cond.validate(); err != nil {
				return AnnouncementTargeting{}, err
			}
			group.AllOf = append(group.AllOf, cond)
		}

		normalized.AnyOf = append(normalized.AnyOf, group)
	}

	return normalized, nil
}

func (t AnnouncementTargeting) ExplicitUserIDs() ([]int64, bool) {
	if len(t.AnyOf) == 0 {
		return nil, false
	}
	var ids []int64
	seen := make(map[int64]struct{})
	for _, group := range t.AnyOf {
		restricted := false
		for _, cond := range group.AllOf {
			if cond.Type != AnnouncementConditionTypeUser || cond.Operator != AnnouncementOperatorIn {
				continue
			}
			restricted = true
			for _, id := range cond.UserIDs {
				if _, ok := seen[id]; !ok {
					seen[id] = struct{}{}
					ids = append(ids, id)
				}
			}
		}
		if !restricted {
			return nil, false
		}
	}
	return ids, true
}

// normalizeAnnouncementUserIDs 校验并按原顺序去重「指定用户」条件里的用户 ID。
func normalizeAnnouncementUserIDs(userIDs []int64) ([]int64, error) {
	out := make([]int64, 0, len(userIDs))
	seen := make(map[int64]struct{}, len(userIDs))
	for _, id := range userIDs {
		if id <= 0 {
			return nil, ErrAnnouncementInvalidTarget
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if len(out) > MaxAnnouncementConditionUserIDs {
		return nil, ErrAnnouncementInvalidTarget
	}
	return out, nil
}

func (c AnnouncementCondition) validate() error {
	switch c.Type {
	case AnnouncementConditionTypeSubscription:
		if c.Operator != AnnouncementOperatorIn {
			return ErrAnnouncementInvalidTarget
		}
		if len(c.GroupIDs) == 0 {
			return ErrAnnouncementInvalidTarget
		}
		return nil

	case AnnouncementConditionTypeUser:
		if c.Operator != AnnouncementOperatorIn || len(c.UserIDs) == 0 {
			return ErrAnnouncementInvalidTarget
		}
		return nil

	case AnnouncementConditionTypeBalance:
		switch c.Operator {
		case AnnouncementOperatorGT, AnnouncementOperatorGTE, AnnouncementOperatorLT, AnnouncementOperatorLTE, AnnouncementOperatorEQ:
			return nil
		default:
			return ErrAnnouncementInvalidTarget
		}

	case AnnouncementConditionTypePlayMembership:
		if c.Operator != AnnouncementOperatorIn {
			return ErrAnnouncementInvalidTarget
		}
		if c.PlayMembership != AnnouncementPlayMembershipOrdinary && c.PlayMembership != AnnouncementPlayMembershipMember {
			return ErrAnnouncementInvalidTarget
		}
		return nil

	default:
		return ErrAnnouncementInvalidTarget
	}
}

type Announcement struct {
	ID         int64
	Title      string
	Content    string
	Status     string
	NotifyMode string
	Targeting  AnnouncementTargeting
	StartsAt   *time.Time
	EndsAt     *time.Time
	CreatedBy  *int64
	UpdatedBy  *int64
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (a *Announcement) IsActiveAt(now time.Time) bool {
	if a == nil {
		return false
	}
	if a.Status != AnnouncementStatusActive {
		return false
	}
	if a.StartsAt != nil && now.Before(*a.StartsAt) {
		return false
	}
	if a.EndsAt != nil && !now.Before(*a.EndsAt) {
		// ends_at 语义：到点即下线
		return false
	}
	return true
}
