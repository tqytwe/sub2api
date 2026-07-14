package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ModelCatalogService manages site model catalog, public/login pricing views, and sync jobs.
type ModelCatalogService struct {
	repo           ModelCatalogRepository
	channelService *ChannelService
	pricingService *PricingService
	billingService *BillingService
	settingService *SettingService
	apiKeyService  *APIKeyService
	syncRunner     *ModelPricingSyncRunner
}

// NewModelCatalogService wires the catalog service.
func NewModelCatalogService(
	repo ModelCatalogRepository,
	channelService *ChannelService,
	pricingService *PricingService,
	billingService *BillingService,
	settingService *SettingService,
	apiKeyService *APIKeyService,
) *ModelCatalogService {
	svc := &ModelCatalogService{
		repo:           repo,
		channelService: channelService,
		pricingService: pricingService,
		billingService: billingService,
		settingService: settingService,
		apiKeyService:  apiKeyService,
	}
	svc.syncRunner = NewModelPricingSyncRunner(svc)
	return svc
}

// ListPublicPricing returns guest-visible catalog rows with official and reference prices.
func (s *ModelCatalogService) ListPublicPricing(ctx context.Context) []PublicModelPricingRow {
	if s.settingService != nil {
		rt := s.settingService.GetPlayRuntime(ctx)
		if !rt.PublicModelsEnabled {
			return []PublicModelPricingRow{}
		}
	}
	visible := true
	entries, err := s.repo.ListCatalog(ctx, CatalogListFilter{VisiblePublic: &visible})
	if err != nil || len(entries) == 0 {
		return legacyPublicPricing(ctx, s)
	}

	mult := 1.0
	if s.settingService != nil {
		mult = s.settingService.GetPublicModelRateMultiplier(ctx)
	}

	channelBase := collectChannelBasePricesFromService(ctx, s.channelService)
	out := make([]PublicModelPricingRow, 0, len(entries))
	for _, e := range entries {
		name := e.ModelName
		platformLabel := displayPlatformName(e.Platform)
		useCase := "chat"
		if e.UseCase != nil && *e.UseCase != "" {
			useCase = *e.UseCase
		}

		officialIn, officialOut := lookupOfficialPrices(s.billingService, name)
		ourIn, ourOut := scalePricePtr(officialIn, mult), scalePricePtr(officialOut, mult)

		if e.InputPrice != nil {
			ourIn = e.InputPrice
		} else if base, ok := channelBase[strings.ToLower(name)]; ok && base.input != nil {
			ourIn = base.input
		}
		if e.OutputPrice != nil {
			ourOut = e.OutputPrice
		} else if base, ok := channelBase[strings.ToLower(name)]; ok && base.output != nil {
			ourOut = base.output
		}

		out = append(out, PublicModelPricingRow{
			Name:                name,
			Platform:            platformLabel,
			UseCase:             useCase,
			OfficialInputPrice:  officialIn,
			OfficialOutputPrice: officialOut,
			OurInputPrice:       ourIn,
			OurOutputPrice:      ourOut,
			RateMultiplier:      mult,
		})
	}
	return out
}

func legacyPublicPricing(ctx context.Context, s *ModelCatalogService) []PublicModelPricingRow {
	play := &PlayService{channelService: s.channelService, settingService: s.settingService}
	return play.ListPublicModelPricing(ctx, s.billingService)
}

func collectChannelBasePricesFromService(ctx context.Context, channelService *ChannelService) map[string]channelPricePair {
	play := &PlayService{channelService: channelService}
	return play.collectChannelBasePrices(ctx)
}

