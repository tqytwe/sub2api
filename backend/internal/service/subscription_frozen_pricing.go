package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/tidwall/gjson"
)

var ErrSubscriptionAdmissionUnsupported = errors.New("subscription admission pricing or usage is outside the supported text scope")

// ResolvedPricing contains private policy fields. Keep them explicitly in the
// versioned snapshot; direct json.Marshal(ResolvedPricing) silently loses them.
type subscriptionResolvedPricing struct {
	Pricing                   *ResolvedPricing
	Channel                   *ChannelModelPricing
	LongContext               bool
	Identified, ChannelPriced bool
}

func freezeResolvedPricing(p *ResolvedPricing) subscriptionResolvedPricing {
	return subscriptionResolvedPricing{Pricing: p, Channel: p.channelPricing, LongContext: p.longContextPricingEnabled}
}
func (p subscriptionResolvedPricing) resolved() *ResolvedPricing {
	if p.Pricing == nil {
		return nil
	}
	v := *p.Pricing
	v.channelPricing = p.Channel
	v.longContextPricingEnabled = p.LongContext
	return &v
}

type subscriptionFrozenPricing struct {
	Version                                       int
	Models                                        map[string]subscriptionResolvedPricing
	Multiplier, BaseMultiplier, AccountMultiplier float64
	PricingAt                                     time.Time
	Surcharge                                     BillingSurchargeConfig
	LongContext, FreeFast, OAuthLike              bool
	UsageFields                                   ChannelUsageFields
	StatsChannel                                  *Channel
	StatsPlatform                                 string
	StatsModels                                   map[string]subscriptionResolvedPricing
}
type subscriptionFrozenKey struct{}
type subscriptionObservedKey struct{}

type subscriptionUsagePersister interface {
	PersistSubscriptionUsage(context.Context, requestledger.SubscriptionObservation) error
}

// PrepareSubscriptionAdmission opts in only native HTTP text subscriptions.
// Unsupported tools/media retain their existing path and receive no revocation
// exception. Every actual transport attempt commits its own receipt.
func (s *OpenAIGatewayService) PrepareSubscriptionAdmission(ctx context.Context, key *APIKey, sub *UserSubscription, account *Account, body []byte, fields ChannelUsageFields, payloadHash string, pricingAt time.Time) (context.Context, error) {
	ctx = requestledger.WithoutSubscriptionAdmission(ctx)
	if s == nil || requestledger.FromContext(ctx) == nil || key == nil || key.Group == nil || key.GroupID == nil || sub == nil || account == nil || !account.IsOpenAI() ||
		!key.Group.IsSubscriptionType() || IsImageStudioManagedBilling(ctx) || (s.cfg != nil && s.cfg.RunMode == config.RunModeSimple) {
		return ctx, nil
	}
	if shouldForwardOpenAIResponsesViaRawChatCompletions(account) || !subscriptionNativeTextRequest(body) {
		return ctx, nil
	}
	model := strings.TrimSpace(gjson.GetBytes(body, "model").String())
	if model == "" {
		return ctx, ErrSubscriptionAdmissionUnsupported
	}
	credential, err := resolveCredentialAccount(ctx, s.accountRepo, account)
	if err != nil {
		return ctx, err
	}
	resolver := s.resolver
	if resolver == nil {
		resolver = NewModelPricingResolver(s.channelService, s.billingService)
	}
	base := s.ResolveUserGroupRateMultiplier(ctx, key.UserID, *key.GroupID, key.Group.RateMultiplier)
	multiplier, _ := computePeakAwareMultipliers(key, base, pricingAt)
	snap := subscriptionFrozenPricing{Version: 1, UsageFields: fields, Models: make(map[string]subscriptionResolvedPricing), StatsModels: make(map[string]subscriptionResolvedPricing), BaseMultiplier: base, Multiplier: multiplier,
		AccountMultiplier: account.BillingRateMultiplier(), PricingAt: pricingAt, FreeFast: groupBillsOpenAIFastAtStandard(key, credential, "priority"), OAuthLike: credential.IsOpenAIOAuthLike()}
	if gate := openAILongContextBillingGate(credential); gate != nil {
		snap.LongContext = *gate
	}
	global := BillingSurchargeConfig{}
	if s.settingService != nil {
		global = s.settingService.GetBillingSurchargeConfig(ctx)
	}
	snap.Surcharge = ResolveGroupBillingSurcharge(key.Group, global)
	if s.channelService != nil {
		var channel *Channel
		channel, err = s.channelService.GetChannelForGroup(ctx, *key.GroupID)
		if err != nil {
			return ctx, err
		}
		if channel != nil {
			snap.StatsChannel = &Channel{ApplyPricingToAccountStats: channel.ApplyPricingToAccountStats, AccountStatsPricingRules: channel.AccountStatsPricingRules}
		}
		snap.StatsPlatform = s.channelService.GetGroupPlatform(ctx, *key.GroupID)
	}
	customerModels := usageBillingModelCandidates(fields.OriginalModel, fields.ChannelMappedModel, model, account.GetMappedModel(model))
	for _, candidate := range customerModels {
		p := resolver.Resolve(ctx, PricingInput{Model: candidate, GroupID: key.GroupID, Group: key.Group})
		if p == nil || (p.Mode != BillingModeToken && p.Mode != BillingModePerRequest) || (p.Mode == BillingModeToken && p.BasePricing == nil) {
			snap.Models[subscriptionPricingModelKey(candidate)] = subscriptionResolvedPricing{}
			continue
		}
		card := freezeResolvedPricing(p)
		card.Identified, card.ChannelPriced = s.hasIdentifiedOpenAIResponsePricing(ctx, candidate, key)
		snap.Models[subscriptionPricingModelKey(candidate)] = card
	}
	// Statistics follow the actual outbound model, including compact mappings
	// and a same-account compatibility fallback. Freeze those independently so
	// they cannot become additional customer billing candidates.
	statsModels := append(append([]string(nil), customerModels...),
		s.resolveOpenAIRequestSchedulingModel(ctx, account, model, false),
		s.resolveOpenAICompactFallbackModel(account, model))
	for _, candidate := range usageBillingModelCandidates(model, statsModels...) {
		stats := NewModelPricingResolver(nil, s.billingService).Resolve(ctx, PricingInput{Model: candidate})
		if stats != nil {
			stats.longContextPricingEnabled = snap.LongContext
			snap.StatsModels[subscriptionPricingModelKey(candidate)] = freezeResolvedPricing(stats)
		}
	}
	payload, err := json.Marshal(snap)
	if err != nil {
		return ctx, err
	}
	var entitlement *int64
	if sub.PackageEntitlement != nil {
		id := sub.PackageEntitlement.ID
		entitlement = &id
	}
	in := requestledger.SubscriptionAdmission{UserID: key.UserID, APIKeyID: key.ID, GroupID: *key.GroupID, SubscriptionID: sub.ID, PackageEntitlementID: entitlement,
		AccountID: account.ID, BillingRequestID: resolveUsageBillingRequestID(ctx, ""), RequestPayloadHash: payloadHash, PricingPayload: string(payload)}
	return requestledger.WithSubscriptionAdmission(ctx, in, SubscriptionWindowLimitsTx), nil
}

