package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

type UserBatchAction string

const (
	UserBatchActionDisable UserBatchAction = "disable"
	UserBatchActionDelete  UserBatchAction = "delete"

	UserBatchActionResultCompleted = "completed"
	UserBatchActionResultPartial   = "partial"
	UserBatchActionResultFailed    = "failed"

	MaxUserBatchActionUsers = 500
	userBatchPreviewTTL     = 5 * time.Minute
)

var (
	ErrUserBatchActionInvalidAction  = infraerrors.BadRequest("USER_BATCH_ACTION_INVALID", "unsupported user batch action")
	ErrUserBatchActionReasonRequired = infraerrors.BadRequest("USER_BATCH_ACTION_REASON_REQUIRED", "batch action reason is required and must not exceed 1000 characters")
	ErrUserBatchActionInvalidIDs     = infraerrors.BadRequest("USER_BATCH_ACTION_INVALID_IDS", "user_ids must contain between 1 and 500 positive user IDs")
	ErrUserBatchActionTooLarge       = infraerrors.BadRequest("USER_BATCH_ACTION_TOO_LARGE", "user_ids cannot exceed 500")
	ErrUserBatchActionPreviewInvalid = infraerrors.BadRequest("USER_BATCH_ACTION_PREVIEW_INVALID", "invalid batch action preview token")
	ErrUserBatchActionPreviewExpired = infraerrors.Conflict("USER_BATCH_ACTION_PREVIEW_EXPIRED", "batch action preview expired; generate a new preview")
	ErrUserBatchActionPreviewStale   = infraerrors.Conflict("USER_BATCH_ACTION_PREVIEW_STALE", "user data changed after preview; generate a new preview")
	ErrUserBatchActionSignerMissing  = infraerrors.InternalServer("USER_BATCH_ACTION_SIGNER_UNAVAILABLE", "batch action preview signer is unavailable")
)

type UserBatchActionInput struct {
	Action            UserBatchAction
	UserIDs           []int64
	Reason            string
	ConfirmationToken string
	ActorAdminID      int64
}

type UserBatchActionTarget struct {
	ID          int64  `json:"id"`
	Email       string `json:"email"`
	Role        string `json:"role"`
	Status      string `json:"status"`
	APIKeyCount int    `json:"api_key_count"`
}

type UserBatchActionPreview struct {
	Action                  UserBatchAction         `json:"action"`
	RequestedCount          int                     `json:"requested_count"`
	EligibleUsers           []UserBatchActionTarget `json:"eligible_users"`
	ProtectedAdministrators []UserBatchActionTarget `json:"protected_administrators"`
	AlreadyDisabledUsers    []UserBatchActionTarget `json:"already_disabled_users"`
	MissingUserIDs          []int64                 `json:"missing_user_ids"`
	AffectedAPIKeys         int                     `json:"affected_api_keys"`
	RequiresStepUp          bool                    `json:"requires_step_up"`
	ConfirmationToken       string                  `json:"confirmation_token"`
	ExpiresAt               time.Time               `json:"expires_at"`
	stateDigest             string
}

type UserBatchActionResultItem struct {
	UserID  int64  `json:"user_id"`
	Email   string `json:"email,omitempty"`
	Reason  string `json:"reason"`
	Message string `json:"message,omitempty"`
}

type UserBatchActionResult struct {
	Action           UserBatchAction             `json:"action"`
	Status           string                      `json:"status"`
	RequestedCount   int                         `json:"requested_count"`
	SucceededUserIDs []int64                     `json:"succeeded_user_ids"`
	Skipped          []UserBatchActionResultItem `json:"skipped"`
	Failed           []UserBatchActionResultItem `json:"failed"`
	AffectedAPIKeys  int                         `json:"affected_api_keys"`
}

type userBatchPreviewToken struct {
	Action       UserBatchAction `json:"action"`
	UserIDs      []int64         `json:"user_ids"`
	ReasonDigest string          `json:"reason_digest"`
	StateDigest  string          `json:"state_digest"`
	ExpiresAt    time.Time       `json:"expires_at"`
}

func (s *adminServiceImpl) PreviewUserBatchAction(ctx context.Context, input UserBatchActionInput) (*UserBatchActionPreview, error) {
	userIDs, reason, err := validateUserBatchActionInput(input)
	if err != nil {
		return nil, err
	}
	if len(s.userBatchPreviewKey) == 0 {
		return nil, ErrUserBatchActionSignerMissing
	}
	preview, err := s.buildUserBatchActionPreview(ctx, input.Action, userIDs)
	if err != nil {
		return nil, err
	}
	preview.ExpiresAt = time.Now().UTC().Add(userBatchPreviewTTL)
	token, err := encodeUserBatchPreviewToken(userBatchPreviewToken{
		Action:       input.Action,
		UserIDs:      userIDs,
		ReasonDigest: userBatchReasonDigest(reason),
		StateDigest:  preview.stateDigest,
		ExpiresAt:    preview.ExpiresAt,
	}, s.userBatchPreviewKey)
	if err != nil {
		return nil, err
	}
	preview.ConfirmationToken = token
	return preview, nil
}

