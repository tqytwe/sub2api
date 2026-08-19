package handler

import (
	"database/sql"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/securityaudit"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/google/wire"
)

// ProvideAdminHandlers creates the AdminHandlers struct
func ProvideAdminHandlers(
	dashboardHandler *admin.DashboardHandler,
	userHandler *admin.UserHandler,
	groupHandler *admin.GroupHandler,
	accountHandler *admin.AccountHandler,
	announcementHandler *admin.AnnouncementHandler,
	dataManagementHandler *admin.DataManagementHandler,
	backupHandler *admin.BackupHandler,
	oauthHandler *admin.OAuthHandler,
	openaiOAuthHandler *admin.OpenAIOAuthHandler,
	geminiOAuthHandler *admin.GeminiOAuthHandler,
	antigravityOAuthHandler *admin.AntigravityOAuthHandler,
	grokOAuthHandler *admin.GrokOAuthHandler,
	proxyHandler *admin.ProxyHandler,
	redeemHandler *admin.RedeemHandler,
	promoHandler *admin.PromoHandler,
	couponHandler *admin.CouponHandler,
	settingHandler *admin.SettingHandler,
	opsHandler *admin.OpsHandler,
	systemHandler *admin.SystemHandler,
	subscriptionHandler *admin.SubscriptionHandler,
	usageHandler *admin.UsageHandler,
	userAttributeHandler *admin.UserAttributeHandler,
	errorPassthroughHandler *admin.ErrorPassthroughHandler,
	tlsFingerprintProfileHandler *admin.TLSFingerprintProfileHandler,
	apiKeyHandler *admin.AdminAPIKeyHandler,
	scheduledTestHandler *admin.ScheduledTestHandler,
	channelHandler *admin.ChannelHandler,
	channelMonitorHandler *admin.ChannelMonitorHandler,
	channelMonitorTemplateHandler *admin.ChannelMonitorRequestTemplateHandler,
	contentModerationHandler *admin.ContentModerationHandler,
	promptAuditHandler *securityaudit.PromptAdminHandler,
	paymentHandler *admin.PaymentHandler,
	affiliateHandler *admin.AffiliateHandler,
	complianceHandler *admin.ComplianceHandler,
	adminPlayHandler *admin.AdminPlayHandler,
	withdrawalHandler *admin.WithdrawalHandler,
	fundHandler *admin.FundHandler,
	modelCatalogHandler *admin.ModelCatalogHandler,
	ipRiskHandler *admin.IPRiskHandler,
	auditLogHandler *admin.AuditLogHandler,
	mobileAttributionAdminHandler *MobileAttributionAdminHandler,
	upstreamBillingProbe *service.UpstreamBillingProbeService,
	ollamaCloudUsage *service.OllamaCloudUsageService,
) *AdminHandlers {
	accountHandler.SetUpstreamBillingProbeService(upstreamBillingProbe)
	accountHandler.SetOllamaCloudUsageService(ollamaCloudUsage)
	return &AdminHandlers{
		Dashboard:              dashboardHandler,
		User:                   userHandler,
		Group:                  groupHandler,
		Account:                accountHandler,
		Announcement:           announcementHandler,
		DataManagement:         dataManagementHandler,
		Backup:                 backupHandler,
		OAuth:                  oauthHandler,
		OpenAIOAuth:            openaiOAuthHandler,
		GeminiOAuth:            geminiOAuthHandler,
		AntigravityOAuth:       antigravityOAuthHandler,
		GrokOAuth:              grokOAuthHandler,
		Proxy:                  proxyHandler,
		Redeem:                 redeemHandler,
		Promo:                  promoHandler,
		Coupon:                 couponHandler,
		Setting:                settingHandler,
		Ops:                    opsHandler,
		System:                 systemHandler,
		Subscription:           subscriptionHandler,
		Usage:                  usageHandler,
		UserAttribute:          userAttributeHandler,
		ErrorPassthrough:       errorPassthroughHandler,
		TLSFingerprintProfile:  tlsFingerprintProfileHandler,
		APIKey:                 apiKeyHandler,
		ScheduledTest:          scheduledTestHandler,
		Channel:                channelHandler,
		ChannelMonitor:         channelMonitorHandler,
		ChannelMonitorTemplate: channelMonitorTemplateHandler,
		ContentModeration:      contentModerationHandler,
		PromptAudit:            promptAuditHandler,
		Payment:                paymentHandler,
		Affiliate:              affiliateHandler,
		Compliance:             complianceHandler,
		Play:                   adminPlayHandler,
		Withdrawal:             withdrawalHandler,
		Fund:                   fundHandler,
		ModelCatalog:           modelCatalogHandler,
		IPRisk:                 ipRiskHandler,
		AuditLog:               auditLogHandler,
		MobileAttribution:      mobileAttributionAdminHandler,
	}
}

