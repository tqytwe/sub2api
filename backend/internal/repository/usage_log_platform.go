package repository

import "fmt"

// appendUsageLogPlatformCondition filters only the request-time snapshot. Historical
// unknown values stay unknown; mutable account/group joins cannot establish history.
func appendUsageLogPlatformCondition(conditions []string, args []any, platform, alias string) ([]string, []any) {
	if platform == "" {
		return conditions, args
	}
	column := alias + "platform"
	if platform == "unknown" {
		return append(conditions, "("+column+" IS NULL OR "+column+" = '')"), args
	}
	conditions = append(conditions, fmt.Sprintf("%s = $%d", column, len(args)+1))
	return conditions, append(args, platform)
}