func frozenSubscriptionPricing(ctx context.Context) *subscriptionFrozenPricing {
	p, _ := ctx.Value(subscriptionFrozenKey{}).(*subscriptionFrozenPricing)
	return p
}

func loadSubscriptionPricing(ctx context.Context, input *OpenAIRecordUsageInput) (context.Context, *subscriptionFrozenPricing, error) {
	r := requestledger.CurrentSubscriptionReceipt(ctx)
	if r == nil {
		return ctx, nil, nil
	}
	var snap subscriptionFrozenPricing
	if requestledger.SubscriptionDigest(r.PricingPayload) != r.PricingHash || json.Unmarshal([]byte(r.PricingPayload), &snap) != nil || snap.Version != 1 {
		return ctx, nil, requestledger.ErrUnavailable
	}
	result := input.Result
	if input.APIKey == nil || input.Account == nil || input.Subscription == nil || r.APIKeyID != input.APIKey.ID || r.UserID != input.APIKey.UserID || r.SubscriptionID != input.Subscription.ID || r.AccountID != input.Account.ID {
		return ctx, nil, ErrUsageBillingOwnershipMismatch
	}
	usage, err := json.Marshal(result.Usage)
	if err != nil {
		return ctx, nil, err
	}
	ctx = context.WithValue(ctx, subscriptionObservedKey{}, string(usage))
	chain := input.ModelMappingChain
	input.ChannelUsageFields = snap.UsageFields
	input.ModelMappingChain = chain
	unsupported := result.OpenAIWSMode || result.ImageCount > 0 || result.VideoCount > 0 || result.AudioUsage != nil || result.WebSearchCalls > 0 || result.SearchCount > 0 || result.StarframeVideoBilling != nil ||
		result.Usage.InputAudioTokens > 0 || result.Usage.OutputAudioTokens > 0 || result.Usage.ImageInputTokens > 0 || result.Usage.ImageOutputTokens > 0 || result.Usage.UnpricedOutputUsage != nil
	if _, known := serviceTierCostRank(optionalStringValue(result.ServiceTier)); !known {
		unsupported = true
	}
	if _, known := serviceTierCostRank(result.UpstreamResponseServiceTier); !known {
		unsupported = true
	}

	// Unknown response-model pricing is pending, never silently re-resolved at
	// today's price. The first implementation admits only the frozen model set.
	if model := strings.TrimSpace(result.UpstreamResponseModel); snap.UsageFields.BillingModelSource == BillingModelSourceResponse && model != "" {
		_, known := snap.Models[subscriptionPricingModelKey(model)]
		unsupported = unsupported || !known || result.UpstreamResponseModelConflict
	}
	if unsupported || !result.HasObservedUsage() {
		if result.HasObservedUsage() || result.Usage.UnpricedOutputUsage != nil {
			if err := requestledger.PersistSubscriptionUsage(ctx, requestledger.SubscriptionObservation{Usage: string(usage), PendingReason: "unsupported_usage"}, nil); err != nil {
				return ctx, nil, err
			}
		}
		return ctx, nil, ErrSubscriptionAdmissionUnsupported
	}
	return context.WithValue(ctx, subscriptionFrozenKey{}, &snap), &snap, nil
}

