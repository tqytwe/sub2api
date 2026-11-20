package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/authguard"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestAuthAbuseGuardSettingsAreIndependent(t *testing.T) {
	mr := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })
	loginEnabled, registrationEnabled := true, true
	router := gin.New()
	require.NoError(t, router.SetTrustedProxies(nil))
	router.Use(AuthAbuseGuardWithOptions(redisClient, AuthAbuseGuardOptions{
		LoginRateLimitEnabled:      func(*gin.Context) bool { return loginEnabled },
		RegistrationIPLimitEnabled: func(*gin.Context) bool { return registrationEnabled },
	}))
	router.POST("/login", func(c *gin.Context) {
		response.ErrorWithDetails(c, http.StatusUnauthorized, "sign-in failed", "INVALID_CREDENTIALS", nil)
	})
	router.POST("/register", func(c *gin.Context) {
		_, enabled := authguard.RegistrationIP(c.Request.Context())
		response.Success(c, enabled)
	})
	request := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, path, nil)
		r.RemoteAddr = "192.0.2.7:1234"
		router.ServeHTTP(w, r)
		return w
	}
	require.Equal(t, http.StatusUnauthorized, request("/login").Code)
	require.Len(t, mr.Keys(), 1)
	loginEnabled = false
	mr.FlushAll()
	require.Equal(t, http.StatusUnauthorized, request("/login").Code)
	require.Empty(t, mr.Keys())
	require.Contains(t, request("/register").Body.String(), `"data":true`)
	registrationEnabled = false
	require.Contains(t, request("/register").Body.String(), `"data":false`)
}

func TestAuthAbuseGuardAvoidsSharedQuotaAtUnconfiguredIngress(t *testing.T) {
	mr := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })
	for _, tt := range []struct {
		name    string
		proxies []string
		active  bool
	}{
		{name: "unconfigured ingress"},
		{name: "configured ingress", proxies: []string{"10.20.30.40/32"}, active: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mr.FlushAll()
			router := gin.New()
			require.NoError(t, router.SetTrustedProxies(tt.proxies))
			router.Use(AuthAbuseGuard(redisClient))
			router.POST("/register", func(c *gin.Context) {
				_, enabled := authguard.RegistrationIP(c.Request.Context())
				response.Success(c, enabled)
			})
			router.POST("/login", func(c *gin.Context) {
				response.ErrorWithDetails(c, http.StatusUnauthorized, "sign-in failed", "INVALID_CREDENTIALS", nil)
			})
			request := func(path string) *httptest.ResponseRecorder {
				r := httptest.NewRequest(http.MethodPost, path, nil)
				r.RemoteAddr = "10.20.30.40:1234"
				r.Header.Set("X-Forwarded-For", "192.0.2.7")
				w := httptest.NewRecorder()
				router.ServeHTTP(w, r)
				return w
			}
			require.Equal(t, http.StatusUnauthorized, request("/login").Code)
			if tt.active {
				require.Len(t, mr.Keys(), 1)
				require.Contains(t, request("/register").Body.String(), `"data":true`)
			} else {
				require.Empty(t, mr.Keys())
				require.Contains(t, request("/register").Body.String(), `"data":false`)
			}
		})
	}
}