func ProvideAnnouncementHandler(
	announcementService *service.AnnouncementService,
	assetService *service.AnnouncementAssetService,
) *AnnouncementHandler {
	return NewAnnouncementHandler(announcementService, assetService)
}

func ProvideAdminAnnouncementHandler(
	announcementService *service.AnnouncementService,
	assetService *service.AnnouncementAssetService,
) *admin.AnnouncementHandler {
	return admin.NewAnnouncementHandler(announcementService, assetService)
}

func ProvideGatewayHandler(
	gatewayService *service.GatewayService,
	openAIGatewayService *service.OpenAIGatewayService,
	geminiCompatService *service.GeminiMessagesCompatService,
	antigravityGatewayService *service.AntigravityGatewayService,
	userService *service.UserService,
	concurrencyService *service.ConcurrencyService,
	billingCacheService *service.BillingCacheService,
	usageService *service.UsageService,
	apiKeyService *service.APIKeyService,
	usageRecordWorkerPool *service.UsageRecordWorkerPool,
	errorPassthroughService *service.ErrorPassthroughService,
	contentModerationService *service.ContentModerationService,
	userMsgQueueService *service.UserMessageQueueService,
	cfg *config.Config,
	settingService *service.SettingService,
	coordinator *securityaudit.Coordinator,
) *GatewayHandler {
	h := NewGatewayHandler(gatewayService, openAIGatewayService, geminiCompatService, antigravityGatewayService,
		userService, concurrencyService, billingCacheService, usageService, apiKeyService, usageRecordWorkerPool,
		errorPassthroughService, contentModerationService, userMsgQueueService, cfg, settingService)
	h.securityAuditCoordinator = coordinator
	return h
}

func ProvideOpenAIGatewayHandler(
	gatewayService *service.OpenAIGatewayService,
	concurrencyService *service.ConcurrencyService,
	billingCacheService *service.BillingCacheService,
	apiKeyService *service.APIKeyService,
	usageRecordWorkerPool *service.UsageRecordWorkerPool,
	errorPassthroughService *service.ErrorPassthroughService,
	contentModerationService *service.ContentModerationService,
	opsService *service.OpsService,
	grokQuotaService *service.GrokQuotaService,
	cfg *config.Config,
	coordinator *securityaudit.Coordinator,
) *OpenAIGatewayHandler {
	h := NewOpenAIGatewayHandler(gatewayService, concurrencyService, billingCacheService, apiKeyService,
		usageRecordWorkerPool, errorPassthroughService, contentModerationService, opsService, cfg)
	h.securityAuditCoordinator = coordinator
	h.grokMediaEligibilityProber = grokQuotaService
	return h
}

func ProvideBatchImageHandler(
	batchService *service.BatchImagePublicService,
	download *service.BatchImageDownloadService,
	cleanup *service.BatchImageCleanupService,
	openAI *OpenAIGatewayHandler,
) *BatchImageHandler {
	h := NewBatchImageHandler(batchService, download, cleanup)
	h.openAI = openAI
	return h
}

// ProvideSystemHandler creates admin.SystemHandler with UpdateService
func ProvideSystemHandler(updateService *service.UpdateService, lockService *service.SystemOperationLockService) *admin.SystemHandler {
	return admin.NewSystemHandler(updateService, lockService)
}