// ListMyPricing aggregates user-visible models with effective prices per group.
func (s *ModelCatalogService) ListMyPricing(ctx context.Context, userID int64) (*MyModelPricingResponse, error) {
	resp := &MyModelPricingResponse{
		Models:             []MyModelPricingRow{},
		RateMultiplierNote: "实付价 = 渠道价 × 分组倍率",
		Enabled:            false,
	}
	if s.settingService == nil || !s.settingService.GetAvailableChannelsRuntime(ctx).Enabled {
		return resp, nil
	}
	resp.Enabled = true
	if s.channelService == nil || s.apiKeyService == nil || userID <= 0 {
		return resp, nil
	}

	userGroups, err := s.apiKeyService.GetAvailableGroups(ctx, userID)
	if err != nil {
		return nil, err
	}
	userRates, _ := s.apiKeyService.GetUserGroupRates(ctx, userID)

	allowedGroupIDs := make(map[int64]struct{}, len(userGroups))
	groupByID := make(map[int64]Group, len(userGroups))
	for _, g := range userGroups {
		allowedGroupIDs[g.ID] = struct{}{}
		groupByID[g.ID] = g
	}

	channels, err := s.channelService.ListAvailable(ctx)
	if err != nil {
		return nil, err
	}

	type rowKey struct {
		model    string
		platform string
		groupID  int64
		channel  string
	}
	rowsByKey := make(map[rowKey]*MyModelPricingRow)

	for _, ch := range channels {
		if ch.Status != StatusActive {
			continue
		}
		visibleGroups := filterAvailableGroupsForUser(ch.Groups, allowedGroupIDs)
		if len(visibleGroups) == 0 {
			continue
		}
		platformGroups := groupRefsByPlatform(visibleGroups)
		for _, sm := range ch.SupportedModels {
			if sm.Name == "" {
				continue
			}
			platform := sm.Platform
			grps := platformGroups[platform]
			if len(grps) == 0 {
				continue
			}
			officialIn, officialOut := lookupOfficialPrices(s.billingService, sm.Name)
			var baseIn, baseOut *float64
			if sm.Pricing != nil {
				baseIn, baseOut = sm.Pricing.InputPrice, sm.Pricing.OutputPrice
			}
			for _, g := range grps {
				mult := g.RateMultiplier
				if userRates != nil {
					if ur, ok := userRates[g.ID]; ok && ur > 0 {
						mult = ur
					}
				}
				key := rowKey{model: sm.Name, platform: platform, groupID: g.ID, channel: ch.Name}
				rowsByKey[key] = &MyModelPricingRow{
					Name:                 sm.Name,
					Platform:             displayPlatformName(platform),
					Channel:              ch.Name,
					Groups:               []MyModelPricingGroup{{ID: g.ID, Name: g.Name, RateMultiplier: mult}},
					BaseInputPrice:       baseIn,
					BaseOutputPrice:      baseOut,
					EffectiveInputPrice:  scalePricePtr(baseIn, mult),
					EffectiveOutputPrice: scalePricePtr(baseOut, mult),
					OfficialInputPrice:   officialIn,
					OfficialOutputPrice:  officialOut,
				}
			}
		}
	}

	keys := make([]rowKey, 0, len(rowsByKey))
	for k := range rowsByKey {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].model != keys[j].model {
			return keys[i].model < keys[j].model
		}
		if keys[i].platform != keys[j].platform {
			return keys[i].platform < keys[j].platform
		}
		return keys[i].groupID < keys[j].groupID
	})
	out := make([]MyModelPricingRow, 0, len(keys))
	for _, k := range keys {
		out = append(out, *rowsByKey[k])
	}
	resp.Models = out
	return resp, nil
}

func filterAvailableGroupsForUser(groups []AvailableGroupRef, allowed map[int64]struct{}) []AvailableGroupRef {
	out := make([]AvailableGroupRef, 0, len(groups))
	for _, g := range groups {
		if _, ok := allowed[g.ID]; ok {
			out = append(out, g)
		}
	}
	return out
}

func groupRefsByPlatform(groups []AvailableGroupRef) map[string][]AvailableGroupRef {
	m := make(map[string][]AvailableGroupRef)
	for _, g := range groups {
		if g.Platform == "" {
			continue
		}
		m[g.Platform] = append(m[g.Platform], g)
	}
	return m
}

// --- Admin catalog CRUD ---

func (s *ModelCatalogService) ListCatalog(ctx context.Context, filter CatalogListFilter) ([]SiteModelCatalogEntry, error) {
	return s.repo.ListCatalog(ctx, filter)
}

func (s *ModelCatalogService) GetCatalogEntry(ctx context.Context, id int64) (*SiteModelCatalogEntry, error) {
	return s.repo.GetCatalogEntry(ctx, id)
}

func (s *ModelCatalogService) SaveCatalogEntry(ctx context.Context, entry *SiteModelCatalogEntry) error {
	if entry.ID > 0 {
		return s.repo.UpdateCatalogEntry(ctx, entry)
	}
	return s.repo.UpsertCatalogEntry(ctx, entry)
}

func (s *ModelCatalogService) DeleteCatalogEntry(ctx context.Context, id int64) error {
	return s.repo.DeleteCatalogEntry(ctx, id)
}

func (s *ModelCatalogService) BatchVisibility(ctx context.Context, ids []int64, visiblePublic, visibleAuth *bool) (int, error) {
	return s.repo.BatchUpdateVisibility(ctx, ids, visiblePublic, visibleAuth)
}

func (s *ModelCatalogService) BatchPrices(ctx context.Context, ids []int64, multiplier *float64, absoluteInput, absoluteOutput *float64) (int, error) {
	return s.repo.BatchUpdatePrices(ctx, ids, multiplier, absoluteInput, absoluteOutput)
}

func (s *ModelCatalogService) ListDiscoveries(ctx context.Context, status string) ([]ModelDiscovery, error) {
	return s.repo.ListDiscoveries(ctx, status, 500)
}

