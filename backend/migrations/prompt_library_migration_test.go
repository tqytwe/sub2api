package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPromptLibraryMigrationContract(t *testing.T) {
	content, err := FS.ReadFile("199_prompt_library.sql")
	require.NoError(t, err)

	sql := strings.ToUpper(strings.Join(strings.Fields(string(content)), " "))

	for _, table := range []string{
		"PROMPT_CATEGORIES",
		"PROMPTS",
		"PROMPT_VERSIONS",
		"PROMPT_CATEGORY_LINKS",
		"PROMPT_MEDIA",
		"PROMPT_SOURCES",
		"PROMPT_FAVORITES",
		"PROMPT_USE_EVENTS",
		"PROMPT_IMPORT_JOBS",
		"PROMPT_IMPORT_ITEMS",
		"PROMPT_REVIEW_RECORDS",
		"PROMPT_REPORTS",
	} {
		require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS "+table)
	}

	require.Contains(t, sql, "CHECK (STATUS IN ('DRAFT', 'PENDING_REVIEW', 'PUBLISHED', 'OFFLINE'))")
	require.Contains(t, sql, "CHECK (BRAND_TYPE IN ('ORIGINAL', 'AUTHORIZED', 'CURATED', 'COMMUNITY'))")
	require.Contains(t, sql, "CHECK (DIMENSION IN ('PURPOSE', 'STYLE', 'SUBJECT', 'MODEL', 'SIZE'))")
	require.Contains(t, sql, "BRAND_TYPE <> 'ORIGINAL'")
	require.Contains(t, sql, "SOURCE_EVIDENCE_VERIFIED")
	require.Contains(t, sql, "AUTHORIZATION_STATUS = 'ORIGINAL'")
	require.Contains(t, sql, "REFERENCE_REQUIREMENT IN ('NONE', 'OPTIONAL', 'REQUIRED')")
	require.Contains(t, sql, "FOREIGN KEY (PROMPT_ID, VERSION) REFERENCES PROMPT_VERSIONS(PROMPT_ID, VERSION)")

	require.Contains(t, sql, "UNIQUE (PROMPT_ID, USER_ID)")
	require.Contains(t, sql, "UNIQUE (PROMPT_ID, VERSION)")
	require.Contains(t, sql, "UNIQUE (SOURCE_KEY, EXTERNAL_ID)")
	require.Contains(t, sql, "UNIQUE (NORMALIZED_HASH)")

	require.Contains(t, sql, "ALTER TABLE IMAGE_STUDIO_JOBS ADD COLUMN IF NOT EXISTS PROMPT_ID BIGINT")
	require.Contains(t, sql, "ALTER TABLE IMAGE_STUDIO_JOBS ADD COLUMN IF NOT EXISTS PROMPT_VERSION INT")
	require.Contains(t, sql, "ALTER TABLE IMAGE_STUDIO_JOBS ADD COLUMN IF NOT EXISTS MODEL TEXT")
	require.Contains(t, sql, "ALTER TABLE IMAGE_STUDIO_JOBS ADD COLUMN IF NOT EXISTS QUALITY TEXT")
	require.Contains(t, sql, "FOREIGN KEY (PROMPT_ID, PROMPT_VERSION)")
	require.Contains(t, sql, "REFERENCES PROMPT_VERSIONS(PROMPT_ID, VERSION)")
}

func TestPromptLibraryMigrationAvoidsDestructiveEvidenceCleanup(t *testing.T) {
	content, err := FS.ReadFile("199_prompt_library.sql")
	require.NoError(t, err)

	sql := strings.ToUpper(strings.Join(strings.Fields(string(content)), " "))
	for _, destructive := range []string{
		"DROP TABLE",
		"TRUNCATE",
		"DELETE FROM PROMPT_SOURCES",
		"ON DELETE CASCADE REFERENCES PROMPTS",
	} {
		require.NotContains(t, sql, destructive)
	}
}