func (p *subscriptionFrozenPricing) calculate(s *BillingService, ctx context.Context, model string, tokens UsageTokens, tier, effort string, disableModelLongContextPricing ...bool) (*CostBreakdown, error) {
	card, ok := p.Models[subscriptionPricingModelKey(model)]
	if !ok {
		return nil, ErrSubscriptionAdmissionUnsupported
	}
	resolved := card.resolved()
	if resolved == nil || (resolved.Mode != BillingModeToken && resolved.Mode != BillingModePerRequest) {
		return nil, ErrSubscriptionAdmissionUnsupported
	}
	disableModelPricing := len(disableModelLongContextPricing) > 0 && disableModelLongContextPricing[0]
	return s.CalculateCostUnified(CostInput{Ctx: ctx, Model: model, Tokens: tokens, RequestCount: 1, RateMultiplier: p.Multiplier, PricingAt: p.PricingAt,
		ServiceTier: tier, ReasoningEffort: effort, Resolver: NewModelPricingResolver(nil, s), Resolved: resolved, LongContextBillingEnabled: &p.LongContext,
		DisableModelLongContextPricing: disableModelPricing})
}

// Keep the affirmative observation durable even when a frozen pricing decision
// cannot yield a command. Recovery never guesses a replacement price.
func pendingSubscriptionPricing(ctx context.Context, reason string, cause error) error {
	if usage, ok := ctx.Value(subscriptionObservedKey{}).(string); ok {
		if err := requestledger.PersistSubscriptionUsage(ctx, requestledger.SubscriptionObservation{Usage: usage, PendingReason: reason}, nil); err != nil {
			return err
		}
	}
	return cause
}

func (p *subscriptionFrozenPricing) accountStats(s *BillingService, accountID, groupID int64, model string, tokens UsageTokens, total float64, tier, effort string, disableModelLongContextPricing ...bool) *float64 {
	if p.StatsChannel == nil || model == "" {
		return nil
	}
	if v := tryCustomRules(p.StatsChannel, accountID, groupID, p.StatsPlatform, model, tokens, 1, effort); v != nil {
		return v
	}
	if p.StatsChannel.ApplyPricingToAccountStats {
		if total <= 0 {
			return nil
		}
		return &total
	}
	card, ok := p.StatsModels[subscriptionPricingModelKey(model)]
	if !ok {
		return nil
	}
	disableModelPricing := len(disableModelLongContextPricing) > 0 && disableModelLongContextPricing[0]
	cost, err := s.CalculateCostUnified(CostInput{Ctx: context.Background(), Model: model, Tokens: tokens, RateMultiplier: 1, ServiceTier: normalizeBillingServiceTier(tier), ReasoningEffort: effort, PricingAt: p.PricingAt, Resolver: NewModelPricingResolver(nil, s), Resolved: card.resolved(), DisableModelLongContextPricing: disableModelPricing})
	if err != nil || cost == nil || cost.TotalCost <= 0 {
		return nil
	}
	return &cost.TotalCost
}

func subscriptionPricingModelKey(model string) string {
	return strings.ToLower(strings.TrimSpace(model))
}

// Legacy aliases and reusable prompts can hide media or tools until later
// normalization. Leave their original billing path intact in this first scope.
func subscriptionNativeTextRequest(body []byte) bool {
	for _, field := range []string{"messages", "functions", "prompt", "modalities", "audio"} {
		if gjson.GetBytes(body, field).Exists() {
			return false
		}
	}
	tools := gjson.GetBytes(body, "tools")
	return (!tools.Exists() || (tools.IsArray() && len(tools.Array()) == 0)) && subscriptionTextInput(gjson.GetBytes(body, "input"))
}

func subscriptionTextInput(v gjson.Result) bool {
	if v.Type == gjson.String {
		return true
	}
	if !v.IsArray() {
		return false
	}
	for _, item := range v.Array() {
		if !item.IsObject() {
			return false
		}
		typ := item.Get("type").String()
		// Native text-history state can trigger the existing compact/encryption
		// compatibility retry. It is not a tool or media input.
		if typ == "compaction" {
			if item.Get("encrypted_content").Type != gjson.String {
				return false
			}
			continue
		}
		if typ == "reasoning" {
			for _, part := range item.Get("summary").Array() {
				if part.Get("type").String() != "summary_text" || part.Get("text").Type != gjson.String {
					return false
				}
			}
			continue
		}
		if typ != "" && typ != "message" {
			return false
		}
		content := item.Get("content")
		if content.Type == gjson.String {
			continue
		}
		if !content.IsArray() {
			return false
		}
		for _, part := range content.Array() {
			if !part.IsObject() {
				return false
			}
			switch part.Get("type").String() {
			case "input_text", "output_text":
				if part.Get("text").Type != gjson.String {
					return false
				}
			default:
				return false
			}
		}
	}
	return true
}
