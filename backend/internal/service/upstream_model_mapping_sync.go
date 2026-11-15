package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"strings"
	"time"
)

const (
	UpstreamModelSyncExtraKey = "upstream_model_sync"
	upstreamModelSyncInterval = 5 * time.Minute
	upstreamModelSyncTimeout  = time.Minute
)

// UpstreamModelMappingSyncRepository keeps automatic reads bounded across
// processes and protects an in-flight discovery from overwriting a credential
// rotation or a concurrent model-mapping edit.
type UpstreamModelMappingSyncRepository interface {
	ClaimAutomaticUpstreamModelSync(ctx context.Context, accountID int64, now, next time.Time) (bool, error)
	UpdateUpstreamModelMappingIfUnchanged(ctx context.Context, expected *Account, mapping map[string]string) (bool, error)
}

type UpstreamModelSyncOptions struct {
	Apply     bool `json:"apply"`
	Automatic bool `json:"automatic"`
}

// SyncUpstreamModelCatalogWithOptions preserves the original discovery-only API
// unless Apply is explicitly requested. An automatic attempt claims its window
// before any upstream request; failures therefore cannot cause a polling storm.
func (s *AccountTestService) SyncUpstreamModelCatalogWithOptions(ctx context.Context, account *Account, options UpstreamModelSyncOptions) (*UpstreamModelCatalog, error) {
	if options.Automatic && !options.Apply {
		return nil, newUpstreamModelSyncConfigError("Automatic model sync requires apply=true", nil)
	}
	if !options.Apply {
		return s.SyncUpstreamModelCatalog(ctx, account)
	}
	if account == nil || account.ID <= 0 {
		return nil, newUpstreamModelSyncConfigError("A saved account is required to apply upstream models", nil)
	}
	// Credential shadows and explicit OAuth alias mode have a narrower account
	// policy than their parent's full catalog; do not broaden that policy.
	if account.IsCredentialShadow() || (account.Platform == PlatformOpenAI && account.Type == AccountTypeOAuth && account.GetCredential("model_mapping_mode") == "aliases") {
		return skippedUpstreamModelSync("account_policy", nil), nil
	}
	// Legacy or administrator-edited OAuth mappings are explicit allowlists.
	// Continue refreshing only a catalog-owned mapping that is still unchanged.
	existingMapping := rawUpstreamModelMapping(account)
	if options.Automatic && account.Type == AccountTypeOAuth && len(existingMapping) > 0 && !hasManagedUpstreamModelMapping(account, existingMapping) {
		return skippedUpstreamModelSync("account_policy", nil), nil
	}
	if options.Automatic && (account.Status != StatusActive || !account.Schedulable) {
		return skippedUpstreamModelSync("inactive", nil), nil
	}
	repo, ok := s.accountRepo.(UpstreamModelMappingSyncRepository)
	if !ok {
		return nil, newUpstreamModelSyncInternalError("Upstream model mapping persistence is not configured", nil)
	}

	var nextSyncAt *time.Time
	if options.Automatic {
		now := time.Now().UTC()
		// The persisted gate has whole-second resolution. Round up so another
		// instance cannot admit an attempt fractionally before five minutes.
		next := time.Unix(now.Add(upstreamModelSyncInterval).Unix()+1, 0).UTC()
		claimed, err := repo.ClaimAutomaticUpstreamModelSync(ctx, account.ID, now, next)
		if err != nil {
			return nil, newUpstreamModelSyncInternalError("Failed to claim automatic model sync", err)
		}
		if !claimed {
			latest, err := s.accountRepo.GetByID(ctx, account.ID)
			if err != nil {
				return nil, newUpstreamModelSyncInternalError("Failed to read automatic model sync state", err)
			}
			return skippedUpstreamModelSync("throttled", upstreamModelNextSyncAt(latest)), nil
		}
		nextSyncAt = &next
	}

	ctx, cancel := context.WithTimeout(ctx, upstreamModelSyncTimeout)
	defer cancel()
	catalog, err := s.SyncUpstreamModelCatalog(ctx, account)
	if err != nil {
		return nil, err
	}
	if catalog == nil {
		return skippedUpstreamModelSync("empty", nextSyncAt), nil
	}
	catalog.NextSyncAt = nextSyncAt
	if catalog.DiscoverySource != "upstream" {
		catalog.Skipped = "unverified_catalog"
		return catalog, nil
	}
	models := dedupeAndSortModelIDs(catalog.Models)
	if len(models) == 0 {
		catalog.Skipped = "empty"
		return catalog, nil
	}
	mapping := reconciledUpstreamModelMapping(rawUpstreamModelMapping(account), models)
	if len(mapping) == 0 {
		catalog.Skipped = "empty"
		return catalog, nil
	}
	changed := !reflect.DeepEqual(existingMapping, mapping)
	if !changed && hasManagedUpstreamModelMapping(account, mapping) {
		catalog.Applied = true
		return catalog, nil
	}
	applied, err := repo.UpdateUpstreamModelMappingIfUnchanged(ctx, account, mapping)
	if err != nil {
		return nil, newUpstreamModelSyncInternalError("Failed to apply upstream model mapping", err)
	}
	if !applied {
		catalog.Skipped = "account_changed"
		return catalog, nil
	}
	catalog.Applied = true
	catalog.Changed = changed
	return catalog, nil
}

// UpstreamModelMappingDigest binds catalog ownership to the exact persisted
// mapping. map[string]string has a canonical JSON encoding with sorted keys.
func UpstreamModelMappingDigest(mapping map[string]string) string {
	raw, _ := json.Marshal(mapping)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func hasManagedUpstreamModelMapping(account *Account, mapping map[string]string) bool {
	state, _ := account.Extra[UpstreamModelSyncExtraKey].(map[string]any)
	digest, _ := state["managed_mapping_sha256"].(string)
	return digest != "" && digest == UpstreamModelMappingDigest(mapping)
}

func skippedUpstreamModelSync(reason string, next *time.Time) *UpstreamModelCatalog {
	return &UpstreamModelCatalog{Models: []string{}, Skipped: reason, NextSyncAt: next}
}

func upstreamModelNextSyncAt(account *Account) *time.Time {
	if account == nil {
		return nil
	}
	state, _ := account.Extra[UpstreamModelSyncExtraKey].(map[string]any)
	var seconds int64
	switch value := state["next_attempt_unix"].(type) {
	case float64:
		seconds = int64(value)
	case int64:
		seconds = value
	case json.Number:
		seconds, _ = value.Int64()
	}
	if seconds <= 0 {
		return nil
	}
	next := time.Unix(seconds, 0).UTC()
	return &next
}

// Read the stored declaration, not GetModelMapping's generated compatibility
// aliases or provider defaults. Only administrator-authored redirects survive.
func rawUpstreamModelMapping(account *Account) map[string]string {
	mapping := make(map[string]string)
	switch raw := account.Credentials["model_mapping"].(type) {
	case map[string]any:
		for source, value := range raw {
			if target, ok := value.(string); ok {
				mapping[source] = target
			}
		}
	case map[string]string:
		for source, target := range raw {
			mapping[source] = target
		}
	}
	return mapping
}

func reconciledUpstreamModelMapping(existing map[string]string, models []string) map[string]string {
	result := make(map[string]string, len(models))
	live := make(map[string]bool, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model != "" && !strings.Contains(model, "*") {
			result[model] = model
			live[model] = true
		}
	}
	for source, target := range existing {
		if source == target || strings.TrimSpace(source) == "" {
			continue
		}
		if live[target] {
			result[source] = target
		}
	}
	return result
}
