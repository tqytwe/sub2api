package service

import (
	"context"
	"encoding/json"
	"time"
)

// SiteModelCatalogEntry is one row in site_model_catalog.
type SiteModelCatalogEntry struct {
	ID               int64                        `json:"id"`
	ModelName        string                       `json:"model_name"`
	Platform         string                       `json:"platform"`
	DisplayName      *string                      `json:"display_name"`
	UseCase          *string                      `json:"use_case"`
	SortOrder        int                          `json:"sort_order"`
	VisiblePublic    bool                         `json:"visible_public"`
	VisibleAuth      bool                         `json:"visible_auth"`
	Featured         bool                         `json:"featured"`
	GroupIDs         []int64                      `json:"group_ids"`
	ToolCapabilities ModelToolCapabilityOverrides `json:"tool_capabilities"`
	// MediaCapabilities remains raw so extensions introduced by an upstream
	// provider survive catalog edits. Nil represents an unreviewed legacy row.
	MediaCapabilities        json.RawMessage `json:"media_capabilities"`
	OfficialInputPrice       *float64        `json:"official_input_price"`
	OfficialOutputPrice      *float64        `json:"official_output_price"`
	OfficialCacheReadPrice   *float64        `json:"official_cache_read_price"`
	OfficialCacheWritePrice  *float64        `json:"official_cache_write_price"`
	OfficialSource           string          `json:"official_source"`
	OfficialUpdatedAt        *time.Time      `json:"official_updated_at"`
	OfficialInputManual      bool            `json:"official_input_manual"`
	OfficialOutputManual     bool            `json:"official_output_manual"`
	OfficialCacheReadManual  bool            `json:"official_cache_read_manual"`
	OfficialCacheWriteManual bool            `json:"official_cache_write_manual"`
	PriceMultiplier          *float64        `json:"price_multiplier"`
	InputPrice               *float64        `json:"input_price"`
	OutputPrice              *float64        `json:"output_price"`
	CacheReadPrice           *float64        `json:"cache_read_price"`
	CacheWritePrice          *float64        `json:"cache_write_price"`
	BillingMode              string          `json:"billing_mode"`
	Source                   string          `json:"source"`
	SourceUpdatedAt          *time.Time      `json:"source_updated_at"`
	CreatedAt                time.Time       `json:"created_at"`
	UpdatedAt                time.Time       `json:"updated_at"`
}

// ModelToolCapabilityOverrides records an administrator's explicit capability
// declaration. Nil means "not declared" and may use exact upstream metadata;
// false is a deliberate denial and must override any upstream metadata.
//
// These fields are intentionally pointers because a boolean cannot distinguish
// an operator's explicit false from an unset legacy catalog row.
type ModelToolCapabilityOverrides struct {
	FunctionCalling *bool `json:"function_calling,omitempty"`
	ToolChoice      *bool `json:"tool_choice,omitempty"`
	WebSearch       *bool `json:"web_search,omitempty"`
	Live            *bool `json:"live,omitempty"`
}

// ModelDiscovery is a newly discovered model from an online pricing source.
type ModelDiscovery struct {
	ID           int64          `json:"id"`
	ModelName    string         `json:"model_name"`
	Platform     string         `json:"platform"`
	Source       string         `json:"source"`
	Payload      map[string]any `json:"payload"`
	Status       string         `json:"status"`
	DiscoveredAt time.Time      `json:"discovered_at"`
}

