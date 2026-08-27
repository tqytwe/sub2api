package server

import (
	"context"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/server/routes"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/web"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const frameSrcRefreshTimeout = 5 * time.Second

var publicHomeStatsService atomic.Pointer[service.PublicHomeStatsService]

func SetPublicHomeStatsService(statsService *service.PublicHomeStatsService) {
	publicHomeStatsService.Store(statsService)
}

func publicHomeStatsRoute() gin.HandlerFunc {
	var once sync.Once
	var route gin.HandlerFunc
	return func(c *gin.Context) {
		statsService := publicHomeStatsService.Load()
		if statsService == nil {
			response.Error(c, http.StatusInternalServerError, "failed to load home stats")
			return
		}
		once.Do(func() {
			route = handler.PublicHomeStats(statsService)
		})
		route(c)
	}
}

// SetupRouter 配置路由器中间件和路由
func SetupRouter(
	r *gin.Engine,
	handlers *handler.Handlers,
	jwtAuth middleware2.JWTAuthMiddleware,
	optionalJWTAuth middleware2.OptionalJWTAuthMiddleware,
	adminAuth middleware2.AdminAuthMiddleware,
	apiKeyAuth middleware2.APIKeyAuthMiddleware,
	auditLog middleware2.AuditLogMiddleware,
	stepUpAuth middleware2.StepUpAuthMiddleware,
	apiKeyService *service.APIKeyService,
	subscriptionService *service.SubscriptionService,
	opsService *service.OpsService,
	settingService *service.SettingService,
	dashboardService *service.DashboardService,
	modelCatalogService *service.ModelCatalogService,
	promptLibraryService *service.PromptLibraryService,
	compositeResolver *service.CompositeRouteResolver,
	cfg *config.Config,
	redisClient *redis.Client,
) *gin.Engine {
	middleware2.SetIngressRejectRecorder(opsService)
	// 缓存 iframe 页面的 origin 列表，用于动态注入 CSP frame-src / frame-ancestors
	var cachedFrameOrigins atomic.Pointer[[]string]
	var cachedFrameAncestorOrigins atomic.Pointer[[]string]
	emptyOrigins := []string{}
	cachedFrameOrigins.Store(&emptyOrigins)
	cachedFrameAncestorOrigins.Store(&emptyOrigins)

	refreshFrameOrigins := func() {
		ctx, cancel := context.WithTimeout(context.Background(), frameSrcRefreshTimeout)
		defer cancel()
		origins, err := settingService.GetFrameSrcOrigins(ctx)
		if err != nil {
			// 获取失败时保留已有缓存，避免 frame-src 被意外清空
			return
		}
		cachedFrameOrigins.Store(&origins)
		ancestorOrigins, err := settingService.GetFrameAncestorOrigins(ctx)
		if err != nil {
			return
		}
		cachedFrameAncestorOrigins.Store(&ancestorOrigins)
	}
	refreshFrameOrigins() // 启动时初始化

	// 应用中间件
	r.Use(middleware2.RequestLogger())
	// 将客户端 IP + UA 注入 request context，供 token 签发/会话绑定/审计日志统一读取。
	// 解析模式按请求快照：兼容开关开启时信任原始转发头，关闭时使用 server.trusted_proxies。
	r.Use(middleware2.SessionBindingContext(cfg))
	r.Use(middleware2.Logger())
	r.Use(middleware2.CORS(cfg.CORS))
	r.Use(middleware2.SecurityHeaders(cfg.Security.CSP, func() []string {
		if p := cachedFrameOrigins.Load(); p != nil {
			return *p
		}
		return nil
	}, func() []string {
		if p := cachedFrameAncestorOrigins.Load(); p != nil {
			return *p
		}
		return nil
	}))
	r.Use(middleware2.ServerTiming(cfg.Server.EnableServerTiming))

	// Serve embedded frontend with settings injection if available
	if web.HasEmbeddedFrontend() {
		frontendServer, err := web.NewFrontendServer(settingService) //nolint:staticcheck // SA4023: the !embed stub always errors; embed builds can return nil
		if err != nil {                                              //nolint:staticcheck // SA4023: see above
			log.Printf("Warning: Failed to create frontend server with settings injection: %v, using legacy mode", err)
			r.Use(web.ServeEmbeddedFrontend())
			settingService.SetOnUpdateCallback(refreshFrameOrigins)
		} else {
			// Register combined callback: invalidate HTML cache + refresh frame origins
			settingService.SetOnUpdateCallback(func() {
				frontendServer.InvalidateCache()
				refreshFrameOrigins()
			})
			r.Use(frontendServer.Middleware())
		}
	} else {
		settingService.SetOnUpdateCallback(refreshFrameOrigins)
	}

	// 注册路由
	registerRoutes(r, handlers, jwtAuth, optionalJWTAuth, adminAuth, apiKeyAuth, auditLog, stepUpAuth, apiKeyService, subscriptionService, opsService, settingService, dashboardService, modelCatalogService, promptLibraryService, compositeResolver, cfg, redisClient)

	return r
}

// registerRoutes 注册所有 HTTP 路由
func registerRoutes(
	r *gin.Engine,
	h *handler.Handlers,
	jwtAuth middleware2.JWTAuthMiddleware,
	optionalJWTAuth middleware2.OptionalJWTAuthMiddleware,
	adminAuth middleware2.AdminAuthMiddleware,
	apiKeyAuth middleware2.APIKeyAuthMiddleware,
	auditLog middleware2.AuditLogMiddleware,
	stepUpAuth middleware2.StepUpAuthMiddleware,
	apiKeyService *service.APIKeyService,
	subscriptionService *service.SubscriptionService,
	opsService *service.OpsService,
	settingService *service.SettingService,
	dashboardService *service.DashboardService,
	modelCatalogService *service.ModelCatalogService,
	promptLibraryService *service.PromptLibraryService,
	compositeResolver *service.CompositeRouteResolver,
	cfg *config.Config,
	redisClient *redis.Client,
) {
	// 通用路由（健康检查、状态等）
	routes.RegisterCommonRoutes(r)
	r.GET("/downloads/android-version.json", h.MobileRelease.CompatibilityManifest)

	// API v1
	v1 := r.Group("/api/v1")

	// 面板 API 限流器：认证接口按用户 ID、公开接口按安全客户端 IP，
	// 防止高频刷管理面接口打爆数据库（阈值可在系统设置中调整）。
	panelRateLimiter := middleware2.NewPanelRateLimiter(redisClient, settingService)

	// 注册各模块路由
	routes.RegisterAuthRoutes(v1, h, jwtAuth, auditLog, redisClient, settingService, panelRateLimiter)
	routes.RegisterUserRoutes(v1, h, jwtAuth, auditLog, settingService, panelRateLimiter)
	routes.RegisterModelPlazaRoutes(v1, h, optionalJWTAuth, settingService, panelRateLimiter)
	routes.RegisterAdminRoutes(v1, h, adminAuth, auditLog, stepUpAuth, settingService, panelRateLimiter)
	routes.RegisterGatewayRoutes(r, h, apiKeyAuth, apiKeyService, subscriptionService, opsService, settingService, compositeResolver, cfg)
	routes.RegisterNextChatRoutes(v1, jwtAuth, apiKeyService, modelCatalogService, promptLibraryService, h.ImageStudio, settingService, cfg, redisClient)
	routes.RegisterMobileNextChatSessionRoutes(v1, jwtAuth, apiKeyService, modelCatalogService, settingService, cfg)
	v1.GET("/mobile/protocol", handler.MobileProtocol)
	v1.GET("/mobile/session/status", gin.HandlerFunc(jwtAuth), handler.MobileSessionStatus)
	v1.GET("/mobile/account-summary", gin.HandlerFunc(jwtAuth), handler.MobileAccountSummary(h.Wallet, h.Payment, h.Subscription))
	v1.GET("/nextchat/mobile/account-summary", gin.HandlerFunc(jwtAuth), handler.MobileAccountSummary(h.Wallet, h.Payment, h.Subscription))
	mobileSkills := handler.NewMobileSkillHandlerFromAuth(h.Auth)
	v1.GET("/mobile/skills", gin.HandlerFunc(jwtAuth), mobileSkills.List)
	v1.GET("/mobile/skills/:slug", gin.HandlerFunc(jwtAuth), mobileSkills.Get)
	v1.POST("/mobile/skills/:slug/install", gin.HandlerFunc(jwtAuth), mobileSkills.Install)
	v1.POST("/mobile/skills/:slug/use", gin.HandlerFunc(jwtAuth), mobileSkills.Use)
	v1.DELETE("/mobile/skills/:slug/install", gin.HandlerFunc(jwtAuth), mobileSkills.Uninstall)
	mobileWriteCorrelationID := middleware2.ClientRequestID()
	mobileAssets := h.MobileAsset
	v1.POST("/mobile/assets", mobileWriteCorrelationID, gin.HandlerFunc(jwtAuth), mobileAssets.Upload)
	v1.GET("/mobile/assets", gin.HandlerFunc(jwtAuth), mobileAssets.List)
	v1.GET("/mobile/assets/sync", gin.HandlerFunc(jwtAuth), mobileAssets.Sync)
	v1.GET("/mobile/assets/:id", gin.HandlerFunc(jwtAuth), mobileAssets.Get)
	v1.GET("/mobile/assets/:id/content", gin.HandlerFunc(jwtAuth), mobileAssets.Content)
	v1.PATCH("/mobile/assets/:id", mobileWriteCorrelationID, gin.HandlerFunc(jwtAuth), mobileAssets.Rename)
	v1.DELETE("/mobile/assets/:id", gin.HandlerFunc(jwtAuth), mobileAssets.Delete)
	v1.POST("/mobile/tasks", gin.HandlerFunc(jwtAuth), h.MobileTask.Create)
	v1.GET("/mobile/tasks", gin.HandlerFunc(jwtAuth), h.MobileTask.List)
	v1.GET("/mobile/tasks/:id", gin.HandlerFunc(jwtAuth), h.MobileTask.Get)
	v1.DELETE("/mobile/tasks/:id", gin.HandlerFunc(jwtAuth), h.MobileTask.Delete)
	v1.POST("/mobile/tasks/:id/cancel", gin.HandlerFunc(jwtAuth), h.MobileTask.Cancel)
	v1.POST("/mobile/tasks/:id/retry", gin.HandlerFunc(jwtAuth), h.MobileTask.Retry)
	v1.POST("/mobile/tasks/:id/status", gin.HandlerFunc(jwtAuth), h.MobileTask.Transition)
	mobileVideo := h.MobileVideo
	if mobileVideo != nil {
		v1.GET("/mobile/video/bootstrap", gin.HandlerFunc(jwtAuth), mobileVideo.Bootstrap)
		v1.GET("/mobile/video/models", gin.HandlerFunc(jwtAuth), mobileVideo.Models)
		v1.POST("/mobile/video/estimate", gin.HandlerFunc(jwtAuth), mobileVideo.Estimate)
		v1.POST("/mobile/video/jobs", mobileWriteCorrelationID, gin.HandlerFunc(jwtAuth), mobileVideo.Create)
		v1.GET("/mobile/video/jobs", gin.HandlerFunc(jwtAuth), mobileVideo.List)
		v1.GET("/mobile/video/jobs/:id", gin.HandlerFunc(jwtAuth), mobileVideo.Get)
		v1.POST("/mobile/video/jobs/:id/cancel", gin.HandlerFunc(jwtAuth), mobileVideo.Cancel)
		v1.POST("/mobile/video/jobs/:id/retry", gin.HandlerFunc(jwtAuth), mobileVideo.Retry)
		v1.GET("/mobile/video/jobs/:id/content", gin.HandlerFunc(jwtAuth), mobileVideo.Content)
		v1.POST("/mobile/video/jobs/:id/content/acknowledge", gin.HandlerFunc(jwtAuth), mobileVideo.AcknowledgeContent)
		v1.POST("/mobile/video/jobs/:id/save-as-asset", gin.HandlerFunc(jwtAuth), mobileVideo.SaveAsAsset)
	}
	v1.GET("/mobile/image-history", gin.HandlerFunc(jwtAuth), h.MobileTask.ImageHistory)
	v1.DELETE("/mobile/image-history/:id", gin.HandlerFunc(jwtAuth), h.MobileTask.DeleteImageHistory)
	v1.POST("/mobile/image-history/:id/retry", gin.HandlerFunc(jwtAuth), h.MobileTask.RetryImageHistory)
	v1.GET("/mobile/support/tickets", gin.HandlerFunc(jwtAuth), h.MobileSupport.List)
	v1.POST("/mobile/support/tickets", mobileWriteCorrelationID, gin.HandlerFunc(jwtAuth), h.MobileSupport.Create)
	v1.GET("/mobile/support/tickets/:id", gin.HandlerFunc(jwtAuth), h.MobileSupport.Detail)
	v1.POST("/mobile/support/tickets/:id/messages", gin.HandlerFunc(jwtAuth), h.MobileSupport.AddMessage)
	v1.POST("/mobile/support/tickets/:id/close", gin.HandlerFunc(jwtAuth), h.MobileSupport.Close)
	v1.POST("/mobile/diagnostics", gin.HandlerFunc(jwtAuth), h.MobileDiagnostic.Create)
	v1.POST("/mobile/web-search", middleware2.ClientRequestID(), gin.HandlerFunc(jwtAuth), panelRateLimiter.Global(), h.MobileWebSearch.Search)
	v1.POST("/mobile/attribution/events", panelRateLimiter.PublicIP(), gin.HandlerFunc(optionalJWTAuth), h.MobileAttribution.Event)
	v1.PUT("/mobile/devices/:installation_id", gin.HandlerFunc(jwtAuth), h.MobileDevice.Register)
	v1.DELETE("/mobile/devices/:installation_id", gin.HandlerFunc(jwtAuth), h.MobileDevice.Delete)
	v1.POST("/redeem-codes/redeem", gin.HandlerFunc(jwtAuth), h.Redeem.Redeem)
	v1.GET("/redeem-codes/history", gin.HandlerFunc(jwtAuth), h.Redeem.GetHistory)
	v1.POST("/mobile/payments/create", gin.HandlerFunc(jwtAuth), h.Payment.MobileCreate)
	v1.GET("/mobile/payments/:order_id", gin.HandlerFunc(jwtAuth), h.Payment.MobileGet)
	v1.POST("/mobile/payments/:order_id/sync", gin.HandlerFunc(jwtAuth), h.Payment.MobileSync)
	mobilePlayBilling := h.MobilePlayBilling
	if mobilePlayBilling == nil {
		mobilePlayBilling = handler.NewMobilePlayBillingHandler()
	}
	v1.POST("/mobile/play-billing/purchases", mobileWriteCorrelationID, gin.HandlerFunc(jwtAuth), mobilePlayBilling.SubmitPurchase)
	v1.GET("/mobile/releases/check", h.MobileRelease.Check)
	v1.GET("/mobile/releases/:id/download", h.MobileRelease.Download)
	routes.RegisterPaymentRoutes(v1, h.Payment, h.PaymentWebhook, h.Admin.Payment, jwtAuth, adminAuth, auditLog, settingService, panelRateLimiter)
	routes.RegisterPlayRoutes(v1, h, jwtAuth, panelRateLimiter)
	routes.RegisterImageStudioRoutes(v1, h, jwtAuth)
	routes.RegisterPromptLibraryRoutes(v1, h, jwtAuth)
	mobileCanvasPrompts := v1.Group("/mobile/canvas-prompts")
	mobileCanvasPrompts.Use(gin.HandlerFunc(jwtAuth))
	routes.RegisterCanvasPromptMirrorRoutes(mobileCanvasPrompts, h)
	routes.RegisterPromptLibrarySEORoutes(r, h)

	v1.GET("/public/home-stats", publicHomeStatsRoute())
	v1.GET("/public/growth-teaser", handler.PublicGrowthTeaser(settingService, dashboardService, h.Play))
	v1.GET("/public/vip-tiers", handler.PublicVIPTiers(settingService))
	v1.GET("/announcement-assets/*filepath", h.Announcement.GetAsset)

	handler.RegisterPageRoutes(v1, cfg.Pricing.DataDir, gin.HandlerFunc(jwtAuth), gin.HandlerFunc(adminAuth), settingService)
}
