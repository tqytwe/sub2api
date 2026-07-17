package handler

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestBuildPromptLibrarySitemapContainsOnlyProvidedPublishedPrompts(t *testing.T) {
	body, err := buildPromptLibrarySitemap("https://www.jisudeng.com", []service.PublicPrompt{
		{ID: 12},
		{ID: 34},
	})
	require.NoError(t, err)
	xml := string(body)
	require.Contains(t, xml, "<loc>https://www.jisudeng.com/prompts</loc>")
	require.Contains(t, xml, "<loc>https://www.jisudeng.com/prompts/12</loc>")
	require.Contains(t, xml, "<loc>https://www.jisudeng.com/prompts/34</loc>")
	require.False(t, strings.Contains(xml, "source_url"))
}

func TestPromptRequestOriginUsesCanonicalProductionHost(t *testing.T) {
	request := httptest.NewRequest("GET", "http://attacker.example/sitemap.xml", nil)
	request.Header.Set("X-Forwarded-Proto", "http")
	require.Equal(t, "https://www.jisudeng.com", promptRequestOrigin(request))
}
