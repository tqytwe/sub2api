package service

import "context"

const QualityActionIsolateModels = "isolate_models"
const QualityModelIsolationKey = "quality_model_isolation"

// Private to trusted internal account probes. No request header or API payload
// can grant this bypass; normal traffic must retain every isolation restriction.
type qualityIsolationProbeKey struct{}

func (a *Account) IsQualityModelIsolated(model string) bool {
	if a == nil {
		return false
	}
	entries, _ := a.Extra[QualityModelIsolationKey].(map[string]any)
	return entries[model] != nil || entries[a.GetMappedModel(model)] != nil
}

func (a *Account) qualityModelBlocked(ctx context.Context, model string) bool {
	if ctx != nil && ctx.Value(qualityIsolationProbeKey{}) == true {
		return false
	}
	return a.IsQualityModelIsolated(model)
}

// Outbound identities have already been mapped. Mapping one a second time could
// incorrectly block a different upstream in an alias chain.
func (a *Account) qualityOutboundModelBlocked(ctx context.Context, requestModel, outboundModel string) bool {
	if a == nil || ctx != nil && ctx.Value(qualityIsolationProbeKey{}) == true {
		return false
	}
	entries, _ := a.Extra[QualityModelIsolationKey].(map[string]any)
	return entries[requestModel] != nil || entries[outboundModel] != nil
}