// ProvideSettingHandler creates SettingHandler with version from BuildInfo
func ProvideSettingHandler(settingService *service.SettingService, buildInfo BuildInfo, notificationEmailService *service.NotificationEmailService) *SettingHandler {
	h := NewSettingHandler(settingService, buildInfo.Version)
	h.SetNotificationEmailService(notificationEmailService)
	return h
}

// ProvideAdminSettingHandler creates admin.SettingHandler with notification template APIs.
func ProvideAdminSettingHandler(settingService *service.SettingService, emailService *service.EmailService, turnstileService *service.TurnstileService, aliyunCaptchaService *service.AliyunCaptchaService, opsService *service.OpsService, paymentConfigService *service.PaymentConfigService, paymentService *service.PaymentService, userAttributeService *service.UserAttributeService, notificationEmailService *service.NotificationEmailService, totpService *service.TotpService, userService *service.UserService) *admin.SettingHandler {
	h := admin.NewSettingHandler(settingService, emailService, turnstileService, opsService, paymentConfigService, paymentService, userAttributeService)
	h.SetNotificationEmailService(notificationEmailService)
	h.SetAliyunCaptchaService(aliyunCaptchaService)
	h.SetStepUpDeps(totpService, userService)
	return h
}

func ProvideIPRiskHandler(
	core *service.IPRiskService,
	adminService service.AdminService,
	apiKeys service.APIKeyRepository,
	invalidator service.APIKeyAuthCacheInvalidator,
	hasher *service.IPRiskHasher,
	totpService *service.TotpService,
	userService *service.UserService,
	repo service.IPRiskRepository,
) *admin.IPRiskHandler {
	return admin.NewIPRiskManagementHandler(
		core,
		adminService,
		apiKeys,
		invalidator,
		hasher,
		totpService,
		userService,
		repo,
	)
}

func ProvideOpsHandler(
	opsService *service.OpsService,
	health *service.ImageRuntimesHealthService,
	imageStudioWorker *ImageStudioWorkerRuntime,
	imageStudioService *service.ImageStudioService,
) *admin.OpsHandler {
	health.SetImageStudioRuntime(imageStudioWorker)
	health.SetImageStudioFeature(imageStudioService)
	return admin.NewOpsHandler(opsService, health)
}

