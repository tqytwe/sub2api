package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBuildPromptLibrarySitemapContainsOnlyExistingPublicPages(t *testing.T) {
	body, err := buildPromptLibrarySitemap("https://www.jisudeng.com")
	require.NoError(t, err)
	xml := string(body)
	require.NotContains(t, xml, "/prompts")
	for _, path := range []string{
		"/", "/catalog", "/catalog/deepseek", "/catalog/qwen", "/catalog/kimi", "/catalog/glm",
		"/docs", "/en/", "/en/catalog", "/en/catalog/deepseek", "/en/catalog/qwen",
		"/en/catalog/kimi", "/en/catalog/glm", "/en/docs", "/about", "/contact", "/en/about", "/en/contact",
		"/download/android",
	} {
		require.Contains(t, xml, "<loc>https://www.jisudeng.com"+path+"</loc>")
	}
	require.Contains(t, xml, `xmlns:xhtml="http://www.w3.org/1999/xhtml"`)
	require.Contains(t, xml, `<changefreq>daily</changefreq>`)
	require.Contains(t, xml, `<priority>1.00</priority>`)
	require.Contains(t, xml, `<xhtml:link rel="alternate" hreflang="en" href="https://www.jisudeng.com/en/catalog"></xhtml:link>`)
	require.Contains(t, xml, `<xhtml:link rel="alternate" hreflang="zh-CN" href="https://www.jisudeng.com/catalog"></xhtml:link>`)
	require.Contains(t, xml, `<xhtml:link rel="alternate" hreflang="x-default" href="https://www.jisudeng.com/catalog"></xhtml:link>`)
	require.Contains(t, xml, `<xhtml:link rel="alternate" hreflang="en" href="https://www.jisudeng.com/en/catalog/deepseek"></xhtml:link>`)
	require.Contains(t, xml, `<xhtml:link rel="alternate" hreflang="zh-CN" href="https://www.jisudeng.com/catalog/deepseek"></xhtml:link>`)
	require.Contains(t, xml, `<xhtml:link rel="alternate" hreflang="x-default" href="https://www.jisudeng.com/catalog/deepseek"></xhtml:link>`)
	require.NotContains(t, xml, "<loc>https://www.jisudeng.com/home</loc>")
	require.NotContains(t, xml, "<loc>https://www.jisudeng.com/models</loc>")
	require.NotContains(t, xml, "<loc>https://www.jisudeng.com/ai-creation-space</loc>")
	require.NotContains(t, xml, "/en/models")
	require.NotContains(t, xml, "/models/")
	require.False(t, strings.Contains(xml, "source_url"))
}

func TestPromptSitemapStaticPathsUseCanonicalCatalogAndChineseDefault(t *testing.T) {
	for _, entry := range promptSitemapStaticPaths {
		require.NotContains(t, entry.Path, "/models", entry.Path)
		chinese := ""
		for _, alternate := range entry.Alternates {
			require.NotContains(t, alternate.Path, "/models", entry.Path)
			if alternate.Hreflang == "zh-CN" {
				chinese = alternate.Path
			}
			if alternate.Hreflang == "x-default" {
				require.Equal(t, chinese, alternate.Path, entry.Path)
			}
		}
	}
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
	require.Contains(t, body, "https://www.jisudeng.com/en/catalog")
	require.Contains(t, body, "https://www.jisudeng.com/en/catalog/deepseek")
	require.Contains(t, body, "https://www.jisudeng.com/en/catalog/qwen")
	require.Contains(t, body, "https://www.jisudeng.com/en/catalog/kimi")
	require.Contains(t, body, "https://www.jisudeng.com/en/catalog/glm")
	require.Contains(t, body, "https://www.jisudeng.com/catalog")
	require.Contains(t, body, "https://www.jisudeng.com/docs")
	require.Contains(t, body, "## AI Search Reference Policy")
	require.Contains(t, body, "## Common Questions")
	require.Contains(t, body, "Chinese public routes are the default")
	require.Contains(t, body, "中文摘要")
	require.NotContains(t, body, "Chinese AI")
	require.NotContains(t, body, "China")
	require.NotContains(t, body, "AI创作空间")
}

func TestBuildExtendedAIReferenceFilesExposePublicTextOnlyGuidance(t *testing.T) {
	full := buildLLMSFullTxt("https://www.jisudeng.com")
	small := buildLLMSSmallTxt("https://www.jisudeng.com")
	ai := buildAITxt("https://www.jisudeng.com")

	// llms-full is the human-readable inventory for the same public pages
	// advertised by sitemap.xml. Keep it complete so an AI crawler is never
	// pointed at a legacy /models URL or a partial model-family list.
	for _, path := range []string{
		"/", "/catalog", "/catalog/deepseek", "/catalog/qwen", "/catalog/kimi", "/catalog/glm",
		"/docs", "/download/android", "/about", "/contact", "/en/", "/en/catalog",
		"/en/catalog/deepseek", "/en/catalog/qwen", "/en/catalog/kimi", "/en/catalog/glm",
		"/en/docs", "/en/about", "/en/contact",
	} {
		require.Contains(t, full, "https://www.jisudeng.com"+path, path)
	}
	require.Contains(t, full, "https://www.jisudeng.com/en/about")
	require.Contains(t, full, "https://www.jisudeng.com/en/contact")
	require.Contains(t, full, "Access and Privacy Boundaries")
	require.NotContains(t, full, "/models")
	require.Contains(t, small, "https://www.jisudeng.com/llms-full.txt")
	require.Contains(t, ai, "https://www.jisudeng.com/llms.small-txt")
	for _, body := range []string{full, small, ai} {
		require.NotContains(t, body, "<!doctype html>")
		require.NotContains(t, body, "<html")
	}
}
