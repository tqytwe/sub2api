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
		return []PublicModelPricingRow{}
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

		officialIn, officialOut := e.OfficialInputPrice, e.OfficialOutputPrice
		if officialIn == nil && officialOut == nil {
			officialIn, officialOut = lookupOfficialPrices(s.billingService, name)
		}
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
	if s.settingService != nil && !s.settingService.GetAvailableChannelsRuntime(ctx).Enabled {
		return resp, nil
	}
	resp.Enabled = true
	if userID <= 0 {
		return resp, nil
	}

	visibleAuth := true
	catalogEntries, err := s.repo.ListCatalog(ctx, CatalogListFilter{VisibleAuth: &visibleAuth})
	if err != nil {
		return nil, err
	}
	catalogByKey := make(map[string]SiteModelCatalogEntry, len(catalogEntries))
	catalogByName := make(map[string]SiteModelCatalogEntry, len(catalogEntries))
	for _, entry := range catalogEntries {
		catalogByKey[catalogSyncKey(entry.ModelName, entry.Platform)] = entry
		nameKey := strings.ToLower(strings.TrimSpace(entry.ModelName))
		if _, exists := catalogByName[nameKey]; !exists {
			catalogByName[nameKey] = entry
		}
	}

	type rowKey struct {
		model    string
		platform string
		groupID  int64
		channel  string
	}
	rowsByKey := make(map[rowKey]*MyModelPricingRow)
	representedCatalog := make(map[string]struct{}, len(catalogEntries))
	siteMultiplier := 1.0
	if s.settingService != nil {
		siteMultiplier = s.settingService.GetPublicModelRateMultiplier(ctx)
	}

	if s.channelService != nil && s.apiKeyService != nil {
		userGroups, groupsErr := s.apiKeyService.GetAvailableGroups(ctx, userID)
		channels, channelsErr := s.channelService.ListAvailable(ctx)
		if groupsErr == nil && channelsErr == nil {
			userRates, _ := s.apiKeyService.GetUserGroupRates(ctx, userID)
			allowedGroupIDs := make(map[int64]struct{}, len(userGroups))
			for _, g := range userGroups {
				allowedGroupIDs[g.ID] = struct{}{}
			}

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
					catalogEntry, catalogVisible := catalogByKey[catalogSyncKey(sm.Name, platform)]
					if !catalogVisible {
						catalogEntry, catalogVisible = catalogByName[strings.ToLower(strings.TrimSpace(sm.Name))]
					}
					if !catalogVisible {
						continue
					}
					grps := platformGroups[strings.ToLower(strings.TrimSpace(platform))]
					if len(grps) == 0 {
						grps = visibleGroups
					}
					officialIn, officialOut := catalogOfficialPrices(s.billingService, catalogEntry)
					siteIn, siteOut := catalogSitePrices(catalogEntry, officialIn, officialOut, siteMultiplier)
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
							SortOrder:            catalogEntry.SortOrder,
							Channel:              ch.Name,
							Groups:               []MyModelPricingGroup{{ID: g.ID, Name: g.Name, RateMultiplier: mult}},
							BaseInputPrice:       baseIn,
							BaseOutputPrice:      baseOut,
							EffectiveInputPrice:  scalePricePtr(baseIn, mult),
							EffectiveOutputPrice: scalePricePtr(baseOut, mult),
							OfficialInputPrice:   officialIn,
							OfficialOutputPrice:  officialOut,
							SiteInputPrice:       siteIn,
							SiteOutputPrice:      siteOut,
							UseCase:              derefCatalogString(catalogEntry.UseCase),
						}
						representedCatalog[catalogSyncKey(catalogEntry.ModelName, catalogEntry.Platform)] = struct{}{}
					}
				}
			}
		}
	}

	for _, catalogEntry := range catalogEntries {
		catalogKey := catalogSyncKey(catalogEntry.ModelName, catalogEntry.Platform)
		if _, represented := representedCatalog[catalogKey]; represented {
			continue
		}
		officialIn, officialOut := catalogOfficialPrices(s.billingService, catalogEntry)
		siteIn, siteOut := catalogSitePrices(catalogEntry, officialIn, officialOut, siteMultiplier)
		key := rowKey{model: catalogEntry.ModelName, platform: catalogEntry.Platform}
		rowsByKey[key] = &MyModelPricingRow{
			Name:                catalogEntry.ModelName,
			Platform:            displayPlatformName(catalogEntry.Platform),
			SortOrder:           catalogEntry.SortOrder,
			Groups:              []MyModelPricingGroup{},
			OfficialInputPrice:  officialIn,
			OfficialOutputPrice: officialOut,
			SiteInputPrice:      siteIn,
			SiteOutputPrice:     siteOut,
			UseCase:             derefCatalogString(catalogEntry.UseCase),
		}
	}

	out := make([]MyModelPricingRow, 0, len(rowsByKey))
	for _, row := range rowsByKey {
		out = append(out, *row)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].SortOrder != out[j].SortOrder {
			return out[i].SortOrder < out[j].SortOrder
		}
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		if out[i].Platform != out[j].Platform {
			return out[i].Platform < out[j].Platform
		}
		leftGroupID, rightGroupID := int64(0), int64(0)
		if len(out[i].Groups) > 0 {
			leftGroupID = out[i].Groups[0].ID
		}
		if len(out[j].Groups) > 0 {
			rightGroupID = out[j].Groups[0].ID
		}
		return leftGroupID < rightGroupID
	})
	resp.Models = out
	return resp, nil
}