func (s *adminServiceImpl) ExecuteUserBatchAction(ctx context.Context, input UserBatchActionInput) (*UserBatchActionResult, error) {
	userIDs, reason, err := validateUserBatchActionInput(input)
	if err != nil {
		return nil, err
	}
	if len(s.userBatchPreviewKey) == 0 {
		return nil, ErrUserBatchActionSignerMissing
	}
	token, err := decodeUserBatchPreviewToken(input.ConfirmationToken, s.userBatchPreviewKey)
	if err != nil {
		return nil, err
	}
	if time.Now().UTC().After(token.ExpiresAt) {
		return nil, ErrUserBatchActionPreviewExpired
	}
	if token.Action != input.Action ||
		!sameInt64Slice(token.UserIDs, userIDs) ||
		token.ReasonDigest != userBatchReasonDigest(reason) {
		return nil, ErrUserBatchActionPreviewStale
	}

	preview, err := s.buildUserBatchActionPreview(ctx, input.Action, userIDs)
	if err != nil {
		return nil, err
	}
	if token.StateDigest != preview.stateDigest {
		return nil, ErrUserBatchActionPreviewStale
	}

	result := &UserBatchActionResult{
		Action:         input.Action,
		RequestedCount: preview.RequestedCount,
	}
	for _, target := range preview.ProtectedAdministrators {
		result.Skipped = append(result.Skipped, UserBatchActionResultItem{
			UserID: target.ID, Email: target.Email, Reason: "protected_administrator",
		})
	}
	for _, target := range preview.AlreadyDisabledUsers {
		result.Skipped = append(result.Skipped, UserBatchActionResultItem{
			UserID: target.ID, Email: target.Email, Reason: "already_disabled",
		})
	}
	for _, userID := range preview.MissingUserIDs {
		result.Skipped = append(result.Skipped, UserBatchActionResultItem{
			UserID: userID, Reason: "not_found",
		})
	}

	for _, target := range preview.EligibleUsers {
		switch input.Action {
		case UserBatchActionDisable:
			_, err = s.UpdateUser(ctx, target.ID, &UpdateUserInput{
				Status: StatusDisabled, ActorAdminID: input.ActorAdminID,
			})
		case UserBatchActionDelete:
			err = s.DeleteUser(ctx, target.ID)
		}
		if err != nil {
			logger.LegacyPrintf(
				"service.admin",
				"user batch action item failed actor_admin_id=%d action=%s target_user_id=%d err=%v",
				input.ActorAdminID, input.Action, target.ID, err,
			)
			result.Failed = append(result.Failed, UserBatchActionResultItem{
				UserID: target.ID, Email: target.Email, Reason: "operation_failed",
				Message: userBatchActionFailureMessage(err),
			})
			continue
		}
		result.SucceededUserIDs = append(result.SucceededUserIDs, target.ID)
		if input.Action == UserBatchActionDelete {
			result.AffectedAPIKeys += target.APIKeyCount
		}
	}

	switch {
	case len(result.Failed) == 0:
		result.Status = UserBatchActionResultCompleted
	case len(result.SucceededUserIDs) > 0:
		result.Status = UserBatchActionResultPartial
	default:
		result.Status = UserBatchActionResultFailed
	}
	logger.LegacyPrintf(
		"service.admin",
		"audit: user batch action actor_admin_id=%d action=%s requested=%d succeeded=%d skipped=%d failed=%d affected_api_keys=%d",
		input.ActorAdminID, input.Action, result.RequestedCount, len(result.SucceededUserIDs),
		len(result.Skipped), len(result.Failed), result.AffectedAPIKeys,
	)
	return result, nil
}

func userBatchActionFailureMessage(err error) string {
	appErr := infraerrors.FromError(err)
	if appErr == nil || appErr.Code >= 500 {
		return ""
	}
	return appErr.Message
}

