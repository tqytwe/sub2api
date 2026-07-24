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
	for _, path := range []string{"/", "/models", "/docs", "/en/", "/en/models", "/en/docs", "/about", "/contact", "/download/android", "/image-studio"} {
		require.Contains(t, xml, "<loc>https://www.jisudeng.com"+path+"</loc>")
	}
	require.Contains(t, xml, `xmlns:xhtml="http://www.w3.org/1999/xhtml"`)
	require.Contains(t, xml, `<changefreq>daily</changefreq>`)
	require.Contains(t, xml, `<priority>1.00</priority>`)
	require.Contains(t, xml, `<xhtml:link rel="alternate" hreflang="en" href="https://www.jisudeng.com/en/models"></xhtml:link>`)
	require.Contains(t, xml, `<xhtml:link rel="alternate" hreflang="zh-CN" href="https://www.jisudeng.com/models"></xhtml:link>`)
	require.NotContains(t, xml, "<loc>https://www.jisudeng.com/home</loc>")
	require.False(t, strings.Contains(xml, "source_url"))
}

func TestPromptRequestOriginUsesCanonicalProductionHost(t *testing.T) {
	request := httptest.NewRequest("GET", "http://attacker.example/sitemap.xml", nil)
	request.Header.Set("X-Forwarded-Proto", "http")
	require.Equal(t, "https://www.jisudeng.com", promptRequestOrigin(request))
}

func TestBuildRobotsTxtAdvertisesSitemapAndKeepsPrivateAPIsOut(t *testing.T) {
	robots := buildRobotsTxt("https://www.jisudeng.com")

	require.Contains(t, robots, "User-agent: *")
	require.Contains(t, robots, "Allow: /")
	require.Contains(t, robots, "Allow: /llms.txt")
	require.Contains(t, robots, "User-agent: OAI-SearchBot")
	require.Contains(t, robots, "User-agent: PerplexityBot")
	require.Contains(t, robots, "User-agent: Baiduspider")
	require.Contains(t, robots, "Content-Signal: search=yes,ai-input=yes,ai-train=no,use=reference")
	require.Contains(t, robots, "Disallow: /api/")
	require.Contains(t, robots, "Disallow: /v1/")
	require.Contains(t, robots, "User-agent: GPTBot\nDisallow: /")
	require.Contains(t, robots, "Sitemap: https://www.jisudeng.com/sitemap.xml")
	require.Contains(t, robots, "LLMs: https://www.jisudeng.com/llms.txt")
}

func TestBuildLLMSTxtExposesBilingualAIReferenceSummary(t *testing.T) {
	body := buildLLMSTxt("https://www.jisudeng.com")

	require.Contains(t, body, "# Jisudeng")
	require.Contains(t, body, "Access DeepSeek, Qwen, Kimi, GLM")
	require.Contains(t, body, "https://www.jisudeng.com/en/models")
	require.Contains(t, body, "https://www.jisudeng.com/docs")
	require.Contains(t, body, "## AI Search Reference Policy")
	require.Contains(t, body, "## Common Questions")
	require.Contains(t, body, "Chinese public routes are the default")
	require.Contains(t, body, "中文摘要")
	require.NotContains(t, body, "Chinese AI")
	require.NotContains(t, body, "China")
}
