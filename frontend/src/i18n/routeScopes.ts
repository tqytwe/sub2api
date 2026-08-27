/**
 * Locale fragments required before a named route can render.  Keep this list
 * independent from the router so it can be loaded during bootstrap as well as
 * during navigation guards.
 */
export const LOCALE_LOAD_SCOPES = [
  'core',
  'public-pages',
  'workspace-shell',
  'admin-shell',
  'user-dashboard',
  'user-usage',
  'user-wallet',
  'user-batch',
  'user-misc',
  'channel-monitor',
  'admin-overview',
  'admin-accounts',
  'admin-channels',
  'admin-ops',
  'admin-play',
  'admin-resources',
  'admin-settings',
  'admin-plugins',
  'admin-audit',
  'admin-prompt-audit',
] as const

export type LocaleLoadScope = typeof LOCALE_LOAD_SCOPES[number]

type RouteLocaleScope = readonly LocaleLoadScope[]

export type LocaleRouteName =
  | 'Setup'
  | 'EnglishHome'
  | 'EnglishModels'
  | 'EnglishModelFamily'
  | 'EnglishDocs'
  | 'EnglishAbout'
  | 'EnglishContact'
  | 'Pricing'
  | 'AndroidDownload'
  | 'Login'
  | 'Register'
  | 'EmailVerify'
  | 'OAuthCallback'
  | 'LinuxDoOAuthCallback'
  | 'WeChatOAuthCallback'
  | 'WeChatPaymentOAuthCallback'
  | 'DingTalkOAuthCallback'
  | 'dingtalk-email-completion'
  | 'OIDCOAuthCallback'
  | 'ForgotPassword'
  | 'ResetPassword'
  | 'KeyUsage'
  | 'LegalDocument'
  | 'About'
  | 'Contact'
  | 'ContactQQ'
  | 'Docs'
  | 'Blindbox'
  | 'Arena'
  | 'QuizQuest'
  | 'Models'
  | 'ModelFamily'
  | 'AgentTeam'
  | 'Home'
  | 'Dashboard'
  | 'Keys'
  | 'KeySpeedTest'
  | 'BatchImageGuide'
  | 'Usage'
  | 'Wallet'
  | 'Redeem'
  | 'AICreationSpace'
  | 'PlayHub'
  | 'CheckIn'
  | 'Affiliate'
  | 'UserAvailableChannels'
  | 'Profile'
  | 'Subscriptions'
  | 'PurchaseSubscription'
  | 'OrderList'
  | 'PaymentQRCode'
  | 'PaymentResult'
  | 'StripePayment'
  | 'AirwallexPayment'
  | 'StripePopup'
  | 'CustomPage'
  | 'AdminDashboard'
  | 'AdminOps'
  | 'AdminPlayOps'
  | 'AdminFunds'
  | 'AdminFundsTab'
  | 'AdminWithdrawals'
  | 'AdminAuditLogs'
  | 'AdminUsers'
  | 'AdminGroups'
  | 'AdminChannels'
  | 'AdminChannelMonitor'
  | 'AdminModelPlaza'
  | 'ChannelStatus'
  | 'AdminSubscriptions'
  | 'AdminAccounts'
  | 'AdminPlugins'
  | 'AdminAnnouncements'
  | 'AdminProxies'
  | 'AdminIPRisk'
  | 'AdminIPRiskActions'
  | 'AdminRedeem'
  | 'AdminPromoCodes'
  | 'AdminSettings'
  | 'AdminRiskControl'
  | 'AdminPromptAudit'
  | 'AdminUsage'
  | 'AdminAffiliateInvites'
  | 'AdminAffiliateRebates'
  | 'AdminAffiliateTransfers'
  | 'AdminPaymentDashboard'
  | 'AdminOrders'
  | 'AdminPaymentPlans'
  | 'AdminPlayBillingConfig'
  | 'NotFound'

const PUBLIC = ['core', 'public-pages'] as const
const WORKSPACE = ['core', 'workspace-shell'] as const
const ADMIN = ['core', 'workspace-shell', 'admin-shell'] as const

/**
 * This is deliberately exhaustive.  A route without an entry cannot rely on
 * a previous page having happened to load its messages.
 */