func (s *ModelCatalogService) ImportDiscoveries(ctx context.Context, ids []int64, toCatalog bool) (int, error) {
	discoveries, err := s.repo.ListDiscoveries(ctx, "new", 1000)
	if err != nil {
		return 0, err
	}
	idSet := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		idSet[id] = struct{}{}
	}
	imported := 0
	for _, d := range discoveries {
		if len(idSet) > 0 {
			if _, ok := idSet[d.ID]; !ok {
				continue
			}
		}
		if !toCatalog {
			continue
		}
		useCase := ""
		if v, ok := d.Payload["use_case"].(string); ok {
			useCase = v
		}
		entry := &SiteModelCatalogEntry{
			ModelName:     d.ModelName,
			Platform:      d.Platform,
			UseCase:       strPtr(useCase),
			VisiblePublic: false,
			VisibleAuth:   true,
			Source:        d.Source,
		}
		if in, out := pricesFromDiscoveryPayload(d.Payload); in != nil || out != nil {
			entry.InputPrice = in
			entry.OutputPrice = out
		}
		if err := s.repo.UpsertCatalogEntry(ctx, entry); err != nil {
			return imported, err
		}
		imported++
	}
	if imported > 0 {
		_, _ = s.repo.UpdateDiscoveryStatus(ctx, ids, "imported")
	}
	return imported, nil
}

func pricesFromDiscoveryPayload(payload map[string]any) (*float64, *float64) {
	var inPtr, outPtr *float64
	if v, ok := payload["input_price"].(float64); ok {
		inPtr = &v
	}
	if v, ok := payload["output_price"].(float64); ok {
		outPtr = &v
	}
	return inPtr, outPtr
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// StartSyncJob creates an async pricing sync job.
func (s *ModelCatalogService) StartSyncJob(ctx context.Context) (*ModelSyncJob, error) {
	id, err := newSyncJobID()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	job := &ModelSyncJob{
		ID:        id,
		Kind:      "pricing_refresh",
		Status:    "running",
		StartedAt: now,
	}
	if err := s.repo.CreateSyncJob(ctx, job); err != nil {
		return nil, err
	}
	go s.syncRunner.Run(context.WithoutCancel(ctx), job.ID)
	return job, nil
}

func (s *ModelCatalogService) GetSyncJob(ctx context.Context, id string) (*ModelSyncJob, error) {
	return s.repo.GetSyncJob(ctx, id)
}

func newSyncJobID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// FillCatalogFromLiteLLM applies LiteLLM official prices to catalog rows missing overrides.
func (s *ModelCatalogService) FillCatalogFromLiteLLM(ctx context.Context, ids []int64) (int, error) {
	entries, err := s.repo.ListCatalog(ctx, CatalogListFilter{})
	if err != nil {
		return 0, err
	}
	idSet := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		idSet[id] = struct{}{}
	}
	updated := 0
	now := time.Now()
	for _, e := range entries {
		if len(idSet) > 0 {
			if _, ok := idSet[e.ID]; !ok {
				continue
			}
		}
		if s.pricingService == nil {
			continue
		}
		p := s.pricingService.GetModelPricing(e.ModelName)
		if p == nil {
			continue
		}
		var in, out float64
		if p.InputCostPerToken > 0 {
			in = p.InputCostPerToken
			e.InputPrice = &in
		}
		if p.OutputCostPerToken > 0 {
			out = p.OutputCostPerToken
			e.OutputPrice = &out
		}
		e.Source = "litellm"
		e.SourceUpdatedAt = &now
		if err := s.repo.UpdateCatalogEntry(ctx, &e); err != nil {
			return updated, err
		}
		updated++
	}
	return updated, nil
}

// Repo exposes the repository for sync runner (same package).
func (s *ModelCatalogService) Repo() ModelCatalogRepository {
	return s.repo
}

func (s *ModelCatalogService) ChannelService() *ChannelService {
	return s.channelService
}

func (s *ModelCatalogService) PricingService() *PricingService {
	return s.pricingService
}

// SyncComplete updates a job after sync.
func (s *ModelCatalogService) SyncComplete(ctx context.Context, job *ModelSyncJob) error {
	return s.repo.UpdateSyncJob(ctx, job)
}

// UpsertDiscovery delegates to repo.
func (s *ModelCatalogService) UpsertDiscovery(ctx context.Context, d *ModelDiscovery) error {
	return s.repo.UpsertDiscovery(ctx, d)
}

// CatalogKeys returns existing catalog keys for sync dedup.
func (s *ModelCatalogService) CatalogKeys(ctx context.Context) (map[string]struct{}, error) {
	type keyLister interface {
		ListCatalogModelKeys(ctx context.Context) (map[string]struct{}, error)
	}
	if kl, ok := s.repo.(keyLister); ok {
		return kl.ListCatalogModelKeys(ctx)
	}
	entries, err := s.repo.ListCatalog(ctx, CatalogListFilter{})
	if err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		out[fmt.Sprintf("%s::%s", strings.ToLower(e.ModelName), strings.ToLower(e.Platform))] = struct{}{}
	}
	return out, nil
}
