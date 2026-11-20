package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
)

type openAIStrictSession struct {
	key      string
	observed int64
	owner    int64
	enforce  bool
}

func (s *OpenAIGatewayService) strictOpenAISessionAffinity(groupID *int64, platform string, imageCapability OpenAIImagesCapability) bool {
	return s != nil && s.cfg != nil && groupID != nil && imageCapability == "" &&
		(platform == PlatformOpenAI || platform == PlatformQwen) &&
		slices.Contains(s.cfg.Gateway.OpenAIScheduler.StrictSessionAffinityGroupIDs, *groupID)
}

func strictSessionUnavailable(reason string) error {
	return fmt.Errorf("%w: strict session affinity %s", ErrNoAvailableAccounts, reason)
}

func (s *defaultOpenAIAccountScheduler) prepareStrictSession(ctx context.Context, req OpenAIAccountScheduleRequest) (OpenAIAccountScheduleRequest, error) {
	strictGroup := s.service.strictOpenAISessionAffinity(req.GroupID, req.Platform, req.RequiredImageCapability)
	if req.RequiredImageCapability != "" || (req.Platform != PlatformQwen && !strictGroup) {
		return req, nil
	}
	session := &openAIStrictSession{enforce: strictGroup}
	req.strictSession = session
	previous := strings.TrimSpace(req.PreviousResponseID)
	if req.SessionHash == "" && previous != "" && req.Platform == PlatformQwen {
		req.SessionHash = "qwen-response:" + DeriveSessionHashFromSeed(previous)
	}
	if req.SessionHash != "" && s.service.cache != nil {
		// The routing cache may be overwritten by the normal failover/admission
		// paths. Keep strict ownership under a separate key.
		session.key = "openai-strict:" + req.SessionHash
		var err error
		session.observed, err = s.service.cache.GetSessionAccountID(ctx, derefGroupID(req.GroupID), session.key)
		if err != nil && !errors.Is(err, ErrStickySessionNotFound) {
			return req, strictSessionUnavailable("binding lookup failed")
		}
		session.owner = session.observed
		if session.owner == 0 {
			session.owner = req.StickyAccountID
		}
	}
	if previous != "" && (req.Platform == PlatformQwen || strictGroup) {
		owner, err := s.service.getOpenAIWSStateStore().GetResponseAccount(ctx, derefGroupID(req.GroupID), previous)
		if err != nil || owner <= 0 || (session.owner > 0 && session.owner != owner) {
			return req, strictSessionUnavailable("response ownership is unknown or conflicts with the session")
		}
		session.owner = owner
		session.enforce = true
	}
	if req.Platform == PlatformQwen && session.owner > 0 {
		owner, err := s.service.accountRepo.GetByID(ctx, session.owner)
		if err != nil || owner == nil {
			return req, strictSessionUnavailable("session owner is unavailable")
		}
		if previous != "" && !owner.QwenSessionCacheEnabled() {
			return req, strictSessionUnavailable("response owner no longer supports native continuation")
		}
		session.enforce = session.enforce || owner.QwenStrictSessionAffinity()
	}
	return req, nil
}

func (s *defaultOpenAIAccountScheduler) selectStrictSession(ctx context.Context, req OpenAIAccountScheduleRequest) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	var decision OpenAIAccountScheduleDecision
	session := req.strictSession
	if session.key == "" {
		return nil, decision, strictSessionUnavailable("requires a session identity and shared cache")
	}
	owner := session.owner
	if req.GuardianParentAccountID > 0 {
		if owner > 0 && req.GuardianParentAccountID != owner {
			return nil, decision, strictSessionUnavailable("task ownership conflicts with the session")
		}
		owner = req.GuardianParentAccountID
	}
	if _, excluded := req.ExcludedIDs[owner]; excluded || (owner == 0 && len(req.ExcludedIDs) > 0) {
		return nil, decision, strictSessionUnavailable("owner failed and cannot be replaced")
	}
	var selection *AccountSelectionResult
	var err error
	if owner > 0 {
		req.StickyAccountID = owner
		req.StickyWeighted = false
		req.DisableStickyEscape = true
		req.PreserveStickyBinding = true
		req.PreviousResponseCanMove = false
		selection, _, err = s.selectBySessionHash(ctx, req)
		decision.Layer = openAIAccountScheduleLayerSessionSticky
		decision.StickySessionHit = true
	} else {
		selection, decision.CandidateCount, decision.TopK, decision.LoadSkew, err = s.selectByLoadBalance(ctx, req)
		decision.Layer = openAIAccountScheduleLayerLoadBalance
	}
	if err != nil {
		return nil, decision, err
	}
	if selection == nil || selection.Account == nil {
		return nil, decision, strictSessionUnavailable("owner is unavailable")
	}
	decision.SelectedAccountID = selection.Account.ID
	decision.SelectedAccountType = selection.Account.Type
	return selection, decision, nil
}

func (s *defaultOpenAIAccountScheduler) claimStrictSession(ctx context.Context, req OpenAIAccountScheduleRequest, account *Account) error {
	session := req.strictSession
	if session.key == "" {
		if session.enforce || account.QwenStrictSessionAffinity() {
			return strictSessionUnavailable("requires a session identity and shared cache")
		}
		return nil
	}
	cache, ok := s.service.cache.(interface {
		CompareAndSwapSessionAccountID(context.Context, int64, SessionAccountBindingSwap) (bool, error)
	})
	if !ok {
		return strictSessionUnavailable("atomic ownership is unavailable")
	}
	applied, err := cache.CompareAndSwapSessionAccountID(ctx, derefGroupID(req.GroupID), SessionAccountBindingSwap{
		SessionHash: session.key, ExpectedAccountID: session.observed, AccountID: account.ID,
		TTL: s.service.openAIWSSessionStickyTTL(),
	})
	if err != nil || !applied {
		return strictSessionUnavailable("binding changed during selection")
	}
	return nil
}
