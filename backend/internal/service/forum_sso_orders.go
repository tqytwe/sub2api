package service

import (
	"context"
	"fmt"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"

	"github.com/shopspring/decimal"
)

// Forum purchases (paid topics, badges, VIP upgrades) are wallet spends, not
// recharges. They are therefore recorded through BalanceLedgerService rather
// than as payment_orders: payment_orders models an inbound recharge with
// coupons, gateway fees and a recharge code, none of which apply here.
//
// The ledger transaction IS the order record. Its IdempotencyKey gives us
// exactly-once semantics, and its Metadata carries the forum item so the
// purchase is auditable from the platform side.
const (
	ForumOrderSourceType = "forum_purchase"

	forumItemTypeTopic      = "topic"
	forumItemTypeBadge      = "badge"
	forumItemTypeVIPUpgrade = "vip_upgrade"
)

var (
	ErrForumOrderInvalidItemType = infraerrors.BadRequest("FORUM_ORDER_INVALID_ITEM_TYPE", "invalid item_type")
	ErrForumOrderInvalidAmount   = infraerrors.BadRequest("FORUM_ORDER_INVALID_AMOUNT", "invalid amount")
	ErrForumOrderInvalidItemID   = infraerrors.BadRequest("FORUM_ORDER_INVALID_ITEM_ID", "item_id is required")
	ErrForumOrderLedgerDown      = infraerrors.InternalServer("FORUM_ORDER_LEDGER_UNAVAILABLE", "balance ledger is unavailable")
)

// forumValidItemTypes mirrors VALID_ITEM_TYPES in the plugin's forum-payment.js.
// Keeping the two in sync matters: the forum grants the entitlement based on
// item_type when the payment callback lands.
func forumValidItemType(itemType string) bool {
	switch itemType {
	case forumItemTypeTopic, forumItemTypeBadge, forumItemTypeVIPUpgrade:
		return true
	default:
		return false
	}
}

// ForumOrderRequest is the body the forum posts to /api/v1/sso/forum/orders.
// Amount arrives as a string ("10.00") because the plugin sends
// numericAmount.toFixed(2).
type ForumOrderRequest struct {
	ItemID      string `json:"item_id"`
	ItemType    string `json:"item_type"`
	Amount      string `json:"amount"`
	Description string `json:"description"`
}

// ForumOrderResult is returned to the forum. order_id is required by the plugin
// (it throws platform_returned_no_order_id otherwise).
type ForumOrderResult struct {
	OrderID       string `json:"order_id"`
	Status        string `json:"status"`
	Amount        string `json:"amount"`
	BalanceAfter  string `json:"balance_after"`
	AlreadyPaid   bool   `json:"already_paid"`
	TransactionID int64  `json:"transaction_id"`
}

// CreateForumOrder charges the user's platform wallet for a forum purchase.
//
// The charge is applied synchronously with reject_negative, so an underfunded
// user gets a clean 400 instead of an overdraft. The caller fires the forum
// payment callback after this returns, which is what grants the entitlement.
func (s *ForumSSOService) CreateForumOrder(ctx context.Context, userID int64, req ForumOrderRequest) (*ForumOrderResult, error) {
	if !s.Enabled() {
		return nil, ErrForumSSODisabled
	}
	if s.ledger == nil {
		return nil, ErrForumOrderLedgerDown
	}

	itemID := strings.TrimSpace(req.ItemID)
	if itemID == "" {
		return nil, ErrForumOrderInvalidItemID
	}
	itemType := strings.TrimSpace(req.ItemType)
	if !forumValidItemType(itemType) {
		return nil, ErrForumOrderInvalidItemType
	}

	amount, err := decimal.NewFromString(strings.TrimSpace(req.Amount))
	if err != nil || amount.LessThanOrEqual(decimal.Zero) {
		return nil, ErrForumOrderInvalidAmount
	}
	// Money is settled at 2dp; a request carrying more precision is a contract
	// violation rather than something to silently round.
	if amount.Exponent() < -2 {
		return nil, ErrForumOrderInvalidAmount
	}

	orderID := forumOrderID(userID, itemType, itemID)

	// Idempotency key derived from the order identity, so a retried POST from the
	// forum charges once. The ledger returns the original transaction on replay.
	idempotencyKey := "forum_purchase:" + orderID

	description := strings.TrimSpace(req.Description)
	if description == "" {
		description = fmt.Sprintf("Forum %s purchase", itemType)
	}
	if len(description) > 255 {
		description = description[:255]
	}

	txn, err := s.ledger.ApplyDelta(ctx, BalanceLedgerApplyInput{
		UserID:         userID,
		BalanceDelta:   amount.Neg().InexactFloat64(),
		SourceType:     ForumOrderSourceType,
		SourceID:       orderID,
		IdempotencyKey: idempotencyKey,
		ActorType:      BalanceLedgerActorUser,
		ActorUserID:    &userID,
		Description:    description,
		Metadata: map[string]any{
			"item_id":   itemID,
			"item_type": itemType,
			"order_id":  orderID,
			"channel":   "community_forum",
		},
		Confidence: BalanceLedgerConfidenceHigh,
		// An underfunded purchase must fail loudly, never overdraw the wallet.
		BalancePolicy: BalanceLedgerPolicyRejectNegative,
	})
	if err != nil {
		return nil, err
	}

	result := &ForumOrderResult{
		OrderID:       orderID,
		Status:        "paid",
		Amount:        amount.StringFixed(2),
		TransactionID: txn.ID,
		AlreadyPaid:   txn.Replayed,
	}
	if txn.BalanceAfter != nil {
		result.BalanceAfter = formatForumMoney(*txn.BalanceAfter)
	}
	return result, nil
}

// forumOrderID is deterministic on (user, item type, item id) so a duplicate
// purchase attempt maps to the same idempotency key and cannot double-charge.
func forumOrderID(userID int64, itemType, itemID string) string {
	return fmt.Sprintf("forum-%d-%s-%s", userID, itemType, itemID)
}

// ForumWalletBalance is the response for /api/v1/sso/wallet/balance. All money
// fields are strings to keep the forum's display and the signed webhook payloads
// on one consistent representation.
type ForumWalletBalance struct {
	UserID           int64   `json:"user_id"`
	Balance          string  `json:"balance"`
	FrozenBalance    string  `json:"frozen_balance"`
	Currency         string  `json:"currency"`
	VIPTier          int     `json:"vip_tier"`
	VIPLabel         string  `json:"vip_label"`
	RechargeBonusPct float64 `json:"recharge_bonus_pct"`
}

// GetWalletBalance returns the live wallet snapshot for a forum user.
func (s *ForumSSOService) GetWalletBalance(ctx context.Context, userID int64) (*ForumWalletBalance, error) {
	if !s.Enabled() {
		return nil, ErrForumSSODisabled
	}
	user, err := s.userService.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrForumSSOBadToken
	}

	vip := s.resolveVIP(ctx, user)
	return &ForumWalletBalance{
		UserID:           user.ID,
		Balance:          formatForumMoney(user.Balance),
		FrozenBalance:    formatForumMoney(user.FrozenBalance),
		Currency:         "CNY",
		VIPTier:          vip.Tier,
		VIPLabel:         vip.Label,
		RechargeBonusPct: vip.RechargeBonusPct,
	}, nil
}
