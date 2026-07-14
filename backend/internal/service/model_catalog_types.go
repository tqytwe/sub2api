package service

import (
	"context"
	"time"
)

// SiteModelCatalogEntry is one row in site_model_catalog.
type SiteModelCatalogEntry struct {
	ID              int64      `json:"id"`
	ModelName       string     `json:"model_name"`
	Platform        string     `json:"platform"`
	DisplayName     *string    `json:"display_name"`
	UseCase         *string    `json:"use_case"`
	SortOrder       int        `json:"sort_order"`
	VisiblePublic   bool       `json:"visible_public"`
	VisibleAuth     bool       `json:"visible_auth"`
	Featured        bool       `json:"featured"`
	InputPrice      *float64   `json:"input_price"`
	OutputPrice     *float64   `json:"output_price"`
	CacheReadPrice  *float64   `json:"cache_read_price"`
	CacheWritePrice *float64   `json:"cache_write_price"`
	BillingMode     string     `json:"billing_mode"`
	Source          string     `json:"source"`
	SourceUpdatedAt *time.Time `json:"source_updated_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
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

// MyModelPricingGroup is a user-visible group with rate multiplier.
type MyModelPricingGroup struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	RateMultiplier float64 `json:"rate_multiplier"`
}

// MyModelPricingRow is one model row for authenticated /models/my-pricing.
type MyModelPricingRow struct {
	Name                  string                `json:"name"`
	Platform              string                `json:"platform"`
	Channel               string                `json:"channel,omitempty"`
	UseCase               string                `json:"use_case,omitempty"`
	Groups                []MyModelPricingGroup `json:"groups"`
	BaseInputPrice        *float64              `json:"base_input_price"`
	BaseOutputPrice       *float64              `json:"base_output_price"`
	EffectiveInputPrice   *float64              `json:"effective_input_price"`
	EffectiveOutputPrice  *float64              `json:"effective_output_price"`
	OfficialInputPrice    *float64              `json:"official_input_price"`
	OfficialOutputPrice   *float64              `json:"official_output_price"`
}

// MyModelPricingResponse is the payload for GET /models/my-pricing.
type MyModelPricingResponse struct {
	Models             []MyModelPricingRow `json:"models"`
	RateMultiplierNote string              `json:"rate_multiplier_note"`
	Enabled            bool                `json:"enabled"`
}

// ModelCatalogRepository persists site catalog, discoveries, and sync jobs.
type ModelCatalogRepository interface {
	ListCatalog(ctx context.Context, filter CatalogListFilter) ([]SiteModelCatalogEntry, error)
	GetCatalogEntry(ctx context.Context, id int64) (*SiteModelCatalogEntry, error)
	UpsertCatalogEntry(ctx context.Context, entry *SiteModelCatalogEntry) error
	UpdateCatalogEntry(ctx context.Context, entry *SiteModelCatalogEntry) error
	DeleteCatalogEntry(ctx context.Context, id int64) error
	BatchUpdateVisibility(ctx context.Context, ids []int64, visiblePublic, visibleAuth *bool) (int, error)
	BatchUpdatePrices(ctx context.Context, ids []int64, multiplier *float64, absoluteInput, absoluteOutput *float64) (int, error)

	ListDiscoveries(ctx context.Context, status string, limit int) ([]ModelDiscovery, error)
	UpsertDiscovery(ctx context.Context, d *ModelDiscovery) error
	UpdateDiscoveryStatus(ctx context.Context, ids []int64, status string) (int, error)

	CreateSyncJob(ctx context.Context, job *ModelSyncJob) error
	UpdateSyncJob(ctx context.Context, job *ModelSyncJob) error
	GetSyncJob(ctx context.Context, id string) (*ModelSyncJob, error)
}

// CatalogListFilter filters admin catalog listing.
type CatalogListFilter struct {
	Platform       string
	VisiblePublic  *bool
	VisibleAuth    *bool
	Search         string
	Limit          int
	Offset         int
}
