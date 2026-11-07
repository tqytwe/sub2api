package repository

import "github.com/Wei-Shaw/sub2api/internal/service"

// Quality ownership changes use their fenced transaction, never ordinary
// account Extra input (including stale edits and account duplication).
func stripQualityIsolationFromExtra(extra map[string]any) map[string]any {
	if _, exists := extra[service.QualityModelIsolationKey]; !exists {
		return extra
	}
	clean := make(map[string]any, len(extra)-1)
	for key, value := range extra {
		if key != service.QualityModelIsolationKey {
			clean[key] = value
		}
	}
	return clean
}