func catalogOfficialPrices(billingService *BillingService, entry SiteModelCatalogEntry) (*float64, *float64) {
	officialIn, officialOut := entry.OfficialInputPrice, entry.OfficialOutputPrice
	if officialIn == nil && officialOut == nil {
		officialIn, officialOut = lookupOfficialPrices(billingService, entry.ModelName)
	}
	return officialIn, officialOut
}

func catalogSitePrices(entry SiteModelCatalogEntry, officialIn, officialOut *float64, defaultMultiplier float64) (*float64, *float64) {
	siteIn, siteOut := entry.InputPrice, entry.OutputPrice
	if siteIn == nil {
		siteIn = scalePricePtr(officialIn, defaultMultiplier)
	}
	if siteOut == nil {
		siteOut = scalePricePtr(officialOut, defaultMultiplier)
	}
	return siteIn, siteOut
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
		key := strings.ToLower(strings.TrimSpace(g.Platform))
		if key == "" {
			continue
		}
		m[key] = append(m[key], g)
	}
	return m
}

// --- Admin catalog CRUD ---

func (s *ModelCatalogService) ListCatalog(ctx context.Context, filter CatalogListFilter) ([]SiteModelCatalogEntry, error) {
	return s.repo.ListCatalog(ctx, filter)
}

// ListAdminCatalog returns catalog rows enriched with official and channel prices for admin comparison UI.
func (s *ModelCatalogService) ListAdminCatalog(ctx context.Context, filter CatalogListFilter) ([]AdminCatalogRow, error) {
	entries, err := s.repo.ListCatalog(ctx, filter)
	if err != nil {
		return nil, err
	}
	channelBase := collectChannelBasePricesFromService(ctx, s.channelService)
	out := make([]AdminCatalogRow, 0, len(entries))
	for _, e := range entries {
		officialIn, officialOut := e.OfficialInputPrice, e.OfficialOutputPrice
		if officialIn == nil && officialOut == nil {
			officialIn, officialOut = lookupOfficialPrices(s.billingService, e.ModelName)
		}
		var chIn, chOut *float64
		if base, ok := channelBase[strings.ToLower(e.ModelName)]; ok {
			chIn, chOut = base.input, base.output
		}
		out = append(out, AdminCatalogRow{
			SiteModelCatalogEntry: e,
			ChannelInputPrice:     chIn,
			ChannelOutputPrice:    chOut,
		})
		out[len(out)-1].OfficialInputPrice = officialIn
		out[len(out)-1].OfficialOutputPrice = officialOut
	}
	return out, nil
}

func (s *ModelCatalogService) GetCatalogEntry(ctx context.Context, id int64) (*SiteModelCatalogEntry, error) {
	return s.repo.GetCatalogEntry(ctx, id)
}

