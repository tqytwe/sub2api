package handler

import (
	"log/slog"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ModelPlazaHandler 处理「模型广场」查询。
//
// 广场路由挂 OptionalJWT 中间件：匿名可访问（除非 require_auth 开启），带 token 则
// 识别用户。可见性规则（橱窗语义，与「可用渠道」的可绑定语义不同）：
//   - 匿名：仅非专属分组（订阅型照常展示）；
//   - 登录：非专属分组 + user_allowed_groups 授权的专属分组（不检查订阅有效性）。
type ModelPlazaHandler struct {
	channelService *service.ChannelService
	apiKeyService  *service.APIKeyService
	settingService *service.SettingService
	catalogService *service.ModelCatalogService
}

// NewModelPlazaHandler 创建模型广场 handler。
func NewModelPlazaHandler(
	channelService *service.ChannelService,
	apiKeyService *service.APIKeyService,
	settingService *service.SettingService,
	catalogServices ...*service.ModelCatalogService,
) *ModelPlazaHandler {
	var catalogService *service.ModelCatalogService
	if len(catalogServices) > 0 {
		catalogService = catalogServices[0]
	}
	return &ModelPlazaHandler{
		channelService: channelService,
		apiKeyService:  apiKeyService,
		settingService: settingService,
		catalogService: catalogService,
	}
}

// modelPlazaOfficialPricing LiteLLM 官方参考价（USD per token）。
type modelPlazaOfficialPricing struct {
	InputPrice        *float64 `json:"input_price"`
	OutputPrice       *float64 `json:"output_price"`
	CacheWritePrice   *float64 `json:"cache_write_price"`
	CacheWrite1hPrice *float64 `json:"cache_write_1h_price,omitempty"`
	CacheReadPrice    *float64 `json:"cache_read_price"`
}

// modelPlazaModel 广场模型条目：展示定价 + 官方参考价。
type modelPlazaModel struct {
	Name            string                     `json:"name"`
	Platform        string                     `json:"platform"`
	DisplayPricing  *userSupportedModelPricing `json:"display_pricing"`
	OfficialPricing *modelPlazaOfficialPricing `json:"official_pricing"`
}

// modelPlazaGroup 广场分组条目（白名单字段）。
type modelPlazaGroup struct {
	ID                 int64    `json:"id"`
	Name               string   `json:"name"`
	Description        string   `json:"description"`
	Platform           string   `json:"platform"`
	SubscriptionType   string   `json:"subscription_type"`
	RateMultiplier     float64  `json:"rate_multiplier"`
	UserRateMultiplier *float64 `json:"user_rate_multiplier,omitempty"`
	PeakRateEnabled    bool     `json:"peak_rate_enabled"`
	PeakStart          string   `json:"peak_start"`
	PeakEnd            string   `json:"peak_end"`
	PeakRateMultiplier float64  `json:"peak_rate_multiplier"`
	IsExclusive        bool     `json:"is_exclusive"`
	// 生图独立倍率：为 true 时图片计费模型的实付倍率取 ImageRateMultiplier，
	// 不取分组/用户专属倍率。
	ImageRateIndependent bool              `json:"image_rate_independent"`
	ImageRateMultiplier  float64           `json:"image_rate_multiplier"`
	Models               []modelPlazaModel `json:"models"`
}

// modelPlazaResponse 广场页响应。
type modelPlazaResponse struct {
	Description string            `json:"description"`
	Groups      []modelPlazaGroup `json:"groups"`
}

// Get 返回模型广场数据。
// GET /api/v1/model-plaza
func (h *ModelPlazaHandler) Get(c *gin.Context) {
	if h.settingService == nil {
		response.NotFound(c, "Model plaza is not enabled")
		return
	}
	rt := h.settingService.GetModelPlazaRuntime(c.Request.Context())
	if !rt.Enabled {
		response.NotFound(c, "Model plaza is not enabled")
		return
	}

	subject, authed := middleware.GetAuthSubjectFromContext(c)
	if rt.RequireAuth && !authed {
		response.Unauthorized(c, "Authentication required")
		return
	}

	if h.channelService == nil || h.catalogService == nil {
		response.NotFound(c, "Model plaza catalog is unavailable")
		return
	}
	entries, err := h.catalogService.ListCatalog(c.Request.Context(), service.CatalogListFilter{})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	groups, err := h.channelService.ListPlazaGroupsIncludingEmpty(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	catalogEntries := make(map[plazaCatalogKey]service.SiteModelCatalogEntry, len(entries))
	for i := range entries {
		e := &entries[i]
		if !e.VisiblePublic && (!authed || !e.VisibleAuth) {
			continue
		}
		catalogEntries[plazaCatalogKey{platform: normalizePlazaKey(e.Platform), model: normalizePlazaKey(e.ModelName)}] = *e
	}
	groups = appendCatalogModelsToPlazaGroups(groups, entries, authed)
	groups = filterPlazaGroupsByCatalog(groups, entries, authed)

	// allowedExclusive == nil 表示匿名；登录用户恒为非 nil（可能为空集合）。
	var allowedExclusive map[int64]struct{}
	var userRates map[int64]float64
	if authed {
		allowedExclusive, err = h.apiKeyService.GetUserAllowedGroupIDSet(c.Request.Context(), subject.UserID)
		if err != nil {
			// 可见性数据拿不到时不能静默降级成匿名视图（会错漏专属分组），直接报错。
			response.ErrorFrom(c, err)
			return
		}
		userRates, err = h.apiKeyService.GetUserGroupRates(c.Request.Context(), subject.UserID)
		if err != nil {
			// 专属倍率仅是展示增强，失败降级为分组默认倍率。
			slog.Warn("model_plaza_user_rates_failed", "error", err, "user_id", subject.UserID)
			userRates = nil
		}
	}

	visible := filterPlazaVisibleGroups(groups, allowedExclusive)

	out := make([]modelPlazaGroup, 0, len(visible))
	for i := range visible {
		groupDTO := toModelPlazaGroupDTO(&visible[i], userRates, catalogEntries)
		out = append(out, groupDTO)
	}
	response.Success(c, modelPlazaResponse{
		Description: rt.Description,
		Groups:      out,
	})
}

type plazaCatalogKey struct{ platform, model string }

func normalizePlazaKey(value string) string { return strings.ToLower(strings.TrimSpace(value)) }

func catalogEntryForPlazaModel(entries map[plazaCatalogKey]service.SiteModelCatalogEntry, model service.PlazaModel) (service.SiteModelCatalogEntry, bool) {
	entry, ok := entries[plazaCatalogKey{platform: normalizePlazaKey(model.Platform), model: normalizePlazaKey(model.Name)}]
	return entry, ok
}

func displayPricingForPlazaModel(model service.PlazaModel, entry service.SiteModelCatalogEntry, managed bool) *userSupportedModelPricing {
	pricing := toUserPricing(model.Pricing)
	if pricing == nil && managed {
		pricing = &userSupportedModelPricing{BillingMode: string(service.BillingModeToken)}
	}
	if pricing == nil {
		return nil
	}
	if pricing.BillingMode == "" {
		pricing.BillingMode = string(service.BillingModeToken)
	}
	if pricing.BillingMode == string(service.BillingModeToken) {
		if pricing.InputPrice == nil {
			pricing.InputPrice = entry.OfficialInputPrice
		}
		if pricing.OutputPrice == nil {
			pricing.OutputPrice = entry.OfficialOutputPrice
		}
		if pricing.CacheReadPrice == nil {
			pricing.CacheReadPrice = entry.OfficialCacheReadPrice
		}
		if pricing.CacheWritePrice == nil {
			pricing.CacheWritePrice = entry.OfficialCacheWritePrice
		}
		for i := range pricing.Intervals {
			interval := &pricing.Intervals[i]
			if interval.InputPrice == nil {
				interval.InputPrice = entry.OfficialInputPrice
			}
			if interval.OutputPrice == nil {
				interval.OutputPrice = entry.OfficialOutputPrice
			}
			if interval.CacheReadPrice == nil {
				interval.CacheReadPrice = entry.OfficialCacheReadPrice
			}
			if interval.CacheWritePrice == nil {
				interval.CacheWritePrice = entry.OfficialCacheWritePrice
			}
		}
	}
	return pricing
}

// appendCatalogModelsToPlazaGroups makes enabled catalog entries visible in
// their explicitly selected active groups even when no channel currently
// advertises the model. A missing channel price stays nil; this never creates
// billing data or replaces a channel-derived price.
func appendCatalogModelsToPlazaGroups(groups []service.PlazaGroup, entries []service.SiteModelCatalogEntry, authed bool) []service.PlazaGroup {
	byID := make(map[int64]int, len(groups))
	for i := range groups {
		byID[groups[i].ID] = i
	}
	seen := make(map[int]map[string]struct{}, len(groups))
	for i := range groups {
		seen[i] = make(map[string]struct{}, len(groups[i].Models))
		for _, model := range groups[i].Models {
			seen[i][strings.ToLower(strings.TrimSpace(model.Platform))+"\x00"+strings.ToLower(strings.TrimSpace(model.Name))] = struct{}{}
		}
	}
	for _, entry := range entries {
		if !entry.VisiblePublic && (!authed || !entry.VisibleAuth) {
			continue
		}
		// NULL means automatic platform association, handled below for every
		// active group. An empty array remains an explicit hide-all rule.
		groupIDs := entry.GroupIDs
		if groupIDs == nil {
			for _, group := range groups {
				if catalogEntryAllowsPlazaGroup(entry, group.ID, group.Platform, entry.Platform) {
					groupIDs = append(groupIDs, group.ID)
				}
			}
		}
		for _, groupID := range groupIDs {
			idx, ok := byID[groupID]
			if !ok {
				continue
			}
			key := strings.ToLower(strings.TrimSpace(entry.Platform)) + "\x00" + strings.ToLower(strings.TrimSpace(entry.ModelName))
			if _, exists := seen[idx][key]; exists {
				continue
			}
			groups[idx].Models = append(groups[idx].Models, service.PlazaModel{Name: entry.ModelName, Platform: entry.Platform})
			seen[idx][key] = struct{}{}
		}
	}
	return groups
}

// filterPlazaGroupsByCatalog applies the model plaza catalog's display controls
// without changing the channel pricing data used for billing.
// A nil GroupIDs value keeps the legacy platform-based association; a non-nil
// value is an explicit allow-list (including an empty list, which hides it).
func filterPlazaGroupsByCatalog(groups []service.PlazaGroup, entries []service.SiteModelCatalogEntry, authed bool) []service.PlazaGroup {
	type catalogKey struct{ platform, name string }
	byKey := make(map[catalogKey]service.SiteModelCatalogEntry, len(entries))
	for _, e := range entries {
		key := catalogKey{platform: strings.ToLower(strings.TrimSpace(e.Platform)), name: strings.ToLower(strings.TrimSpace(e.ModelName))}
		if key.name == "" {
			continue
		}
		byKey[key] = e
	}

	visible := make([]service.PlazaGroup, 0, len(groups))
	for _, group := range groups {
		models := make([]service.PlazaModel, 0, len(group.Models))
		for _, model := range group.Models {
			key := catalogKey{platform: strings.ToLower(strings.TrimSpace(model.Platform)), name: strings.ToLower(strings.TrimSpace(model.Name))}
			entry, managed := byKey[key]
			if !managed || (!entry.VisiblePublic && (!authed || !entry.VisibleAuth)) || !catalogEntryAllowsPlazaGroup(entry, group.ID, group.Platform, model.Platform) {
				continue
			}
			models = append(models, model)
		}
		if len(models) == 0 {
			continue
		}
		group.Models = models
		visible = append(visible, group)
	}
	return visible
}

func catalogEntryAllowsPlazaGroup(entry service.SiteModelCatalogEntry, groupID int64, groupPlatform, modelPlatform string) bool {
	if entry.GroupIDs != nil {
		for _, allowedID := range entry.GroupIDs {
			if allowedID == groupID {
				return true
			}
		}
		return false
	}
	group := strings.ToLower(strings.TrimSpace(groupPlatform))
	model := strings.ToLower(strings.TrimSpace(modelPlatform))
	return group == model || (group == service.PlatformAntigravity && (model == service.PlatformAnthropic || model == service.PlatformGemini))
}

// filterPlazaVisibleGroups 按登录态裁剪分组可见性。
// allowedExclusive == nil 表示匿名（仅非专属）；非 nil 表示登录（非专属 + 授权专属）。
func filterPlazaVisibleGroups(
	groups []service.PlazaGroup,
	allowedExclusive map[int64]struct{},
) []service.PlazaGroup {
	visible := make([]service.PlazaGroup, 0, len(groups))
	for _, g := range groups {
		if g.IsExclusive {
			if allowedExclusive == nil {
				continue
			}
			if _, ok := allowedExclusive[g.ID]; !ok {
				continue
			}
		}
		visible = append(visible, g)
	}
	return visible
}

// toModelPlazaGroupDTO 将 service 层广场分组映射为白名单 DTO,并合并用户专属倍率。
func toModelPlazaGroupDTO(g *service.PlazaGroup, userRates map[int64]float64, catalogMaps ...map[plazaCatalogKey]service.SiteModelCatalogEntry) modelPlazaGroup {
	catalogEntries := map[plazaCatalogKey]service.SiteModelCatalogEntry{}
	if len(catalogMaps) > 0 && catalogMaps[0] != nil {
		catalogEntries = catalogMaps[0]
	}
	models := make([]modelPlazaModel, 0, len(g.Models))
	for i := range g.Models {
		m := &g.Models[i]
		entry, managed := catalogEntryForPlazaModel(catalogEntries, *m)
		displayPricing := displayPricingForPlazaModel(*m, entry, managed)
		official := m.OfficialPricing
		if managed {
			official = &service.PlazaOfficialPricing{InputPrice: entry.OfficialInputPrice, OutputPrice: entry.OfficialOutputPrice, CacheReadPrice: entry.OfficialCacheReadPrice, CacheWritePrice: entry.OfficialCacheWritePrice}
		}
		models = append(models, modelPlazaModel{
			Name:            m.Name,
			Platform:        m.Platform,
			DisplayPricing:  displayPricing,
			OfficialPricing: toModelPlazaOfficialPricing(official),
		})
	}
	dto := modelPlazaGroup{
		ID:                   g.ID,
		Name:                 g.Name,
		Description:          g.Description,
		Platform:             g.Platform,
		SubscriptionType:     g.SubscriptionType,
		RateMultiplier:       g.RateMultiplier,
		PeakRateEnabled:      g.PeakRateEnabled,
		PeakStart:            g.PeakStart,
		PeakEnd:              g.PeakEnd,
		PeakRateMultiplier:   g.PeakRateMultiplier,
		IsExclusive:          g.IsExclusive,
		ImageRateIndependent: g.ImageRateIndependent,
		ImageRateMultiplier:  g.ImageRateMultiplier,
		Models:               models,
	}
	if rate, ok := userRates[g.ID]; ok {
		dto.UserRateMultiplier = &rate
	}
	return dto
}

// toModelPlazaOfficialPricing 转换官方参考价；nil 透传（前端显示 "-"）。
func toModelPlazaOfficialPricing(p *service.PlazaOfficialPricing) *modelPlazaOfficialPricing {
	if p == nil {
		return nil
	}
	return &modelPlazaOfficialPricing{
		InputPrice:        p.InputPrice,
		OutputPrice:       p.OutputPrice,
		CacheWritePrice:   p.CacheWritePrice,
		CacheWrite1hPrice: p.CacheWrite1hPrice,
		CacheReadPrice:    p.CacheReadPrice,
	}
}