// ModelSyncJob tracks an async pricing sync run.
type ModelSyncJob struct {
	ID          string         `json:"id"`
	Kind        string         `json:"kind"`
	Status      string         `json:"status"`
	Result      map[string]any `json:"result,omitempty"`
	Error       string         `json:"error,omitempty"`
	StartedAt   time.Time      `json:"started_at"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
}

// ModelSyncResult is stored in model_sync_jobs.result.
type ModelSyncResult struct {
	Updated    int      `json:"updated"`
	Discovered int      `json:"discovered"`
	Retired    int      `json:"retired"`
	Warnings   []string `json:"warnings,omitempty"`
	Source     string   `json:"source"`
}

// NextChatDisplayGroup is internal NextChat display metadata.
type NextChatDisplayGroup struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	RateMultiplier float64 `json:"rate_multiplier"`
}

// NextChatDisplayModel is one internal NextChat model metadata row.
type NextChatDisplayModel struct {
	Name                 string                 `json:"name"`
	Platform             string                 `json:"platform"`
	SortOrder            int                    `json:"sort_order"`
	Channel              string                 `json:"channel,omitempty"`
	UseCase              string                 `json:"use_case,omitempty"`
	Groups               []NextChatDisplayGroup `json:"groups"`
	BaseInputPrice       *float64               `json:"base_input_price"`
	BaseOutputPrice      *float64               `json:"base_output_price"`
	EffectiveInputPrice  *float64               `json:"effective_input_price"`
	EffectiveOutputPrice *float64               `json:"effective_output_price"`
	OfficialInputPrice   *float64               `json:"official_input_price"`
	OfficialOutputPrice  *float64               `json:"official_output_price"`
	SiteInputPrice       *float64               `json:"site_input_price"`
	SiteOutputPrice      *float64               `json:"site_output_price"`
}

// NextChatDisplayMetadata is an internal, non-HTTP display payload.
type NextChatDisplayMetadata struct {
	Models             []NextChatDisplayModel `json:"models"`
	RateMultiplierNote string                 `json:"rate_multiplier_note"`
	Enabled            bool                   `json:"enabled"`
}

// ModelCatalogRepository persists site catalog, discoveries, and sync jobs.
type ModelCatalogRepository interface {
	ListCatalog(ctx context.Context, filter CatalogListFilter) ([]SiteModelCatalogEntry, error)
	GetCatalogEntry(ctx context.Context, id int64) (*SiteModelCatalogEntry, error)
	UpsertCatalogEntry(ctx context.Context, entry *SiteModelCatalogEntry) error
	UpsertDiscoveryCatalogEntry(ctx context.Context, entry *SiteModelCatalogEntry) error
	UpdateCatalogEntry(ctx context.Context, entry *SiteModelCatalogEntry) error
	DeleteCatalogEntry(ctx context.Context, id int64) error
	BatchUpdateVisibility(ctx context.Context, ids []int64, visiblePublic, visibleAuth *bool) (int, error)
	BatchUpdateGroups(ctx context.Context, ids []int64, groupIDs []int64) (int, error)
	UpdateCatalogOfficialPrices(ctx context.Context, modelName, platform, source string, input, output, cacheRead, cacheWrite *float64, updatedAt time.Time) (int, error)

	ListDiscoveries(ctx context.Context, filter DiscoveryListFilter) (DiscoveryListResult, error)
	ListDiscoveriesByIDs(ctx context.Context, ids []int64) ([]ModelDiscovery, error)
	UpsertDiscovery(ctx context.Context, d *ModelDiscovery) error
	UpdateDiscoveryStatus(ctx context.Context, ids []int64, status string) (int, error)

	CreateSyncJob(ctx context.Context, job *ModelSyncJob) error
	UpdateSyncJob(ctx context.Context, job *ModelSyncJob) error
	GetSyncJob(ctx context.Context, id string) (*ModelSyncJob, error)
}

// AdminCatalogRow is the catalog entry shape used by the admin comparison UI.
type AdminCatalogRow struct {
	SiteModelCatalogEntry
}

// DiscoveryListFilter filters discovery pool listing.
type DiscoveryListFilter struct {
	Status string
	Search string
	Limit  int
	Offset int
}

// DiscoveryListResult is a paginated discovery pool response.
type DiscoveryListResult struct {
	Items []ModelDiscovery `json:"items"`
	Total int              `json:"total"`
}

// CatalogListFilter filters admin catalog listing.
type CatalogListFilter struct {
	Platform      string
	VisiblePublic *bool
	VisibleAuth   *bool
	Search        string
	Limit         int
	Offset        int
}
