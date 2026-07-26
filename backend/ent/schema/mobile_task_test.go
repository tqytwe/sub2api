package schema

import (
	"testing"

	"entgo.io/ent/dialect/entsql"
	"github.com/stretchr/testify/require"
)

func TestMobileTaskSchemaContract(t *testing.T) {
	schema := MobileTask{}
	fields := schema.Fields()
	names := make([]string, 0, len(fields))
	for _, item := range fields {
		names = append(names, item.Descriptor().Name)
	}
	require.ElementsMatch(t, []string{
		"id", "user_id", "kind", "operation", "status", "progress",
		"parent_task_id", "retry_of", "client_request_id", "resource",
		"artifacts", "error", "protocol_version", "created_at", "updated_at",
		"started_at", "finished_at",
	}, names)
	require.Len(t, schema.Indexes(), 6)
	annotation, ok := schema.Annotations()[0].(entsql.Annotation)
	require.True(t, ok)
	require.Equal(t, "mobile_tasks", annotation.Table)
}
