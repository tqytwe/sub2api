package admin

import (
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func parseUsagePlatform(c *gin.Context) (string, error) {
	platform := strings.ToLower(strings.TrimSpace(c.Query("platform")))
	if platform == "" || platform == "unknown" {
		return platform, nil
	}
	switch platform {
	case service.PlatformAnthropic, service.PlatformOpenAI, service.PlatformGemini, service.PlatformAntigravity, service.PlatformGrok, service.PlatformKimi, service.PlatformZhipu, service.PlatformDeepseek, service.PlatformMiniMax, service.PlatformQwen, service.PlatformTypeSafe, service.PlatformOpenCodeGo, service.PlatformCline, service.PlatformCommandCode, service.PlatformMistral:
		return platform, nil
	default:
		return "", fmt.Errorf("invalid platform: use a supported provider or unknown")
	}
}
