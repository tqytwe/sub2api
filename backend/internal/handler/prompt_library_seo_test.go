package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
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
	for _, path := range []string{
		"/", "/pricing", "/pricing/deepseek", "/pricing/qwen", "/pricing/kimi", "/pricing/glm",
		"/docs", "/en/", "/en/models", "/en/models/deepseek", "/en/models/qwen",
		"/en/models/kimi", "/en/models/glm", "/en/docs", "/about", "/contact", "/en/about", "/en/contact",
		"/download/android", "/image-studio",
	} {
		require.Contains(t, xml, "<loc>https://www.jisudeng.com"+path+"</loc>")
	}
	require.Contains(t, xml, `xmlns:xhtml="http://www.w3.org/1999/xhtml"`)
	require.Contains(t, xml, `<changefreq>daily</changefreq>`)
	require.Contains(t, xml, `<priority>1.00</priority>`)
	require.Contains(t, xml, `<xhtml:link rel="alternate" hreflang="en" href="https://www.jisudeng.com/en/models"></xhtml:link>`)
	require.Contains(t, xml, `<xhtml:link rel="alternate" hreflang="zh-CN" href="https://www.jisudeng.com/pricing"></xhtml:link>`)
	require.Contains(t, xml, `<xhtml:link rel="alternate" hreflang="en" href="https://www.jisudeng.com/en/models/deepseek"></xhtml:link>`)
	require.Contains(t, xml, `<xhtml:link rel="alternate" hreflang="zh-CN" href="https://www.jisudeng.com/pricing/deepseek"></xhtml:link>`)
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
	require.Contains(t, robots, "Allow: /llms-full.txt")
	require.Contains(t, robots, "Allow: /llms.small-txt")
	require.Contains(t, robots, "Allow: /.well-known/ai.txt")
	require.Contains(t, robots, "User-agent: OAI-SearchBot")
	require.Contains(t, robots, "User-agent: PerplexityBot")
	require.Contains(t, robots, "User-agent: Baiduspider")
	require.Contains(t, robots, "Content-Signal: search=yes,ai-input=yes,ai-train=no,use=reference")
	require.Contains(t, robots, "Disallow: /api/")
	require.Contains(t, robots, "Disallow: /v1/")
	require.Contains(t, robots, "User-agent: GPTBot\nDisallow: /")
	require.Contains(t, robots, "Sitemap: https://www.jisudeng.com/sitemap.xml")
	require.Contains(t, robots, "LLMs: https://www.jisudeng.com/llms.txt")
	require.Contains(t, robots, "LLMs-Full: https://www.jisudeng.com/llms-full.txt")
	require.Contains(t, robots, "LLMs-Small: https://www.jisudeng.com/llms.small-txt")
	require.Contains(t, robots, "AI-Policy: https://www.jisudeng.com/.well-known/ai.txt")
}

func TestRobotsAndLLMSTxtUseRevalidationHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &PromptLibraryHandler{}

	for _, tc := range []struct {
		name     string
		path     string
		callFunc func(*gin.Context)
	}{
		{name: "robots", path: "/robots.txt", callFunc: handler.Robots},
		{name: "llms", path: "/llms.txt", callFunc: handler.LLMSTxt},
		{name: "llms-full", path: "/llms-full.txt", callFunc: handler.LLMSFullTxt},
		{name: "llms-small", path: "/llms.small-txt", callFunc: handler.LLMSSmallTxt},
		{name: "ai", path: "/.well-known/ai.txt", callFunc: handler.AITxt},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodGet, tc.path, nil)

			tc.callFunc(c)

			require.Equal(t, "no-cache, max-age=0, must-revalidate", recorder.Header().Get("Cache-Control"))
			require.Equal(t, "index, follow", recorder.Header().Get("X-Robots-Tag"))
		})
	}
}

func TestHomeRedirectsToCanonicalRoot(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/home", nil)

	(&PromptLibraryHandler{}).HomeRedirect(c)

	require.Equal(t, http.StatusMovedPermanently, recorder.Code)
	require.Equal(t, "/", recorder.Header().Get("Location"))
}

func TestBuildLLMSTxtExposesBilingualAIReferenceSummary(t *testing.T) {
	body := buildLLMSTxt("https://www.jisudeng.com")

	require.Contains(t, body, "# Jisudeng")
	require.Contains(t, body, "Access DeepSeek, Qwen, Kimi, GLM")
	require.Contains(t, body, "https://www.jisudeng.com/en/models")
	require.Contains(t, body, "https://www.jisudeng.com/en/models/deepseek")
	require.Contains(t, body, "https://www.jisudeng.com/en/models/qwen")
	require.Contains(t, body, "https://www.jisudeng.com/en/models/kimi")
	require.Contains(t, body, "https://www.jisudeng.com/en/models/glm")
	require.Contains(t, body, "https://www.jisudeng.com/docs")
	require.Contains(t, body, "## AI Search Reference Policy")
	require.Contains(t, body, "## Common Questions")
	require.Contains(t, body, "Chinese public routes are the default")
	require.Contains(t, body, "中文摘要")
	require.NotContains(t, body, "Chinese AI")
	require.NotContains(t, body, "China")
}

func TestBuildExtendedAIReferenceFilesExposePublicTextOnlyGuidance(t *testing.T) {
	full := buildLLMSFullTxt("https://www.jisudeng.com")
	small := buildLLMSSmallTxt("https://www.jisudeng.com")
	ai := buildAITxt("https://www.jisudeng.com")

	require.Contains(t, full, "https://www.jisudeng.com/en/about")
	require.Contains(t, full, "https://www.jisudeng.com/en/contact")
	require.Contains(t, full, "Access and Privacy Boundaries")
	require.Contains(t, small, "https://www.jisudeng.com/llms-full.txt")
	require.Contains(t, ai, "https://www.jisudeng.com/llms.small-txt")
	for _, body := range []string{full, small, ai} {
		require.NotContains(t, body, "<!doctype html>")
		require.NotContains(t, body, "<html")
	}
}
