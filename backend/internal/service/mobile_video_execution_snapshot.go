package service

import (
	"context"
	"errors"
	"math"
	"strings"
)

// MobileVideoExecutionSnapshotVersion identifies the private, immutable
// execution identity stored with a funded mobile video job. It intentionally
// serializes the auth projection, never the API key credential itself.
const MobileVideoExecutionSnapshotVersion = 1

var ErrMobileVideoExecutionSnapshotInvalid = errors.New("mobile video execution snapshot is invalid")

// MobileVideoExecutionSnapshot freezes the authorization and customer video
// multiplier accepted at submission time. A worker may restore it after a
// subscription, group assignment, or balance changes, but it must still check
// that the dedicated execution key was not disabled/deleted and that the
// current provider mapping can actually run the request.
type MobileVideoExecutionSnapshot struct {
	Version                      int                `json:"version"`
	APIKey                       APIKeyAuthSnapshot `json:"api_key"`
	EffectiveVideoRateMultiplier float64            `json:"effective_video_rate_multiplier"`
}

// CaptureMobileVideoExecutionSnapshot performs the normal live authorization
// check at submission time, then stores only a redacted auth snapshot. The
// user/group multiplier is frozen before the balance hold is calculated so a
// later admin rate update cannot change an already accepted task's price.
func (s *APIKeyService) CaptureMobileVideoExecutionSnapshot(
	ctx context.Context,
	userID, groupID, keyID int64,
) (*MobileVideoExecutionSnapshot, error) {
	apiKey, err := s.GetMobileVideoExecutionKey(ctx, userID, groupID, keyID)
	if err != nil {
		return nil, err
	}
	if apiKey == nil || apiKey.User == nil || apiKey.Group == nil ||
		apiKey.UserID != userID || apiKey.GroupID == nil || *apiKey.GroupID != groupID {
		return nil, ErrMobileVideoExecutionSnapshotInvalid
	}
	snapshot := s.snapshotFromAPIKey(ctx, apiKey)
	if snapshot == nil || snapshot.Group == nil || snapshot.UserID != userID ||
		snapshot.APIKeyID != keyID || snapshot.GroupID == nil || *snapshot.GroupID != groupID ||
		!IsMobileVideoExecutionAPIKeyName(snapshot.Name) {
		return nil, ErrMobileVideoExecutionSnapshotInvalid
	}

	multiplier := apiKey.Group.RateMultiplier
	if s != nil && s.userGroupRateRepo != nil {
		override, overrideErr := s.userGroupRateRepo.GetByUserAndGroup(ctx, userID, groupID)
		if overrideErr != nil {
			// Submission must not quote one rate and reserve another merely because
			// the per-user override lookup was temporarily unavailable.
			return nil, overrideErr
		}
		if override != nil {
			multiplier = *override
		}
	}
	multiplier = resolveVideoRateMultiplier(apiKey, multiplier)
	if math.IsNaN(multiplier) || math.IsInf(multiplier, 0) || multiplier < 0 {
		return nil, ErrMobileVideoExecutionSnapshotInvalid
	}

	return &MobileVideoExecutionSnapshot{
		Version:                      MobileVideoExecutionSnapshotVersion,
		APIKey:                       *snapshot,
		EffectiveVideoRateMultiplier: multiplier,
	}, nil
}

// RestoreMobileVideoExecutionSnapshot reconstructs a private APIKey without a
// raw credential. It deliberately performs no current entitlement/balance
// lookup: those checks happened before accepting and reserving the task.
func (s *APIKeyService) RestoreMobileVideoExecutionSnapshot(snapshot *MobileVideoExecutionSnapshot) (*APIKey, error) {
	if !ValidMobileVideoExecutionSnapshot(snapshot, 0, 0, 0) {
		return nil, ErrMobileVideoExecutionSnapshotInvalid
	}
	apiKey := s.snapshotToAPIKey("", &snapshot.APIKey)
	if apiKey == nil || apiKey.Key != "" || apiKey.User == nil || apiKey.Group == nil {
		return nil, ErrMobileVideoExecutionSnapshotInvalid
	}
	// Freeze the final video multiplier. Normal gateway billing consumes this
	// reconstructed group and must not recalculate a user/group override.
	apiKey.Group.VideoRateIndependent = true
	apiKey.Group.VideoRateMultiplier = snapshot.EffectiveVideoRateMultiplier
	return apiKey, nil
}

// ValidateMobileVideoExecutionKeyForExecution verifies the dedicated managed
// key is still structurally usable without re-evaluating the user's live group
// entitlement. A disabled/deleted key remains an operational stop condition;
// a revoked subscription does not strand an already accepted task.
func (s *APIKeyService) ValidateMobileVideoExecutionKeyForExecution(
	ctx context.Context,
	userID, groupID, keyID int64,
) error {
	if s == nil || userID <= 0 || groupID <= 0 || keyID <= 0 {
		return ErrMobileVideoExecutionSnapshotInvalid
	}
	apiKey, err := s.GetByID(ctx, keyID)
	if err != nil {
		return err
	}
	if apiKey == nil || apiKey.UserID != userID || apiKey.GroupID == nil ||
		*apiKey.GroupID != groupID || !apiKey.IsActive() || apiKey.IsExpired() ||
		!IsMobileVideoExecutionAPIKeyName(apiKey.Name) {
		return ErrMobileVideoExecutionSnapshotInvalid
	}
	return nil
}

// ValidMobileVideoExecutionSnapshot is shared by the store and gateway
// adapter. Passing zero identity fields validates only the snapshot shape.
func ValidMobileVideoExecutionSnapshot(snapshot *MobileVideoExecutionSnapshot, userID, groupID, keyID int64) bool {
	if snapshot == nil || snapshot.Version != MobileVideoExecutionSnapshotVersion ||
		snapshot.APIKey.Version <= 0 || snapshot.APIKey.APIKeyID <= 0 ||
		snapshot.APIKey.UserID <= 0 || snapshot.APIKey.User.ID <= 0 ||
		snapshot.APIKey.UserID != snapshot.APIKey.User.ID || snapshot.APIKey.GroupID == nil ||
		snapshot.APIKey.Group == nil || snapshot.APIKey.Group.ID <= 0 ||
		strings.TrimSpace(snapshot.APIKey.Name) == "" ||
		!IsMobileVideoExecutionAPIKeyName(snapshot.APIKey.Name) ||
		math.IsNaN(snapshot.EffectiveVideoRateMultiplier) ||
		math.IsInf(snapshot.EffectiveVideoRateMultiplier, 0) ||
		snapshot.EffectiveVideoRateMultiplier < 0 {
		return false
	}
	if *snapshot.APIKey.GroupID != snapshot.APIKey.Group.ID {
		return false
	}
	if userID > 0 && snapshot.APIKey.UserID != userID {
		return false
	}
	if groupID > 0 && *snapshot.APIKey.GroupID != groupID {
		return false
	}
	return keyID <= 0 || snapshot.APIKey.APIKeyID == keyID
}