// ProvideHandlers creates the Handlers struct
func ProvideHandlers(
	authHandler *AuthHandler,
	userHandler *UserHandler,
	apiKeyHandler *APIKeyHandler,
	usageHandler *UsageHandler,
	redeemHandler *RedeemHandler,
	subscriptionHandler *SubscriptionHandler,
	announcementHandler *AnnouncementHandler,
	channelMonitorUserHandler *ChannelMonitorUserHandler,
	adminHandlers *AdminHandlers,
	gatewayHandler *GatewayHandler,
	openaiGatewayHandler *OpenAIGatewayHandler,
	settingHandler *SettingHandler,
	totpHandler *TotpHandler,
	passkeyHandler *PasskeyHandler,
	paymentHandler *PaymentHandler,
	paymentWebhookHandler *PaymentWebhookHandler,
	couponWalletHandler *CouponWalletHandler,
	availableChannelHandler *AvailableChannelHandler,
	modelPlazaHandler *ModelPlazaHandler,
	asyncImageHandler *AsyncImageHandler,
	batchImageHandler *BatchImageHandler,
	playHandler *PlayHandler,
	walletHandler *WalletHandler,
	fundHandler *FundHandler,
	imageStudioHandler *ImageStudioHandler,
	modelPricingHandler *ModelPricingHandler,
	promptLibraryHandler *PromptLibraryHandler,
	mobileAssetHandler *MobileAssetHandler,
	mobileTaskHandler *MobileTaskHandler,
	mobileVideoHandler *MobileVideoHandler,
	mobileSupportHandler *MobileSupportHandler,
	mobileDiagnosticHandler *MobileDiagnosticHandler,
	mobileDeviceHandler *MobileDeviceHandler,
	mobileAttributionHandler *MobileAttributionHandler,
	mobileWebSearchHandler *MobileWebSearchHandler,
	mobilePlayBillingHandler *MobilePlayBillingHandler,
	mobileReleaseHandler *MobileAppReleaseHandler,
	forumSSOHandler *ForumSSOHandler,
	_ *service.IdempotencyCoordinator,
	_ *service.IdempotencyCleanupService,
) *Handlers {
	return &Handlers{
		Auth:              authHandler,
		User:              userHandler,
		APIKey:            apiKeyHandler,
		Usage:             usageHandler,
		Redeem:            redeemHandler,
		Subscription:      subscriptionHandler,
		Announcement:      announcementHandler,
		ChannelMonitor:    channelMonitorUserHandler,
		Admin:             adminHandlers,
		Gateway:           gatewayHandler,
		OpenAIGateway:     openaiGatewayHandler,
		Setting:           settingHandler,
		Totp:              totpHandler,
		Passkey:           passkeyHandler,
		Payment:           paymentHandler,
		PaymentWebhook:    paymentWebhookHandler,
		Coupon:            couponWalletHandler,
		AvailableChannel:  availableChannelHandler,
		ModelPlaza:        modelPlazaHandler,
		AsyncImage:        asyncImageHandler,
		BatchImage:        batchImageHandler,
		Play:              playHandler,
		Wallet:            walletHandler,
		Fund:              fundHandler,
		ImageStudio:       imageStudioHandler,
		ModelPricing:      modelPricingHandler,
		PromptLibrary:     promptLibraryHandler,
		MobileAsset:       mobileAssetHandler,
		MobileTask:        mobileTaskHandler,
		MobileVideo:       mobileVideoHandler,
		MobileSupport:     mobileSupportHandler,
		MobileDiagnostic:  mobileDiagnosticHandler,
		MobileDevice:      mobileDeviceHandler,
		MobileAttribution: mobileAttributionHandler,
		MobileWebSearch:   mobileWebSearchHandler,
		MobilePlayBilling: mobilePlayBillingHandler,
		MobileRelease:     mobileReleaseHandler,
		ForumSSO:          forumSSOHandler,
	}
}

func ProvideWalletHandler(walletService *service.WalletService, withdrawalService *service.WithdrawalService) *WalletHandler {
	return NewWalletHandler(walletService, withdrawalService)
}

func ProvidePlayHandler(
	playService *service.PlayService,
	billingService *service.BillingService,
	feedbackAssetService *service.AnnouncementAssetService,
) *PlayHandler {
	return NewPlayHandler(playService, billingService, feedbackAssetService)
}

func ProvideAdminPlayHandler(
	playService *service.PlayService,
	totpService *service.TotpService,
	userService *service.UserService,
	releaseService *service.MobileAppReleaseService,
) *admin.AdminPlayHandler {
	h := admin.NewAdminPlayHandler(playService, totpService, userService)
	h.SetMobileAppReleaseService(releaseService)
	return h
}

func ProvideMobileAssetHandler(db *sql.DB, storage service.MobileAssetStorage) *MobileAssetHandler {
	return NewMobileAssetHandlerWithStorage(db, storage)
}

func ProvideMobileTaskHandler(db *sql.DB, push *service.MobilePushService) *MobileTaskHandler {
	return NewMobileTaskHandlerWithPush(service.NewMobileTaskService(db), push)
}

func ProvideMobileVideoHandler(db *sql.DB, apiKeys *service.APIKeyService) *MobileVideoHandler {
	return NewMobileVideoHandler(service.NewMobileTaskService(db), apiKeys, service.NewMobileVideoJobService(db))
}

func ProvideMobileSupportHandler(playService *service.PlayService, feedbackAssetService *service.AnnouncementAssetService) *MobileSupportHandler {
	return NewMobileSupportHandler(playService, feedbackAssetService)
}

func ProvideMobileDeviceHandler(pushService *service.MobilePushService) *MobileDeviceHandler {
	return NewMobileDeviceHandler(pushService)
}

