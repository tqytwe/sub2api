package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMobileAppReleasesMigrationProtectsReleaseInvariants(t *testing.T) {
	content, err := FS.ReadFile("253_mobile_app_releases.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(content))
	for _, fragment := range []string{
		"create table if not exists mobile_app_releases",
		"distribution in ('direct', 'play')",
		"artifact_type in ('apk', 'aab')",
		"distribution = 'direct' and artifact_type = 'apk'",
		"distribution = 'play' and artifact_type = 'aab'",
		"unique (distribution, version_code)",
		"notes_i18n jsonb",
		"storage_key text not null",
	} {
		require.Contains(t, sql, fragment)
	}
}