func TestPromptLibrarySeedMigrationKeepsExternalContentInReview(t *testing.T) {
	content, err := FS.ReadFile("200_prompt_library_seed.sql")
	require.NoError(t, err)

	sql := strings.ToUpper(strings.Join(strings.Fields(string(content)), " "))

	for _, category := range []string{
		"PROFILE-AVATAR",
		"SOCIAL-MEDIA-POST",
		"INFOGRAPHIC-EDU-VISUAL",
		"ECOMMERCE-MAIN-IMAGE",
		"GAME-ASSET",
		"APP-WEB-DESIGN",
		"FREE-CREATION",
		"PACKAGING-DESIGN",
		"INTERIOR-REDESIGN",
		"VIRTUAL-TRY-ON",
		"LOCAL-LIFE-MAP",
		"HEALTHCARE-VISUAL",
		"AD-CREATIVE",
		"LIVE-COMMERCE",
		"MOCKUP-SCENE",
		"DOCUMENT-PRESENTATION",
		"PUBLIC-SERVICE",
		"INDUSTRIAL-MANUFACTURING",
		"CYBERPUNK-SCI-FI",
		"LUXURY-EDITORIAL",
		"FLAT-VECTOR",
		"POSTER-TYPOGRAPHY",
		"TEXT-TYPOGRAPHY",
		"MACHINERY-PART",
		"GPT-IMAGE-2",
		"1024X1536",
	} {
		require.Contains(t, sql, category)
	}

	require.Contains(t, sql, "JISUDENG-GPT-IMAGE-2-CURATED-SEED-20260717")
	require.Contains(t, sql, "PROMPT_IMPORT_JOBS")
	require.Contains(t, sql, "PROMPT_IMPORT_ITEMS")
	require.Contains(t, sql, "'PENDING_REVIEW'")
	require.Contains(t, sql, "'CURATED'")
	require.Contains(t, sql, "SOURCE_HEAD")
	require.Contains(t, sql, "51A031C7874169621688D77ADB98B5EFD20DD10D")
	require.Contains(t, sql, "ON CONFLICT DO NOTHING")
	require.NotContains(t, sql, "INSERT INTO PROMPTS")
	require.NotContains(t, sql, "STATUS, BRAND_TYPE")
	require.NotContains(t, sql, "'PUBLISHED'")
}

func TestPromptLibraryPublicSeedMigrationPublishesCuratedContent(t *testing.T) {
	content, err := FS.ReadFile("201_prompt_library_public_seed.sql")
	require.NoError(t, err)

	sql := strings.ToUpper(strings.Join(strings.Fields(string(content)), " "))

	require.Contains(t, sql, "JISUDENG-GPT-IMAGE-2-CURATED-SEED-20260717")
	require.Contains(t, sql, "PROMPT-REVIEWER@JISUDENG.LOCAL")
	require.Contains(t, sql, "INSERT INTO PROMPTS")
	require.Contains(t, sql, "INSERT INTO PROMPT_VERSIONS")
	require.Contains(t, sql, "INSERT INTO PROMPT_CATEGORY_LINKS")
	require.Contains(t, sql, "INSERT INTO PROMPT_MEDIA")
	require.Contains(t, sql, "INSERT INTO PROMPT_SOURCES")
	require.Contains(t, sql, "INSERT INTO PROMPT_REVIEW_RECORDS")
	require.Contains(t, sql, "STATUS = 'PUBLISHED'")
	require.Contains(t, sql, "PUBLISHED_VERSION = 1")
	require.Contains(t, sql, "'CURATED'")
	require.Contains(t, sql, "'EXTERNAL'")
	require.Contains(t, sql, "极速蹬整理、翻译并完成模型适配")
	require.Contains(t, sql, "STATUS = 'APPROVED'")
	require.Contains(t, sql, "ON CONFLICT (PROMPT_ID, VERSION) DO UPDATE")
	require.NotContains(t, sql, "'ORIGINAL'")
}