func ProvideMobilePlayBillingHandler(playBillingService *service.MobilePlayBillingService) *MobilePlayBillingHandler {
	return NewMobilePlayBillingHandler(playBillingService)
}

func ProvideMobileReleaseHandler(releaseService *service.MobileAppReleaseService) *MobileAppReleaseHandler {
	return NewMobileAppReleaseHandler(releaseService)
}

func ProvideMobileAttributionEventService(svc *service.MobileAttributionService) mobileAttributionEventService {
	return svc
}

func ProvideMobileAttributionAdminService(svc *service.MobileAttributionService) mobileAttributionAdminService {
	return svc
}

// ProviderSet is the Wire provider set for all handlers
var ProviderSet = wire.NewSet(
	// Top-level handlers
	NewAuthHandler,
	NewUserHandler,
	NewAPIKeyHandler,
	NewUsageHandler,
	NewRedeemHandler,
	NewSubscriptionHandler,
	ProvideAnnouncementHandler,
	NewChannelMonitorUserHandler,
	ProvideGatewayHandler,
	ProvideOpenAIGatewayHandler,
	NewTotpHandler,
	NewPasskeyHandler,
	ProvideSettingHandler,
	NewPaymentHandler,
	NewPaymentWebhookHandler,
	NewCouponWalletHandler,
	NewAvailableChannelHandler,
	ProvideAsyncImageHandler,
	NewModelPlazaHandler,
	ProvideBatchImageHandler,
	ProvidePlayHandler,
	ProvideWalletHandler,
	NewFundHandler,
	NewImageStudioHandler,
	ProvideImageStudioWorkerRuntime,
	NewModelPricingHandler,
	NewPromptLibraryHandler,
	ProvideMobileAssetHandler,
	ProvideMobileTaskHandler,
	ProvideMobileVideoHandler,
	ProvideMobileSupportHandler,
	NewMobileDiagnosticHandler,
	ProvideMobileDeviceHandler,
	NewMobileAttributionHandler,
	NewMobileWebSearchHandlerFromEnvironment,
	ProvideMobilePlayBillingHandler,
	ProvideMobileReleaseHandler,
	NewForumSSOHandler,
	ProvideMobileAttributionEventService,
	ProvideMobileAttributionAdminService,

	// Admin handlers
	admin.NewDashboardHandler,
	admin.NewUserHandler,
	admin.NewGroupHandler,
	admin.ProvideAccountHandler,
	ProvideAdminAnnouncementHandler,
	admin.NewDataManagementHandler,
	admin.NewBackupHandler,
	admin.NewOAuthHandler,
	admin.NewOpenAIOAuthHandler,
	admin.NewGeminiOAuthHandler,
	admin.NewAntigravityOAuthHandler,
	admin.NewGrokOAuthHandler,
	admin.NewProxyHandler,
	admin.NewRedeemHandler,
	admin.NewPromoHandler,
	admin.NewCouponHandler,
	ProvideAdminSettingHandler,
	ProvideOpsHandler,
	ProvideSystemHandler,
	admin.NewSubscriptionHandler,
	admin.NewUsageHandler,
	admin.NewUserAttributeHandler,
	admin.NewErrorPassthroughHandler,
	admin.NewTLSFingerprintProfileHandler,
	admin.NewAdminAPIKeyHandler,
	admin.NewScheduledTestHandler,
	admin.NewChannelHandler,
	admin.NewChannelMonitorHandler,
	admin.NewChannelMonitorRequestTemplateHandler,
	admin.NewContentModerationHandler,
	securityaudit.NewPromptAdminHandler,
	admin.NewPaymentHandler,
	admin.NewAffiliateHandler,
	admin.NewComplianceHandler,
	ProvideAdminPlayHandler,
	admin.NewWithdrawalHandler,
	admin.NewFundHandler,
	admin.NewModelCatalogHandler,
	ProvideIPRiskHandler,
	admin.NewAuditLogHandler,
	NewMobileAttributionAdminHandler,

	// AdminHandlers and Handlers constructors
	ProvideAdminHandlers,
	ProvideHandlers,
)
