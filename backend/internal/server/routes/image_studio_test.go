package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestImageStudioGenerateRouteRejectsOversizedBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")
	RegisterImageStudioRoutes(
		v1,
		&handler.Handlers{ImageStudio: &handler.ImageStudioHandler{}},
		servermiddleware.JWTAuthMiddleware(func(c *gin.Context) {
			c.Set(
				string(servermiddleware.ContextKeyUser),
				servermiddleware.AuthSubject{UserID: 42},
			)
			c.Next()
		}),
	)

	body := `{"user_prompt":"` +
		strings.Repeat("x", int(handler.ImageStudioGenerateRequestBodyLimit)) +
		`"}`
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/image-studio/generate",
		strings.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
	require.Equal(
		t,
		"IMAGE_STUDIO_REQUEST_TOO_LARGE",
		gjson.Get(recorder.Body.String(), "reason").String(),
	)
}

func TestImageStudioGenerateRouteRejectsOversizedTrailingWhitespace(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")
	RegisterImageStudioRoutes(
		v1,
		&handler.Handlers{ImageStudio: &handler.ImageStudioHandler{}},
		servermiddleware.JWTAuthMiddleware(func(c *gin.Context) {
			c.Set(
				string(servermiddleware.ContextKeyUser),
				servermiddleware.AuthSubject{UserID: 42},
			)
			c.Next()
		}),
	)

	body := `{"user_prompt":"ok"}` +
		strings.Repeat(" ", int(handler.ImageStudioGenerateRequestBodyLimit))
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/image-studio/generate",
		strings.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
	require.Equal(
		t,
		"IMAGE_STUDIO_REQUEST_TOO_LARGE",
		gjson.Get(recorder.Body.String(), "reason").String(),
	)
}