func (s *ModelCatalogService) SaveCatalogEntry(ctx context.Context, entry *SiteModelCatalogEntry) error {
	if entry == nil || strings.TrimSpace(entry.ModelName) == "" {
		return fmt.Errorf("model name is required")
	}
	if hasNegativeCatalogPrice(entry) {
		return fmt.Errorf("model prices cannot be negative")
	}
	if entry.ID > 0 {
		existing, err := s.repo.GetCatalogEntry(ctx, entry.ID)
		if err != nil {
			return err
		}
		if existing == nil {
			return fmt.Errorf("catalog entry not found: %d", entry.ID)
		}
		entry.OfficialInputPrice = existing.OfficialInputPrice
		entry.OfficialOutputPrice = existing.OfficialOutputPrice
		entry.OfficialCacheReadPrice = existing.OfficialCacheReadPrice
		entry.OfficialCacheWritePrice = existing.OfficialCacheWritePrice
		entry.OfficialSource = existing.OfficialSource
		entry.OfficialUpdatedAt = existing.OfficialUpdatedAt
	}
	if entry.PriceMultiplier != nil {
		if *entry.PriceMultiplier <= 0 {
			return fmt.Errorf("price multiplier must be greater than zero")
		}
		entry.InputPrice = scalePricePtr(entry.OfficialInputPrice, *entry.PriceMultiplier)
		entry.OutputPrice = scalePricePtr(entry.OfficialOutputPrice, *entry.PriceMultiplier)
		entry.CacheReadPrice = scalePricePtr(entry.OfficialCacheReadPrice, *entry.PriceMultiplier)
		entry.CacheWritePrice = scalePricePtr(entry.OfficialCacheWritePrice, *entry.PriceMultiplier)
	}
	entry.ModelName = strings.TrimSpace(entry.ModelName)
	entry.Platform = strings.ToLower(strings.TrimSpace(entry.Platform))
	entry.Source = "manual"
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

func (s *ModelCatalogService) ListDiscoveries(ctx context.Context, filter DiscoveryListFilter) (DiscoveryListResult, error) {
	if filter.Status == "" {
		filter.Status = "new"
	}
	return s.repo.ListDiscoveries(ctx, filter)
}

func (s *ModelCatalogService) ImportDiscoveries(ctx context.Context, ids []int64, toCatalog bool, siteMultiplier *float64) (int, error) {
	if len(ids) == 0 {
		return 0, fmt.Errorf("ids required: select discoveries to import")
	}
	discoveries, err := s.repo.ListDiscoveriesByIDs(ctx, ids)
	if err != nil {
		return 0, err
	}
	imported := 0
	importedIDs := make([]int64, 0, len(ids))
	for _, d := range discoveries {
		if !toCatalog {
			continue
		}
		useCase := ""
		if v, ok := d.Payload["use_case"].(string); ok {
			useCase = v
		}
		now := time.Now()
		entry := &SiteModelCatalogEntry{
			ModelName:         d.ModelName,
			Platform:          d.Platform,
			DisplayName:       catalogOptionalString(payloadString(d.Payload, "display_name")),
			UseCase:           catalogOptionalString(useCase),
			VisiblePublic:     false,
			VisibleAuth:       true,
			Source:            d.Source,
			OfficialSource:    d.Source,
			OfficialUpdatedAt: &now,
		}
		entry.OfficialInputPrice, entry.OfficialOutputPrice,
			entry.OfficialCacheReadPrice, entry.OfficialCacheWritePrice = pricesFromDiscoveryPayload(d.Payload)
		if siteMultiplier != nil {
			if *siteMultiplier <= 0 {
				return imported, fmt.Errorf("site multiplier must be greater than zero")
			}
			entry.PriceMultiplier = siteMultiplier
			entry.InputPrice = scalePricePtr(entry.OfficialInputPrice, *siteMultiplier)
			entry.OutputPrice = scalePricePtr(entry.OfficialOutputPrice, *siteMultiplier)
			entry.CacheReadPrice = scalePricePtr(entry.OfficialCacheReadPrice, *siteMultiplier)
			entry.CacheWritePrice = scalePricePtr(entry.OfficialCacheWritePrice, *siteMultiplier)
		}
		if err := s.repo.UpsertDiscoveryCatalogEntry(ctx, entry); err != nil {
			if len(importedIDs) > 0 {
				_, _ = s.repo.UpdateDiscoveryStatus(ctx, importedIDs, "imported")
			}
			return imported, err
		}
		imported++
		importedIDs = append(importedIDs, d.ID)
	}
	if len(importedIDs) > 0 {
		if _, err := s.repo.UpdateDiscoveryStatus(ctx, importedIDs, "imported"); err != nil {
			return imported, err
		}
	}
	return imported, nil
}

func pricesFromDiscoveryPayload(payload map[string]any) (*float64, *float64, *float64, *float64) {
	return payloadFloat(payload, "input_price"), payloadFloat(payload, "output_price"),
		payloadFloat(payload, "cache_read_price"), payloadFloat(payload, "cache_write_price")
}

func payloadFloat(payload map[string]any, key string) *float64 {
	if v, ok := payload[key].(float64); ok {
		value := v
		return &value
	}
	return nil
}

func payloadString(payload map[string]any, key string) string {
	v, _ := payload[key].(string)
	return strings.TrimSpace(v)
}

func derefCatalogString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func hasNegativeCatalogPrice(entry *SiteModelCatalogEntry) bool {
	prices := []*float64{entry.InputPrice, entry.OutputPrice, entry.CacheReadPrice, entry.CacheWritePrice}
	for _, price := range prices {
		if price != nil && *price < 0 {
			return true
		}
	}
	return false
}

func catalogOptionalString(s string) *string {
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

// Repo exposes the repository for sync runner (same package).
func (s *ModelCatalogService) Repo() ModelCatalogRepository {
	return s.repo
}

func (s *ModelCatalogService) ChannelService() *ChannelService {
	return s.channelService
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
