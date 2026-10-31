package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestAPIKeyGroupsMigrationIsAdditiveAndIndexed(t *testing.T) {
	b, err := os.ReadFile("291_api_key_groups.sql")
	if err != nil { t.Fatal(err) }
	s := string(b)
	for _, want := range []string{"CREATE TABLE IF NOT EXISTS api_key_groups", "ON DELETE CASCADE", "PRIMARY KEY (api_key_id, group_id)", "idx_api_key_groups_group_id"} {
		if !strings.Contains(s, want) { t.Fatalf("migration missing %q", want) }
	}
	if strings.Contains(s, "DROP TABLE") || strings.Contains(s, "ALTER TABLE api_keys DROP") {
		t.Fatal("migration must remain additive")
	}
}
