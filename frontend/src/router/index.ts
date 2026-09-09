/**
 * Vue Router configuration for Sub2API frontend
 * Defines all application routes with lazy loading and navigation guards
 */

import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { applyLocaleFromRoute, ensureLocaleMessagesForRoute, getLocale, inheritedEnglishLocaleQuery } from '@/i18n'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { useAdminSettingsStore } from '@/stores/adminSettings'
import { useAdminComplianceStore } from '@/stores/adminCompliance'
import { useNavigationLoadingState } from '@/composables/useNavigationLoading'
import { getSetupStatus } from '@/api/setup'
import { resolveCompletedSetupRedirectPath } from './setupRedirect'
import { resolveRouteDocumentTitle } from './title'
import { useTheme } from '@/composables/useTheme'
import { recoverFromChunkLoadError } from './chunkRecovery'
import { applyPublicRouteSeo } from '@/utils/routeSeo'
import { FORUM_SSO_RESUME_PARAM } from '@/composables/useForumSsoResume'
import { resolveCustomMenuRouteById, resolveLegacyDocsCustomPageRoute } from './customMenuTarget'
import { createNavigationGeneration } from './navigationGeneration'

const adminPromptAuditPath = '/admin/pro' + 'mpt-audit'

/**
 * Route definitions with lazy loading
 */