export const ROUTE_LOCALE_SCOPES = {
  Setup: ['core'],
  EnglishHome: ['core'],
  EnglishModels: [...PUBLIC, 'user-dashboard'],
  EnglishModelFamily: [...PUBLIC, 'user-dashboard'],
  EnglishDocs: PUBLIC,
  EnglishAbout: PUBLIC,
  EnglishContact: PUBLIC,
  Pricing: [...PUBLIC, 'user-dashboard'],
  AndroidDownload: PUBLIC,
  Login: ['core'],
  Register: ['core'],
  EmailVerify: ['core'],
  OAuthCallback: ['core'],
  LinuxDoOAuthCallback: ['core'],
  WeChatOAuthCallback: ['core'],
  WeChatPaymentOAuthCallback: ['core'],
  DingTalkOAuthCallback: ['core'],
  'dingtalk-email-completion': ['core'],
  OIDCOAuthCallback: ['core'],
  ForgotPassword: ['core'],
  ResetPassword: ['core'],
  KeyUsage: ['core'],
  LegalDocument: ['core'],
  About: PUBLIC,
  Contact: PUBLIC,
  ContactQQ: PUBLIC,
  Docs: PUBLIC,
  Blindbox: [...PUBLIC, 'user-dashboard'],
  Arena: [...PUBLIC, 'user-dashboard'],
  QuizQuest: [...PUBLIC, 'user-dashboard'],
  Models: [...PUBLIC, 'user-dashboard'],
  ModelFamily: [...PUBLIC, 'user-dashboard'],
  AgentTeam: [...PUBLIC, 'user-dashboard'],
  Home: ['core'],
  Dashboard: [...WORKSPACE, 'user-dashboard'],
  Keys: [...WORKSPACE, 'user-dashboard'],
  KeySpeedTest: [...WORKSPACE, 'user-dashboard'],
  BatchImageGuide: [...WORKSPACE, 'user-dashboard', 'user-batch'],
  // The user usage page reuses a compact admin-namespaced label subset. Keep
  // it separate from the much larger admin resources fragment.
  Usage: [...WORKSPACE, 'user-dashboard', 'user-usage'],
  Wallet: [...WORKSPACE, 'user-dashboard', 'user-wallet', 'user-misc'],
  Redeem: [...WORKSPACE, 'user-dashboard'],
  // Image Studio and prompt-library copy are public-page fragments even when
  // the route is authenticated, so keep that dependency explicit.
  AICreationSpace: [...WORKSPACE, 'public-pages', 'user-dashboard', 'user-misc'],
  PlayHub: [...WORKSPACE, 'public-pages', 'user-dashboard'],
  CheckIn: [...WORKSPACE, 'public-pages', 'user-dashboard'],
  Affiliate: [...WORKSPACE, 'user-dashboard'],
  UserAvailableChannels: [...WORKSPACE, 'user-dashboard'],
  Profile: [...WORKSPACE, 'user-dashboard'],
  Subscriptions: [...WORKSPACE, 'user-dashboard', 'user-misc'],
  PurchaseSubscription: [...WORKSPACE, 'user-misc'],
  OrderList: [...WORKSPACE, 'user-misc'],
  PaymentQRCode: [...WORKSPACE, 'user-misc'],
  PaymentResult: ['core', 'user-misc'],
  StripePayment: [...WORKSPACE, 'user-misc'],
  AirwallexPayment: [...WORKSPACE, 'user-misc'],
  StripePopup: ['core', 'user-misc'],
  CustomPage: [...WORKSPACE, 'user-misc'],
  AdminDashboard: [...ADMIN, 'admin-overview'],
  AdminOps: [...ADMIN, 'admin-ops'],
  AdminPlayOps: [...ADMIN, 'admin-play', 'admin-resources'],
  AdminFunds: [...ADMIN, 'admin-play', 'admin-resources'],
  AdminFundsTab: [...ADMIN, 'admin-play', 'admin-resources'],
  AdminWithdrawals: [...ADMIN, 'admin-play', 'admin-resources'],
  AdminAuditLogs: [...ADMIN, 'admin-resources', 'admin-audit'],
  AdminUsers: [...ADMIN, 'admin-resources'],
  // Groups reuse account status labels in their filters and tables.
  AdminGroups: [...ADMIN, 'admin-resources', 'admin-accounts'],
  AdminChannels: [...ADMIN, 'admin-channels'],
  AdminChannelMonitor: [...ADMIN, 'admin-channels', 'channel-monitor'],
  AdminModelPlaza: [...ADMIN, 'user-dashboard', 'admin-channels'],
  ChannelStatus: [...WORKSPACE, 'user-dashboard', 'channel-monitor'],
  AdminSubscriptions: [...ADMIN, 'admin-resources'],
  AdminAccounts: [...ADMIN, 'admin-accounts'],
  AdminPlugins: [...ADMIN, 'admin-settings', 'admin-plugins'],
  AdminAnnouncements: [...ADMIN, 'admin-resources'],
  // Proxies reuse account status labels in filter controls and row badges.
  AdminProxies: [...ADMIN, 'admin-resources', 'admin-accounts'],
  AdminIPRisk: [...ADMIN, 'admin-resources', 'admin-accounts'],
  AdminIPRiskActions: [...ADMIN, 'admin-resources', 'admin-accounts'],
  AdminRedeem: [...ADMIN, 'admin-resources'],
  AdminPromoCodes: [...ADMIN, 'admin-resources'],
  AdminSettings: [...ADMIN, 'admin-settings'],
  AdminRiskControl: [...ADMIN, 'admin-resources', 'admin-channels'],
  AdminPromptAudit: [...ADMIN, 'admin-channels', 'admin-prompt-audit'],
  // Usage also mounts shared operational error-log controls.
  AdminUsage: [...ADMIN, 'admin-resources', 'user-dashboard', 'admin-ops'],
  AdminAffiliateInvites: [...ADMIN, 'admin-resources'],
  AdminAffiliateRebates: [...ADMIN, 'admin-resources'],
  AdminAffiliateTransfers: [...ADMIN, 'admin-resources'],
  AdminPaymentDashboard: [...ADMIN, 'admin-resources'],
  AdminOrders: [...ADMIN, 'admin-resources'],
  AdminPaymentPlans: [...ADMIN, 'admin-resources'],
  AdminPlayBillingConfig: [...ADMIN, 'admin-resources'],
  NotFound: ['core', 'public-pages'],
} as const satisfies Record<LocaleRouteName, RouteLocaleScope>

export function localeScopesForRouteName(routeName: unknown): LocaleLoadScope[] {
  if (typeof routeName !== 'string' || !(routeName in ROUTE_LOCALE_SCOPES)) {
    return ['core']
  }

  return [...ROUTE_LOCALE_SCOPES[routeName as LocaleRouteName]]
}