func validateUserBatchActionInput(input UserBatchActionInput) ([]int64, string, error) {
	if input.Action != UserBatchActionDisable && input.Action != UserBatchActionDelete {
		return nil, "", ErrUserBatchActionInvalidAction
	}
	reason := strings.TrimSpace(input.Reason)
	if reason == "" || len([]rune(reason)) > 1000 {
		return nil, "", ErrUserBatchActionReasonRequired
	}
	if len(input.UserIDs) > MaxUserBatchActionUsers {
		return nil, "", ErrUserBatchActionTooLarge
	}
	seen := make(map[int64]struct{}, len(input.UserIDs))
	userIDs := make([]int64, 0, len(input.UserIDs))
	for _, id := range input.UserIDs {
		if id <= 0 {
			return nil, "", ErrUserBatchActionInvalidIDs
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		userIDs = append(userIDs, id)
	}
	if len(userIDs) == 0 {
		return nil, "", ErrUserBatchActionInvalidIDs
	}
	sort.Slice(userIDs, func(i, j int) bool { return userIDs[i] < userIDs[j] })
	return userIDs, reason, nil
}

func (s *adminServiceImpl) buildUserBatchActionPreview(
	ctx context.Context,
	action UserBatchAction,
	userIDs []int64,
) (*UserBatchActionPreview, error) {
	preview := &UserBatchActionPreview{
		Action: action, RequestedCount: len(userIDs), RequiresStepUp: true,
	}
	state := make([]string, 0, len(userIDs))
	for _, userID := range userIDs {
		user, err := s.userRepo.GetByID(ctx, userID)
		if err != nil {
			if errors.Is(err, ErrUserNotFound) {
				preview.MissingUserIDs = append(preview.MissingUserIDs, userID)
				state = append(state, fmt.Sprintf("%d:missing", userID))
				continue
			}
			return nil, err
		}
		target := UserBatchActionTarget{
			ID: user.ID, Email: user.Email, Role: user.Role, Status: user.Status,
		}
		if user.Role == RoleAdmin {
			preview.ProtectedAdministrators = append(preview.ProtectedAdministrators, target)
			state = append(state, fmt.Sprintf("%d:admin:%s", user.ID, user.Status))
			continue
		}
		if action == UserBatchActionDisable && user.Status == StatusDisabled {
			preview.AlreadyDisabledUsers = append(preview.AlreadyDisabledUsers, target)
			state = append(state, fmt.Sprintf("%d:disabled", user.ID))
			continue
		}
		if action == UserBatchActionDelete {
			keys, err := s.listUserAPIKeysForDeletion(ctx, user.ID)
			if err != nil {
				return nil, err
			}
			target.APIKeyCount = len(keys)
			preview.AffectedAPIKeys += len(keys)
			keyIDs := make([]int64, 0, len(keys))
			for _, key := range keys {
				keyIDs = append(keyIDs, key.ID)
			}
			sort.Slice(keyIDs, func(i, j int) bool { return keyIDs[i] < keyIDs[j] })
			state = append(state, fmt.Sprintf("%d:user:%s:%v", user.ID, user.Status, keyIDs))
		} else {
			state = append(state, fmt.Sprintf("%d:user:%s", user.ID, user.Status))
		}
		preview.EligibleUsers = append(preview.EligibleUsers, target)
	}
	digest := sha256.Sum256([]byte(strings.Join(state, "|")))
	preview.stateDigest = hex.EncodeToString(digest[:])
	return preview, nil
}

func deriveUserBatchPreviewSigningKey(cfg *config.Config) []byte {
	if cfg == nil {
		return nil
	}
	secret := strings.TrimSpace(cfg.JWT.Secret)
	if secret == "" {
		return nil
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte("sub2api:admin-user-batch-preview:v1"))
	return mac.Sum(nil)
}

func encodeUserBatchPreviewToken(payload userBatchPreviewToken, signingKey []byte) (string, error) {
	if len(signingKey) == 0 {
		return "", ErrUserBatchActionSignerMissing
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, signingKey)
	_, _ = mac.Write([]byte("admin-user-batch-preview\x00"))
	_, _ = mac.Write(raw)
	return base64.RawURLEncoding.EncodeToString(raw) + "." + hex.EncodeToString(mac.Sum(nil)), nil
}

func decodeUserBatchPreviewToken(token string, signingKey []byte) (*userBatchPreviewToken, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 2 || len(signingKey) == 0 {
		return nil, ErrUserBatchActionPreviewInvalid
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrUserBatchActionPreviewInvalid
	}
	signature, err := hex.DecodeString(parts[1])
	if err != nil {
		return nil, ErrUserBatchActionPreviewInvalid
	}
	mac := hmac.New(sha256.New, signingKey)
	_, _ = mac.Write([]byte("admin-user-batch-preview\x00"))
	_, _ = mac.Write(raw)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return nil, ErrUserBatchActionPreviewInvalid
	}
	var payload userBatchPreviewToken
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, ErrUserBatchActionPreviewInvalid
	}
	return &payload, nil
}

func userBatchReasonDigest(reason string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(reason)))
	return hex.EncodeToString(sum[:])
}

func sameInt64Slice(left, right []int64) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