const routes: RouteRecordRaw[] = [
  // ==================== Setup Routes ====================
  {
    path: '/setup',
    name: 'Setup',
    component: () => import('@/views/setup/SetupWizardView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Setup'
    }
  },

  // ==================== Public Routes ====================
  {
    path: '/en',
    name: 'EnglishHome',
    component: () => import('@/views/HomeView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Jisudeng',
      absoluteTitle: 'Jisudeng: One OpenAI-Compatible API for Frontier AI Models',
      frame: 'fluid'
    }
  },
  {
    path: '/en/catalog',
    name: 'EnglishModels',
    component: () => import('@/views/ModelPlazaView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Models & Pricing',
      absoluteTitle: 'DeepSeek, Qwen, Kimi, GLM, Claude API Pricing | Jisudeng',
      frame: 'workspace'
    }
  },
  {
    path: '/en/catalog/:family(deepseek|qwen|kimi|glm)',
    name: 'EnglishModelFamily',
    component: () => import('@/views/ModelPlazaView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Model pricing',
      frame: 'workspace'
    }
  },
  {
    path: '/en/models',
    // Legacy browser route. The protected root /models API remains a backend
    // contract; old English page bookmarks move to the canonical catalog.
    redirect: to => ({ path: '/en/catalog', query: to.query }),
  },
  {
    path: '/en/models/:family(deepseek|qwen|kimi|glm)',
    redirect: to => ({ path: `/en/catalog/${to.params.family}`, query: to.query }),
  },
  {
    path: '/en/docs',
    name: 'EnglishDocs',
    component: () => import('@/views/public/DocsView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Docs',
      absoluteTitle: 'Jisudeng API Docs - OpenAI-Compatible Gateway Quickstart',
      frame: 'workspace'
    }
  },
  {
    path: '/en/status',
    name: 'EnglishStatus',
    component: () => import('@/views/public/PublicStatusView.vue'),
    meta: {
      requiresAuth: false,
      title: 'System status',
      absoluteTitle: 'Jisudeng System Status',
      frame: 'reading',
    },
  },
  {
    path: '/en/about',
    name: 'EnglishAbout',
    component: () => import('@/views/public/AboutView.vue'),
    meta: {
      requiresAuth: false,
      title: 'About Jisudeng',
      frame: 'reading'
    }
  },
  {
    path: '/en/contact',
    name: 'EnglishContact',
    component: () => import('@/views/public/ContactView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Contact Jisudeng',
      frame: 'reading'
    }
  },
  {
    path: '/pricing',
    // design-governance-allow: visual-evidence - this compatibility route only restores a named redirect and does not add a new visual surface
    name: 'Pricing',
    redirect: to => ({ path: '/catalog', query: to.query }),
  },
  {
    path: '/pricing/:family(deepseek|qwen|kimi|glm)',
    redirect: to => ({ path: `/catalog/${to.params.family}`, query: to.query }),
  },
  {
    path: '/download/android',
    name: 'AndroidDownload',
    component: () => import('@/views/public/AndroidDownloadView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Android 下载',
      titleKey: 'androidDownload.metaTitle',
      frame: 'content'
    }
  },
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/auth/LoginView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Login',
      titleKey: 'home.login'
    }
  },
  {
    path: '/register',
    name: 'Register',
    component: () => import('@/views/auth/RegisterView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Register',
      titleKey: 'auth.createAccount'
    }
  },
  {
    path: '/email-verify',
    name: 'EmailVerify',
    component: () => import('@/views/auth/EmailVerifyView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Verify Email'
    }
  },
  {
    path: '/auth/callback',
    name: 'OAuthCallback',
    alias: '/auth/oauth/callback',
    component: () => import('@/views/auth/OAuthCallbackView.vue'),
    meta: {
      requiresAuth: false,
      title: 'OAuth Callback',
      titleKey: 'auth.oauthCallbackPageTitle'
    }
  },
  {
    path: '/auth/linuxdo/callback',
    name: 'LinuxDoOAuthCallback',
    component: () => import('@/views/auth/LinuxDoCallbackView.vue'),
    meta: {
      requiresAuth: false,
      title: 'LinuxDo OAuth Callback',
      titleKey: 'auth.linuxdoCallbackPageTitle'
    }
  },
  {
    path: '/auth/wechat/callback',
    name: 'WeChatOAuthCallback',
    component: () => import('@/views/auth/WechatCallbackView.vue'),
    meta: {
      requiresAuth: false,
      title: 'WeChat OAuth Callback',
      titleKey: 'auth.wechatCallbackPageTitle'
    }
  },
  {
    path: '/auth/wechat/payment/callback',
    name: 'WeChatPaymentOAuthCallback',
    component: () => import('@/views/auth/WechatPaymentCallbackView.vue'),
    meta: {
      requiresAuth: false,
      title: 'WeChat Payment Callback',
      titleKey: 'auth.wechatPaymentCallbackPageTitle'
    }
  },
  {
    path: '/auth/dingtalk/callback',
    name: 'DingTalkOAuthCallback',
    component: () => import('@/views/auth/DingTalkCallbackView.vue'),
    meta: {
      requiresAuth: false,
      title: 'DingTalk OAuth Callback',
      titleKey: 'auth.dingtalkCallbackPageTitle'
    }
  },
  {
    path: '/auth/dingtalk/email-completion',
    name: 'dingtalk-email-completion',
    component: () => import('@/views/auth/DingTalkEmailCompletionView.vue'),
    meta: {
      requiresAuth: false,
      title: 'DingTalk Email Completion'
    }
  },
  {
    path: '/auth/oidc/callback',
    name: 'OIDCOAuthCallback',
    component: () => import('@/views/auth/OidcCallbackView.vue'),
    meta: {
      requiresAuth: false,
      title: 'OIDC OAuth Callback',
      titleKey: 'auth.oidcCallbackPageTitle'
    }
  },
  {
    path: '/forgot-password',
    name: 'ForgotPassword',
    component: () => import('@/views/auth/ForgotPasswordView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Forgot Password',
      titleKey: 'auth.forgotPasswordTitle'
    }
  },
  {
    path: '/reset-password',
    name: 'ResetPassword',
    component: () => import('@/views/auth/ResetPasswordView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Reset Password'
    }
  },
  {
    path: '/key-usage',
    name: 'KeyUsage',
    component: () => import('@/views/KeyUsageView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Key Usage',
      frame: 'reading'
    }
  },
  {
    path: '/legal/:documentId',
    name: 'LegalDocument',
    component: () => import('@/views/public/LegalDocumentView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Legal Document'
    }
  },
  {
    path: '/about',
    name: 'About',
    component: () => import('@/views/public/AboutView.vue'),
    meta: {
      requiresAuth: false,
      title: 'About',
      titleKey: 'about.eyebrow',
      frame: 'reading'
    }
  },
  {
    path: '/contact',
    name: 'Contact',
    component: () => import('@/views/public/ContactView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Contact',
      titleKey: 'contact.title',
      frame: 'reading'
    }
  },
  {
    path: '/contact/qq',
    name: 'ContactQQ',
    redirect: '/contact'
  },
  {
    path: '/docs',
    name: 'Docs',
    component: () => import('@/views/public/DocsView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Docs',
      titleKey: 'docs.title',
      frame: 'workspace'
    }
  },
  {
    path: '/status',
    name: 'Status',
    component: () => import('@/views/public/PublicStatusView.vue'),
    meta: {
      requiresAuth: false,
      title: '系统状态',
      absoluteTitle: '极速蹬系统状态',
      frame: 'reading',
    },
  },
  {
    path: '/blindbox',
    name: 'Blindbox',
    component: () => import('@/views/public/BlindboxView.vue'),
    meta: {
      requiresAuth: false,
      playFeature: 'blindbox',
      title: 'Blindbox',
      titleKey: 'play.blindbox.title',
      hidePageHeader: true,
      frame: 'workspace',
    }
  },
  {
    path: '/arena',
    name: 'Arena',
    component: () => import('@/views/public/ArenaView.vue'),
    meta: {
      requiresAuth: false,
      playFeature: 'arena',
      title: 'Token Farm',
      titleKey: 'play.arena.title',
      hidePageHeader: true,
      frame: 'workspace',
    }
  },
  {
    path: '/quiz-quest',
    name: 'QuizQuest',
    component: () => import('@/views/public/QuizQuestView.vue'),
    meta: {
      requiresAuth: false,
      playFeature: 'quiz-quest',
      title: 'Quiz Quest',
      titleKey: 'play.quizQuest.title',
      hidePageHeader: true,
      frame: 'workspace',
    }
  },
  {
    path: '/catalog',
    name: 'Models',
    component: () => import('@/views/ModelPlazaView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Model Plaza',
      titleKey: 'modelPlaza.title'
    }
  },
  {
    path: '/catalog/:family(deepseek|qwen|kimi|glm)',
    name: 'ModelFamily',
    component: () => import('@/views/ModelPlazaView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Model pricing',
      frame: 'workspace'
    }
  },
  {
    path: '/models',
    // Keep in-app legacy links useful without reintroducing /models as an SEO
    // page or changing the OpenAI-compatible API endpoint on the server.
    redirect: to => ({ path: '/catalog', query: to.query }),
  },
  {
    path: '/models/:family(deepseek|qwen|kimi|glm)',
    redirect: to => ({ path: `/catalog/${to.params.family}`, query: to.query }),
  },
  {
    path: '/model-plaza',
    redirect: to => ({ path: '/catalog', query: to.query }),
  },
  {
    path: '/agent-team',
    name: 'AgentTeam',
    component: () => import('@/views/public/AgentTeamView.vue'),
    meta: {
      requiresAuth: false,
      playFeature: 'agent-team',
      title: 'Agent Team',
      titleKey: 'play.agentTeam.title',
      hidePageHeader: true,
      frame: 'workspace',
    }
  },

  // ==================== User Routes ====================
  {
    path: '/',
    name: 'Home',
    component: () => import('@/views/HomeView.vue'),
    meta: {
      requiresAuth: false,
      title: '',
      frame: 'fluid'
    }
  },
  {
    path: '/home',
    redirect: '/'
  },
  {
    path: '/dashboard',
    name: 'Dashboard',
    component: () => import('@/views/user/DashboardView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Dashboard',
      titleKey: 'dashboard.title',
      descriptionKey: 'dashboard.welcomeMessage',
      localeScopes: ['user-dashboard'],
      frame: 'workspace'
    }
  },
  {
    path: '/keys',
    name: 'Keys',
    component: () => import('@/views/user/KeysView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'API Keys',
      titleKey: 'keys.title',
      descriptionKey: 'keys.description',
      frame: 'workspace'
    }
  },
  {
    path: '/keys/speed-test',
    name: 'KeySpeedTest',
    component: () => import('@/views/user/SpeedTestView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Speed Test',
      titleKey: 'keys.speedTest.title',
      descriptionKey: 'keys.speedTest.description',
      frame: 'workspace'
    }
  },
  {
    path: '/batch-image',
    name: 'BatchImageGuide',
    alias: '/docs/batch-image',
    component: () => import('@/views/user/BatchImageGuideView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Batch Image Guide',
      titleKey: 'batchImageGuide.title',
      descriptionKey: 'batchImageGuide.description',
      localeScopes: ['user-dashboard', 'user-batch'],
      frame: 'workspace'
    }
  },
  {
    path: '/usage',
    name: 'Usage',
    component: () => import('@/views/user/UsageView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Usage Records',
      titleKey: 'usage.title',
      descriptionKey: 'usage.description',
      frame: 'workspace'
    }
  },
  {
    path: '/wallet',
    name: 'Wallet',
    component: () => import('@/views/user/WalletView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Wallet',
      titleKey: 'wallet.title',
      descriptionKey: 'wallet.description',
      localeScopes: ['user-dashboard', 'user-wallet', 'user-misc'],
      frame: 'content'
    }
  },
  {
    path: '/redeem',
    name: 'Redeem',
    component: () => import('@/views/user/RedeemView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Redeem Code',
      titleKey: 'redeem.title',
      descriptionKey: 'redeem.description',
      frame: 'form'
    }
  },
  {
    path: '/ai',
    redirect: to => ({ path: '/ai-creation-space', query: to.query }),
  },
  {
    path: '/ai-creation-space',
    name: 'AICreationSpace',
    component: () => import('@/views/user/CanvasLaunchView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      requiresNextChat: true,
      title: 'AI创作空间',
      titleKey: 'nav.aiCreationSpace',
      hideMobileSupport: true,
      hidePageHeader: true,
      frame: 'fluid',
    },
  },
  {
    path: '/image-studio',
    redirect: to => ({ path: '/ai-creation-space', query: to.query }),
  },
  {
    path: '/play',
    name: 'PlayHub',
    component: () => import('@/views/user/PlayHubView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Play Hub',
      titleKey: 'playHub.title',
      descriptionKey: 'playHub.subtitle',
      hidePageHeader: true,
      frame: 'workspace',
    },
  },
  {
    path: '/check-in',
    name: 'CheckIn',
    component: () => import('@/views/user/CheckInView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Daily Check-in',
      titleKey: 'checkin.title',
      descriptionKey: 'checkin.description',
      hidePageHeader: true,
      frame: 'workspace',
    }
  },
  {
    path: '/affiliate',
    name: 'Affiliate',
    component: () => import('@/views/user/AffiliateView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Affiliate',
      titleKey: 'affiliate.title',
      descriptionKey: 'affiliate.description',
      hidePageHeader: true,
      frame: 'workspace',
    }
  },
  {
    path: '/available-channels',
    name: 'UserAvailableChannels',
    component: () => import('@/views/user/AvailableChannelsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Available Channels',
      titleKey: 'availableChannels.title',
      descriptionKey: 'availableChannels.description',
      frame: 'workspace'
    }
  },
  {
    path: '/profile',
    name: 'Profile',
    component: () => import('@/views/user/ProfileView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Profile',
      titleKey: 'profile.title',
      descriptionKey: 'profile.description',
      frame: 'form'
    }
  },
  {
    path: '/subscriptions',
    name: 'Subscriptions',
    component: () => import('@/views/user/SubscriptionsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'My Subscriptions',
      titleKey: 'userSubscriptions.title',
      descriptionKey: 'userSubscriptions.description',
      frame: 'content'
    }
  },
  {
    path: '/purchase',
    name: 'PurchaseSubscription',
    component: () => import('@/views/user/PaymentView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Purchase Subscription',
      titleKey: 'nav.buySubscription',
      descriptionKey: 'purchase.description',
      requiresPayment: true,
      frame: 'content'
    }
  },
  {
    path: '/orders',
    name: 'OrderList',
    component: () => import('@/views/user/UserOrdersView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'My Orders',
      titleKey: 'nav.myOrders',
      requiresPayment: true,
      frame: 'workspace'
    }
  },
  {
    path: '/payment/qrcode',
    name: 'PaymentQRCode',
    component: () => import('@/views/user/PaymentQRCodeView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Payment',
      titleKey: 'payment.qr.scanToPay',
      requiresPayment: true,
      hidePageHeader: true,
      frame: 'compact'
    }
  },
  {
    path: '/payment/result',
    name: 'PaymentResult',
    component: () => import('@/views/user/PaymentResultView.vue'),
    meta: {
      requiresAuth: false,
      requiresAdmin: false,
      title: 'Payment Result',
      titleKey: 'payment.result.success',
      requiresPayment: false,
      frame: 'compact'
    }
  },
  {
    path: '/payment/stripe',
    name: 'StripePayment',
    component: () => import('@/views/user/StripePaymentView.vue'),
    meta: {
      requiresAuth: false,
      requiresAdmin: false,
      title: 'Stripe Payment',
      titleKey: 'payment.stripePay',
      requiresPayment: false,
      hidePageHeader: true,
      frame: 'compact'
    }
  },
  {
    path: '/payment/airwallex',
    name: 'AirwallexPayment',
    component: () => import('@/views/user/AirwallexPaymentView.vue'),
    meta: {
      requiresAuth: false,
      requiresAdmin: false,
      title: 'Airwallex Payment',
      titleKey: 'payment.airwallexPay',
      requiresPayment: false,
      hidePageHeader: true,
      frame: 'compact'
    }
  },
  {
    path: '/payment/stripe-popup',
    name: 'StripePopup',
    component: () => import('@/views/user/StripePopupView.vue'),
    meta: {
      requiresAuth: false,
      requiresAdmin: false,
      title: 'Payment',
      requiresPayment: false,
      frame: 'compact'
    }
  },
  {
    path: '/custom/:id',
    name: 'CustomPage',
    component: () => import('@/views/user/CustomPageView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Custom Page',
      titleKey: 'customPage.title',
      frame: 'workspace',
    }
  },

  // ==================== Admin Routes ====================
  {
    path: '/admin',
    redirect: '/admin/dashboard'
  },
  {
    path: '/admin/dashboard',
    name: 'AdminDashboard',
    component: () => import('@/views/admin/DashboardView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Admin Dashboard',
      titleKey: 'admin.dashboard.title',
      descriptionKey: 'admin.dashboard.description',
      localeScopes: ['admin-overview'],
      frame: 'workspace'
    }
  },
  {
    path: '/admin/ops',
    name: 'AdminOps',
    component: () => import('@/views/admin/ops/OpsDashboard.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Ops Monitoring',
      titleKey: 'admin.ops.title',
      descriptionKey: 'admin.ops.description',
      localeScopes: ['admin-ops'],
      frame: 'workspace'
    }
  },
  {
    path: '/admin/play-ops',
    name: 'AdminPlayOps',
    component: () => import('@/views/admin/PlayOpsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Play Ops',
      titleKey: 'admin.playOps.title',
      descriptionKey: 'admin.playOps.description',
      localeScopes: ['admin-play', 'admin-resources'],
      frame: 'workspace'
    }
  },
  {
    path: '/admin/funds',
    name: 'AdminFunds',
    component: () => import('@/views/admin/AdminFundsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Fund Management',
      titleKey: 'admin.funds.title',
      descriptionKey: 'admin.funds.description',
      frame: 'workspace'
    }
  },
  {
    path: '/admin/funds/classification',
    redirect: '/admin/funds/operations',
  },
  {
    path: '/admin/funds/:tab(refunds|credits|operations)',
    name: 'AdminFundsTab',
    component: () => import('@/views/admin/AdminFundsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Fund Management',
      titleKey: 'admin.funds.title',
      descriptionKey: 'admin.funds.description',
      frame: 'workspace'
    }
  },
  {
    path: '/admin/withdrawals',
    name: 'AdminWithdrawals',
    component: () => import('@/views/admin/AdminWithdrawalsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Withdrawals',
      titleKey: 'admin.withdrawals.title',
      descriptionKey: 'admin.withdrawals.description',
      frame: 'workspace'
    }
  },
  {
    path: '/admin/audit-logs',
    name: 'AdminAuditLogs',
    component: () => import('@/views/admin/AuditLogView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Audit Logs',
      titleKey: 'admin.audit.title',
      descriptionKey: 'admin.audit.description',
      frame: 'workspace'
    }
  },
  {
    path: '/admin/users',
    name: 'AdminUsers',
    component: () => import('@/views/admin/UsersView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'User Management',
      titleKey: 'admin.users.title',
      descriptionKey: 'admin.users.description',
      localeScopes: ['admin-resources'],
      frame: 'workspace'
    }
  },
  {
    path: '/admin/groups',
    name: 'AdminGroups',
    component: () => import('@/views/admin/GroupsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Group Management',
      titleKey: 'admin.groups.title',
      descriptionKey: 'admin.groups.description',
      frame: 'workspace'
    }
  },
  {
    path: '/admin/channels',
    redirect: '/admin/channels/pricing'
  },
  {
    path: '/admin/channels/pricing',
    name: 'AdminChannels',
    component: () => import('@/views/admin/ChannelsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Channel Management',
      titleKey: 'admin.channels.title',
      descriptionKey: 'admin.channels.description',
      localeScopes: ['admin-channels'],
      frame: 'workspace'
    }
  },
  {
    path: '/admin/channels/monitor',
    name: 'AdminChannelMonitor',
    component: () => import('@/views/admin/ChannelMonitorView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Channel Monitor',
      titleKey: 'admin.channelMonitor.title',
      descriptionKey: 'admin.channelMonitor.description',
      localeScopes: ['admin-channels', 'channel-monitor'],
      frame: 'workspace'
    }
  },
  {
    path: '/admin/model-plaza',
    name: 'AdminModelPlaza',
    component: () => import('@/views/admin/ModelCatalogView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Model Catalog',
      titleKey: 'modelPlaza.title',
      descriptionKey: 'modelPlaza.description',
      frame: 'workspace'
    }
  },
  {
    path: '/monitor',
    name: 'ChannelStatus',
    component: () => import('@/views/user/ChannelStatusView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Channel Status',
      titleKey: 'nav.channelStatus',
      frame: 'workspace'
    }
  },
  {
    path: '/admin/subscriptions',
    name: 'AdminSubscriptions',
    component: () => import('@/views/admin/SubscriptionsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Subscription Management',
      titleKey: 'admin.subscriptions.title',
      descriptionKey: 'admin.subscriptions.description',
      frame: 'workspace'
    }
  },
  {
    path: '/admin/accounts',
    name: 'AdminAccounts',
    component: () => import('@/views/admin/AccountsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Account Management',
      titleKey: 'admin.accounts.title',
      descriptionKey: 'admin.accounts.description',
      localeScopes: ['admin-accounts'],
      frame: 'workspace'
    }
  },
  {
    path: '/admin/plugins',
    name: 'AdminPlugins',
    component: () => import('@/views/admin/PluginsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Plugin Management',
      titleKey: 'admin.plugins.title',
      descriptionKey: 'admin.plugins.description',
      localeScopes: ['admin-settings', 'admin-plugins']
    }
  },
  {
    path: '/admin/announcements',
    name: 'AdminAnnouncements',
    component: () => import('@/views/admin/AnnouncementsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Announcements',
      titleKey: 'admin.announcements.title',
      descriptionKey: 'admin.announcements.description',
      frame: 'workspace'
    }
  },
  {
    path: '/admin/proxies',
    name: 'AdminProxies',
    component: () => import('@/views/admin/ProxiesView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Proxy Management',
      titleKey: 'admin.proxies.title',
      descriptionKey: 'admin.proxies.description',
      frame: 'workspace'
    }
  },
  {
    path: '/admin/proxies/risk',
    name: 'AdminIPRisk',
    component: () => import('@/views/admin/ProxiesView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'IP Risk Detection',
      titleKey: 'admin.ipRisk.title',
      descriptionKey: 'admin.ipRisk.description',
      frame: 'workspace'
    }
  },
  {
    path: '/admin/proxies/actions',
    name: 'AdminIPRiskActions',
    component: () => import('@/views/admin/ProxiesView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'IP Risk Actions',
      titleKey: 'admin.ipRisk.actionsView.title',
      descriptionKey: 'admin.ipRisk.actionsView.description',
      frame: 'workspace'
    }
  },
  {
    path: '/admin/redeem',
    name: 'AdminRedeem',
    component: () => import('@/views/admin/RedeemView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Redeem Code Management',
      titleKey: 'admin.redeem.title',
      descriptionKey: 'admin.redeem.description',
      frame: 'workspace'
    }
  },
  {
    path: '/admin/promo-codes',
    name: 'AdminPromoCodes',
    component: () => import('@/views/admin/PromoCodesView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Promo Code Management',
      titleKey: 'admin.promo.title',
      descriptionKey: 'admin.promo.description',
      frame: 'workspace'
    }
  },
  {
    path: '/admin/settings',
    name: 'AdminSettings',
    component: () => import('@/views/admin/SettingsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'System Settings',
      titleKey: 'admin.settings.title',
      descriptionKey: 'admin.settings.description',
      localeScopes: ['admin-settings'],
      frame: 'workspace'
    }
  },
  {
    path: '/admin/risk-control',
    name: 'AdminRiskControl',
    component: () => import('@/views/admin/RiskControlView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Risk Control',
      titleKey: 'admin.riskControl.title',
      descriptionKey: 'admin.riskControl.description',
      requiresRiskControl: true,
      frame: 'workspace'
    }
  },
  {
    path: adminPromptAuditPath,
    name: 'AdminPromptAudit',
    component: () => import('@/features/security-audit/PromptAuditRouteView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: '提示词审计',
      titleKey: 'admin.promptAudit.title',
      descriptionKey: 'admin.promptAudit.description',
      requiresRiskControl: true,
      frame: 'workspace'
    }
  },
  {
    path: '/admin/usage',
    name: 'AdminUsage',
    component: () => import('@/views/admin/UsageView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Usage Records',
      titleKey: 'admin.usage.title',
      descriptionKey: 'admin.usage.description',
      frame: 'workspace'
    }
  },
  {
    path: '/admin/affiliates',
    redirect: '/admin/affiliates/invites'
  },
  {
    path: '/admin/affiliates/invites',
    name: 'AdminAffiliateInvites',
    component: () => import('@/views/admin/affiliates/AdminAffiliateInvitesView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Affiliate Invite Records',
      titleKey: 'nav.affiliateInviteRecords',
      descriptionKey: 'admin.affiliates.invitesDescription',
      frame: 'workspace'
    }
  },
  {
    path: '/admin/affiliates/rebates',
    name: 'AdminAffiliateRebates',
    component: () => import('@/views/admin/affiliates/AdminAffiliateRebatesView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Affiliate Rebate Records',
      titleKey: 'nav.affiliateRebateRecords',
      descriptionKey: 'admin.affiliates.rebatesDescription',
      frame: 'workspace'
    }
  },
  {
    path: '/admin/affiliates/transfers',
    name: 'AdminAffiliateTransfers',
    component: () => import('@/views/admin/affiliates/AdminAffiliateTransfersView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Affiliate Transfer Records',
      titleKey: 'nav.affiliateTransferRecords',
      descriptionKey: 'admin.affiliates.transfersDescription',
      frame: 'workspace'
    }
  },


  // ==================== Payment Admin Routes ====================
  {
    path: '/admin/orders/dashboard',
    name: 'AdminPaymentDashboard',
    component: () => import('@/views/admin/orders/AdminPaymentDashboardView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Payment Dashboard',
      titleKey: 'nav.paymentDashboard',
      requiresPayment: true,
      frame: 'workspace'
    }
  },
  {
    path: '/admin/orders',
    name: 'AdminOrders',
    component: () => import('@/views/admin/orders/AdminOrdersView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Order Management',
      titleKey: 'nav.orderManagement',
      requiresPayment: true,
      frame: 'workspace'
    }
  },
  {
    path: '/admin/orders/plans',
    name: 'AdminPaymentPlans',
    component: () => import('@/views/admin/orders/AdminPaymentPlansView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Subscription Plans',
      titleKey: 'nav.paymentPlans',
      requiresPayment: true,
      frame: 'workspace'
    }
  },
  {
    path: '/admin/orders/play-billing',
    name: 'AdminPlayBillingConfig',
    component: () => import('@/views/admin/orders/AdminPlayBillingConfigView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Google Play Billing',
      titleKey: 'nav.playBilling',
      requiresPayment: true,
      frame: 'workspace'
    }
  },

  // ==================== 404 Not Found ====================
  {
    path: '/:pathMatch(.*)*',
    name: 'NotFound',
    component: () => import('@/views/NotFoundView.vue'),
    meta: {
      title: '404 Not Found'
    }
  }
]

