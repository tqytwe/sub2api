package repository

import (
	"strings"
	"testing"
)

func TestExternalIntegrationConfig(t *testing.T) {
	validDSN := "postgres://sub2api_test@127.0.0.1:55432/sub2api_test_migration?sslmode=disable"
	for _, tc := range []struct {
		name, mode, dsn, redis string
		wantEnabled, wantError bool
	}{
		{"default docker", "", "", "", false, false},
		{"local isolated", "local", validDSN, "127.0.0.1:56379", true, false},
		{"ipv6 loopback", "local", "postgres://sub2api_test@[::1]:55432/sub2api_test_migration?sslmode=disable", "[::1]:56379", true, false},
		{"implicit mode", "", validDSN, "127.0.0.1:56379", false, true},
		{"unknown mode", "remote", validDSN, "127.0.0.1:56379", false, true},
		{"missing postgres", "local", "", "127.0.0.1:56379", false, true},
		{"remote postgres", "local", "postgres://sub2api_test@example.com:55432/sub2api_test_migration?sslmode=disable", "127.0.0.1:56379", false, true},
		{"production database", "local", "postgres://sub2api_test@127.0.0.1:55432/production?sslmode=disable", "127.0.0.1:56379", false, true},
		{"wrong user", "local", "postgres://admin@127.0.0.1:55432/sub2api_test_migration?sslmode=disable", "127.0.0.1:56379", false, true},
		{"password rejected", "local", "postgres://sub2api_test:secret@127.0.0.1:55432/sub2api_test_migration?sslmode=disable", "127.0.0.1:56379", false, true},
		{"host override", "local", validDSN + "&host=example.com", "127.0.0.1:56379", false, true},
		{"duplicate sslmode", "local", validDSN + "&sslmode=require", "127.0.0.1:56379", false, true},
		{"missing port", "local", "postgres://sub2api_test@127.0.0.1/sub2api_test_migration?sslmode=disable", "127.0.0.1:56379", false, true},
		{"encoded host override", "local", validDSN + "&%68ost=example.com", "127.0.0.1:56379", false, true},
		{"missing database", "local", "postgres://sub2api_test@127.0.0.1:55432?sslmode=disable", "127.0.0.1:56379", false, true},
		{"fragment", "local", validDSN + "#ignored", "127.0.0.1:56379", false, true},
		{"oversized database", "local", "postgres://sub2api_test@127.0.0.1:55432/sub2api_test_" + strings.Repeat("a", 64) + "?sslmode=disable", "127.0.0.1:56379", false, true},
		{"redis zero port", "local", validDSN, "127.0.0.1:0", false, true},
		{"redis out of range", "local", validDSN, "127.0.0.1:65536", false, true},
		{"redis localhost name", "local", validDSN, "localhost:56379", false, true},
		{"remote redis", "local", validDSN, "example.com:6379", false, true},
		{"redis missing port", "local", validDSN, "127.0.0.1", false, true},
		{"redis standard port", "local", validDSN, "127.0.0.1:6379", false, true},
		{"postgres standard port", "local", "postgres://sub2api_test@127.0.0.1:5432/sub2api_test_migration?sslmode=disable", "127.0.0.1:56379", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseExternalIntegrationConfig(tc.mode, tc.dsn, tc.redis)
			if (err != nil) != tc.wantError {
				t.Fatalf("error presence = %v, want %v", err != nil, tc.wantError)
			}
			if got.enabled != tc.wantEnabled {
				t.Fatalf("enabled = %v, want %v", got.enabled, tc.wantEnabled)
			}
		})
	}
}

func TestExternalIntegrationConfigDoesNotDiscloseSuppliedCredentials(t *testing.T) {
	_, err := parseExternalIntegrationConfig("local", "postgres://private-user:private-password@private-host/private-db", "private-redis:6379")
	if err == nil {
		t.Fatal("expected invalid configuration")
	}
	if strings.Contains(err.Error(), "private-") {
		t.Fatal("configuration error disclosed supplied data")
	}
}
