package service

import "context"

type oauthAutomationManualKey struct{}
type OAuthAutomationManualEdit struct{ Concurrency, Models, Scheduling bool }

func WithOAuthAutomationManualEdit(ctx context.Context, concurrency, models, scheduling bool) context.Context {
	return context.WithValue(ctx, oauthAutomationManualKey{}, OAuthAutomationManualEdit{concurrency, models, scheduling})
}
func OAuthAutomationManualEditFrom(ctx context.Context) OAuthAutomationManualEdit {
	v, _ := ctx.Value(oauthAutomationManualKey{}).(OAuthAutomationManualEdit)
	return v
}

// A token refresh is not a model-policy edit.
func OAuthAutomationModelsEdited(credentials map[string]any) bool {
	for _, key := range []string{"model_mapping", "model_mapping_mode", "compact_model_mapping", OAuthInitialModelSelectionKey} {
		if _, ok := credentials[key]; ok {
			return true
		}
	}
	return false
}