/**
 * Create router instance
 */
const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes,
  scrollBehavior(_to, _from, savedPosition) {
    // Scroll to saved position when using browser back/forward
    if (savedPosition) {
      return savedPosition
    }
    // Scroll to top for new routes
    return { top: 0 }
  }
})

/**
 * Navigation guard: Authentication check
 */
let authInitialized = false

// 初始化导航加载状态
const navigationLoading = useNavigationLoadingState()
const navigationGeneration = createNavigationGeneration()
const BACKEND_MODE_ALLOWED_PATHS = [
  '/login',
  '/key-usage',
  '/setup',
  '/payment/result',
  '/payment/airwallex',
  '/legal',
  '/download/android',
  '/catalog',
  '/en/catalog',
]
const BACKEND_MODE_CALLBACK_PATHS = [
  '/auth/callback',
  '/auth/linuxdo/callback',
  '/auth/dingtalk/callback',
  '/auth/dingtalk/email-completion',
  '/auth/oidc/callback',
  '/auth/wechat/callback',
  '/auth/wechat/payment/callback',
]
const BACKEND_MODE_PENDING_AUTH_PATHS = ['/register', '/email-verify']

function isBackendModePublicRouteAllowed(path: string, hasPendingAuthSession: boolean): boolean {
  if (BACKEND_MODE_ALLOWED_PATHS.some((allowedPath) => path === allowedPath || path.startsWith(allowedPath))) {
    return true
  }

  if (BACKEND_MODE_CALLBACK_PATHS.some((callbackPath) => path === callbackPath)) {
    return true
  }

  if (hasPendingAuthSession && BACKEND_MODE_PENDING_AUTH_PATHS.some((allowedPath) => path === allowedPath)) {
    return true
  }

  return false
}

