package service

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service/basispoints"
	"github.com/gin-gonic/gin"
)

func (s *OpenAIGatewayService) getExcelBPSImageRelay(settings ExcelBPSImageRelaySettings) (*basispoints.ImageRelay, error) {
	s.excelBPSImagesMu.Lock()
	defer s.excelBPSImagesMu.Unlock()
	if s.excelBPSImages == nil {
		dataDir := strings.TrimSpace(os.Getenv("DATA_DIR"))
		if dataDir == "" {
			dataDir = "./data"
		}
		relay, err := basispoints.NewImageRelay(settings.BaseURL, filepath.Join(dataDir, "bps-images"))
		if err != nil {
			return nil, err
		}
		s.excelBPSImages = relay
	}
	if err := s.excelBPSImages.Configure(settings.BaseURL, settings.Limits); err != nil {
		return nil, err
	}
	return s.excelBPSImages, nil
}

func (s *OpenAIGatewayService) CloseExcelBPSImages() error {
	if s == nil {
		return nil
	}
	s.excelBPSImagesMu.Lock()
	defer s.excelBPSImagesMu.Unlock()
	return s.excelBPSImages.Close()
}

// ServeExcelBPSImage serves the short-lived opaque URL created for an upstream request.
func (s *OpenAIGatewayService) ServeExcelBPSImage(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	settings, err := s.settingService.GetExcelBPSImageRelaySettings(c.Request.Context())
	if err != nil || !settings.Enabled || settings.Mode != ExcelBPSImageModeRelay {
		http.NotFound(c.Writer, c.Request)
		return
	}
	s.excelBPSImagesMu.Lock()
	relay := s.excelBPSImages
	s.excelBPSImagesMu.Unlock()
	if relay == nil {
		http.NotFound(c.Writer, c.Request)
		return
	}
	relay.ServeHTTP(c.Writer, c.Request)
}
