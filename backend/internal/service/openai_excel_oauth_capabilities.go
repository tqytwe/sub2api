package service

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// ExcelBPSCapability is the safe, public projection of one model returned by
// the Excel/BPS access contract. It deliberately excludes the bearer token and
// the provider response envelope.
type ExcelBPSCapability struct {
	ID               string   `json:"id"`
	DisplayName      string   `json:"display_name,omitempty"`
	ReasoningEfforts []string `json:"reasoning_efforts,omitempty"`
}

// ExcelBPSCapabilities is the allowlist used by callers before selecting an
// Excel-only account. An unavailable or malformed response never creates an
// allowlist.
type ExcelBPSCapabilities struct {
	Allowed bool                 `json:"allowed"`
	Models  []ExcelBPSCapability `json:"models"`
}

var excelBPSCapabilityModelID = regexp.MustCompile(`^gpt-[a-z0-9]+(?:[._-][a-z0-9]+)*$`)

// ParseExcelBPSCapabilities validates and projects the official
// /basispoints/api/responses/access response. Unknown fields are ignored, and
// restricted, disabled, malformed, and duplicate models are omitted.
func ParseExcelBPSCapabilities(raw []byte) (ExcelBPSCapabilities, error) {
	var envelope struct {
		Allowed *bool `json:"allowed"`
		Catalog *struct {
			Models     *[]json.RawMessage `json:"models"`
			Restricted []string           `json:"restricted_models"`
		} `json:"model_catalog"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || envelope.Allowed == nil {
		return ExcelBPSCapabilities{}, fmt.Errorf("invalid Excel BPS access response")
	}
	result := ExcelBPSCapabilities{Allowed: *envelope.Allowed, Models: []ExcelBPSCapability{}}
	if !result.Allowed {
		return result, nil
	}
	if envelope.Catalog == nil || envelope.Catalog.Models == nil {
		return ExcelBPSCapabilities{}, fmt.Errorf("missing Excel BPS model catalog")
	}
	restricted := make(map[string]struct{}, len(envelope.Catalog.Restricted))
	for _, id := range envelope.Catalog.Restricted {
		restricted[strings.TrimSpace(id)] = struct{}{}
	}
	seen := make(map[string]struct{}, len(*envelope.Catalog.Models))
	for _, rawModel := range *envelope.Catalog.Models {
		var entry struct {
			ID        string `json:"id"`
			Label     string `json:"label"`
			Available *bool  `json:"available"`
			Enabled   *bool  `json:"enabled"`
			Picker    *bool  `json:"model_picker_enabled"`
			Policy    *struct {
				State string `json:"state"`
			} `json:"policy"`
			Efforts []struct {
				Value string `json:"value"`
			} `json:"efforts"`
		}
		if json.Unmarshal(rawModel, &entry) != nil {
			continue
		}
		id := strings.TrimSpace(entry.ID)
		if !excelBPSCapabilityModelID.MatchString(id) {
			continue
		}
		if _, ok := restricted[id]; ok {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		if entry.Available != nil && !*entry.Available || entry.Enabled != nil && !*entry.Enabled || entry.Picker != nil && !*entry.Picker {
			continue
		}
		if entry.Policy != nil && (entry.Policy.State == "disabled" || entry.Policy.State == "unconfigured") {
			continue
		}
		efforts := make([]string, 0, len(entry.Efforts))
		effortSeen := make(map[string]struct{}, len(entry.Efforts))
		for _, effort := range entry.Efforts {
			value := strings.TrimSpace(effort.Value)
			switch value {
			case "low", "medium", "high", "xhigh":
				if _, ok := effortSeen[value]; !ok {
					efforts = append(efforts, value)
					effortSeen[value] = struct{}{}
				}
			}
		}
		model := ExcelBPSCapability{ID: id, DisplayName: strings.TrimSpace(entry.Label), ReasoningEfforts: efforts}
		result.Models = append(result.Models, model)
		seen[id] = struct{}{}
	}
	return result, nil
}

// AllowsModel answers only from the explicit server capability response.
func (c ExcelBPSCapabilities) AllowsModel(model string) bool {
	model = strings.TrimSpace(model)
	if !c.Allowed || model == "" {
		return false
	}
	for _, candidate := range c.Models {
		if candidate.ID == model {
			return true
		}
	}
	return false
}