router.beforeEach(async (to, from, next) => {
  const guardGeneration = navigationGeneration.begin()
  const abortIfSuperseded = (): boolean => {
    if (navigationGeneration.isCurrent(guardGeneration)) return false
    next(false)
    return true
  }

  // 开始导航加载状态
  navigationLoading.startNavigation()

  // An explicit English workspace query is an opt-in session marker. Preserve it
  // across internal links, while a direct no-query visit still defaults to Chinese.
  const inheritedLocaleQuery = inheritedEnglishLocaleQuery(from.query, to.query, to.path)
  if (inheritedLocaleQuery) {
    next({ path: to.path, query: inheritedLocaleQuery, hash: to.hash })
    return
  }

  const localeTransitionApplied = await applyLocaleFromRoute(to.path, to.query)
  if (!localeTransitionApplied) {
    next(false)
    return
  }
  if (abortIfSuperseded()) return
  await ensureLocaleMessagesForRoute(to.name, undefined, to.meta.localeScopes)
  if (abortIfSuperseded()) return

  const authStore = useAuthStore()

  // Restore auth state from localStorage on first navigation (page refresh)
  if (!authInitialized) {
    authStore.checkAuth()
    authInitialized = true
  }

  // Set page title
  const appStore = useAppStore()
  const adminSettingsStore = useAdminSettingsStore()
  if (to.name === 'CustomPage') {
    const customMenuID = typeof to.params.id === 'string' ? to.params.id : ''
    const legacyDocsRoute = resolveLegacyDocsCustomPageRoute(customMenuID, getLocale())
    if (legacyDocsRoute) {
      next(legacyDocsRoute)
      return
    }
    if (!appStore.publicSettingsLoaded) {
      await appStore.fetchPublicSettings()
      if (abortIfSuperseded()) return
    }
    // AppSidebar normally loads admin settings after the first render. A
    // direct cold-start visit to an admin custom page can arrive before that
    // request completes, so wait for the shared in-flight fetch before
    // deciding whether the entry is a first-party native docs route.
    if (authStore.isAdmin && !adminSettingsStore.loaded) {
      await adminSettingsStore.fetch()
      if (abortIfSuperseded()) return
    }
    const nativeDocsRoute = resolveCustomMenuRouteById([
      ...(appStore.cachedPublicSettings?.custom_menu_items ?? []),
      ...(authStore.isAdmin ? adminSettingsStore.customMenuItems : []),
    ], customMenuID, getLocale())
    if (nativeDocsRoute) {
      next(nativeDocsRoute)
      return
    }
  }
  const customMenuItems = [
    ...(appStore.cachedPublicSettings?.custom_menu_items ?? []),
    ...(authStore.isAdmin ? adminSettingsStore.customMenuItems : []),
  ]
  if (!applyPublicRouteSeo(to.path)) {
    document.title = resolveRouteDocumentTitle(to, appStore.siteName, customMenuItems)
  }

  // Check if route requires authentication
  const requiresAuth = to.meta.requiresAuth !== false // Default to true
  const requiresAdmin = to.meta.requiresAdmin === true

  if (to.path === '/setup') {
    try {
      const status = await getSetupStatus()
      if (abortIfSuperseded()) return
      if (!status.needs_setup) {
        next(resolveCompletedSetupRedirectPath(authStore.isAuthenticated, authStore.isAdmin))
        return
      }
    } catch {
      // If setup status cannot be determined, keep the setup page reachable.
      if (abortIfSuperseded()) return
    }
  }

  // If route doesn't require auth, allow access
  if (!requiresAuth) {
    // If already authenticated and trying to access login/register, redirect to appropriate dashboard
    if (authStore.isAuthenticated && (to.path === '/login' || to.path === '/register')) {
      // In backend mode, non-admin users should NOT be redirected away from login
      // (they are blocked from all protected routes, so redirecting would cause a loop)
      if (appStore.backendModeEnabled && !authStore.isAdmin) {
        next()
        return
      }
      if (to.path === '/login' && to.query[FORUM_SSO_RESUME_PARAM]) {
        next()
        return
      }
      // Admin users go to admin dashboard, regular users go to user dashboard
      next(authStore.isAdmin ? '/admin/dashboard' : '/dashboard')
      return
    }
    // Model Plaza:公开路由但受「启用开关 + 可选强制登录」双重控制(后端同口径 fail-closed)
    const isModelPlazaRoute =
      to.path === '/model-plaza' ||
      to.path === '/catalog' ||
      to.path.startsWith('/catalog/') ||
      to.path === '/en/catalog' ||
      to.path.startsWith('/en/catalog/')
    if (isModelPlazaRoute) {
      if (!appStore.publicSettingsLoaded) {
        try {
          await appStore.fetchPublicSettings()
          if (abortIfSuperseded()) return
        } catch (error) {
          console.warn('Failed to load public settings in route guard', error)
          if (abortIfSuperseded()) return
        }
      }
      const plazaSettings = appStore.cachedPublicSettings
      // 仅在设置成功加载且明确为 false 时拦截(瞬时加载失败视为未知,由后端 404 兜底)
      if (appStore.publicSettingsLoaded && plazaSettings?.model_plaza_enabled === false) {
        next(
          authStore.isAuthenticated
            ? authStore.isAdmin
              ? '/admin/dashboard'
              : '/dashboard'
            : '/'
        )
        return
      }
      if (plazaSettings?.model_plaza_require_auth === true && !authStore.isAuthenticated) {
        next({ path: '/login', query: { redirect: to.fullPath } })
        return
      }
      // Backend mode:登录的非管理员也不可见(匿名由下方公共拦截处理,广场不在白名单)
      if (appStore.backendModeEnabled && authStore.isAuthenticated && !authStore.isAdmin) {
        next('/login')
        return
      }
    }
    // Backend mode: block public pages for unauthenticated users (except login, key-usage, setup)
    if (appStore.backendModeEnabled && !authStore.isAuthenticated) {
      const isAllowed = isBackendModePublicRouteAllowed(to.path, authStore.hasPendingAuthSession)
      if (!isAllowed) {
        next('/login')
        return
      }
    }
    next()
    return
  }

  // Route requires authentication
  if (!authStore.isAuthenticated) {
    // Not authenticated, redirect to login
    next({
      path: '/login',
      query: { redirect: to.fullPath } // Save intended destination
    })
    return
  }

  // Check admin requirement
  if (requiresAdmin && !authStore.isAdmin) {
    // User is authenticated but not admin, redirect to user dashboard
    next('/dashboard')
    return
  }

  if (requiresAdmin && authStore.isAdmin) {
    const adminComplianceStore = useAdminComplianceStore()
    if (!adminComplianceStore.initialized) {
      try {
        await adminComplianceStore.fetchStatus()
        if (abortIfSuperseded()) return
      } catch (error) {
        if (abortIfSuperseded()) return
        const err = error as { status?: number; code?: string; metadata?: Record<string, string> }
        if (err.status === 423 && err.code === 'ADMIN_COMPLIANCE_ACK_REQUIRED') {
          adminComplianceStore.requireAcknowledgement(err.metadata)
        }
      }
    }
  }


  // 公共设置可能尚未加载（App.vue 的 onMounted 异步拉取晚于首次导航，且纯静态部署
  // 无 __APP_CONFIG__ 注入）。此时 cachedPublicSettings 为空会把 payment/risk_control
  // 误判为“未启用”而错误拦截，故这里先确保设置加载完成。
  if (
    (to.meta.requiresPayment || to.meta.requiresRiskControl || to.meta.requiresNextChat) &&
    !appStore.publicSettingsLoaded
  ) {
    try {
      await appStore.fetchPublicSettings()
      if (abortIfSuperseded()) return
    } catch (error) {
      console.warn('Failed to load public settings before route guard:', error)
      if (abortIfSuperseded()) return
    }
  }

  // Only an explicit value from successfully loaded settings can disable a route.
  // A transient settings failure is unknown state, not a confirmed feature toggle.
  if (
    to.meta.requiresPayment &&
    appStore.publicSettingsLoaded &&
    appStore.cachedPublicSettings?.payment_enabled === false
  ) {
    next(authStore.isAdmin ? '/admin/dashboard' : '/dashboard')
    return
  }

  if (
    to.meta.requiresRiskControl &&
    appStore.publicSettingsLoaded &&
    appStore.cachedPublicSettings?.risk_control_enabled === false
  ) {
    next(authStore.isAdmin ? '/admin/settings' : '/dashboard')
    return
  }

  if (
    to.meta.requiresNextChat &&
    appStore.publicSettingsLoaded &&
    appStore.cachedPublicSettings?.nextchat_enabled !== true
  ) {
    next('/dashboard')
    return
  }

  // 简易模式下限制访问某些页面
  if (authStore.isSimpleMode) {
    const restrictedPaths = [
      '/admin/groups',
      '/admin/play-ops',
      '/admin/subscriptions',
      '/admin/redeem',
      '/subscriptions',
      '/redeem'
    ]

    if (restrictedPaths.some((path) => to.path.startsWith(path))) {
      // 简易模式下访问受限页面,重定向到仪表板
      next(authStore.isAdmin ? '/admin/dashboard' : '/dashboard')
      return
    }
  }

  // Backend mode: admin gets full access, non-admin blocked
  if (appStore.backendModeEnabled) {
    if (authStore.isAuthenticated && authStore.isAdmin) {
      next()
      return
    }
    const isAllowed = isBackendModePublicRouteAllowed(to.path, authStore.hasPendingAuthSession)
    if (!isAllowed) {
      next('/login')
      return
    }
  }

  // All checks passed, allow navigation
  next()
})

/**
 * Navigation guard: End loading
 */
router.afterEach(() => {
  // Keep html.dark in sync when crossing public vs authenticated layouts.
  useTheme().applyThemeClass()

  // 结束导航加载状态
  navigationLoading.endNavigation()

})

/**
 * Navigation guard: Error handling
 * Handles dynamic import failures caused by deployment updates
 */
router.onError((error, to) => {
  console.error('Router error:', error)
  recoverFromChunkLoadError(error, to?.fullPath)
})

export default router
