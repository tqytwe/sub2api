package service

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestForwardExcelBPSRelaysInlineImageWhenConfigured(t *testing.T) {
	t.Setenv("DATA_DIR", t.TempDir())
	account := excelAccount()
	upstream := &excelBPSHTTPUpstreamStub{resp: &http.Response{
		StatusCode: http.StatusOK, Header: http.Header{},
		Body: io.NopCloser(strings.NewReader("event: response.completed\n" +
			`data: {"type":"response.completed","response":{"id":"resp-image","model":"gpt-5","status":"completed","output":[]}}` + "\n\n")),
	}}
	gateway := newExcelBPSRuntime(t, account, `{"allowed":true,"model_catalog":{"models":[{"id":"gpt-5","available":true}]}}`, true, upstream)
	gateway.settingService = &SettingService{settingRepo: &excelBPSImageSettingsRepo{values: map[string]string{
		"excel_bps_image_mode": "relay", "excel_bps_image_base_url": "https://example.org",
	}}}
	t.Cleanup(func() { require.NoError(t, gateway.CloseExcelBPSImages()) })
	c, _ := newExcelBPSContext()
	body := []byte(`{"model":"gpt-5","input":[{"type":"message","role":"user","content":[{"type":"input_image","image_url":"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jv5sAAAAASUVORK5CYII="}]}]}`)
	_, err := gateway.forwardExcelBPS(c.Request.Context(), c, account, body, time.Now())
	require.NoError(t, err)
	require.Len(t, upstream.requests, 1, "relay mode does not upload a native attachment")
	imagePart := gjson.GetBytes(upstream.bodies[0], `input.#(content.#(type=="input_image").type=="input_image").content.#(type=="input_image")`)
	imageURL := imagePart.Get("image_url").String()
	require.True(t, strings.HasPrefix(imageURL, "https://example.org/api/bps-images/"), "expected a temporary relay URL")
	require.False(t, imagePart.Get("file_id").Exists())
	r := httptest.NewRecorder()
	imageCtx, _ := gin.CreateTestContext(r)
	imageCtx.Request = httptest.NewRequest(http.MethodGet, imageURL, nil)
	gateway.ServeExcelBPSImage(imageCtx)
	require.Equal(t, http.StatusOK, r.Code)
	require.Equal(t, "image/png", r.Header().Get("Content-Type"))
	require.Equal(t, "private, no-store", r.Header().Get("Cache-Control"))
}
