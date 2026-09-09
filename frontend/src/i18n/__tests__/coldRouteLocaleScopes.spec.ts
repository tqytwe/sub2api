import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

import { describe, expect, it, vi } from 'vitest'

type Locale = 'zh' | 'en'

type RouteLiteralReference = {
  source: string
  key: string
}

type ColdRouteCase = {
  routeName: string
  requiredScope: string
  literals: readonly RouteLiteralReference[]
}

const COLD_ROUTE_CASES: readonly ColdRouteCase[] = [
  {
    routeName: 'Dashboard',
    requiredScope: 'workspace-shell',
    literals: [
      {
        source: 'src/components/layout/AppSidebar.vue',
        key: 'nav.fundManagement',
      },
      {
        source: 'src/components/layout/AppSidebar.vue',
        key: 'nav.refundRequests',
      },
      {
        source: 'src/components/layout/AppSidebar.vue',
        key: 'nav.fundCredits',
      },
      {
        source: 'src/components/layout/AppSidebar.vue',
        key: 'nav.fundOperationHistory',
      },
      {
        source: 'src/components/layout/AppSidebar.vue',
        key: 'nav.aiCreationSpace',
      },
    ],
  },
  {
    routeName: 'AdminPlayBillingConfig',
    requiredScope: 'admin-resources',
    literals: [
      {
        source: 'src/views/admin/orders/AdminPlayBillingConfigView.vue',
        key: 'payment.admin.playBilling.title',
      },
    ],
  },
  {
    routeName: 'AdminRiskControl',
    requiredScope: 'admin-channels',
    literals: [
      {
        source: 'src/views/admin/RiskControlView.vue',
        key: 'admin.riskControl.proxy',
      },
    ],
  },
  {
    routeName: 'AdminUsage',
    requiredScope: 'admin-ops',
    literals: [
      {
        source: 'src/views/admin/UsageView.vue',
        key: 'admin.ops.errorLog.type',
      },
    ],
  },
  {
    routeName: 'AdminAccounts',
    requiredScope: 'admin-accounts',
    literals: [
      {
        source: 'src/components/admin/account/AccountTestModal.vue',
        key: 'admin.accounts.testAccountConnection',
      },
      {
        source: 'src/utils/accountStatus.ts',
        key: 'admin.accounts.status.active',
      },
    ],
  },
  {
    routeName: 'AdminProxies',
    requiredScope: 'admin-accounts',
    literals: [
      {
        source: 'src/views/admin/ProxiesView.vue',
        key: 'admin.accounts.status.active',
      },
    ],
  },
  {
    routeName: 'AdminUsage',
    requiredScope: 'user-dashboard',
    literals: [
      {
        source: 'src/components/admin/usage/UsageStatsCards.vue',
        key: 'usage.totalRequests',
      },
      {
        source: 'src/views/admin/UsageView.vue',
        key: 'usage.tabs.usage',
      },
    ],
  },
  {
    routeName: 'Usage',
    requiredScope: 'user-usage',
    literals: [
      { source: 'src/views/user/UsageView.vue', key: 'admin.dashboard.timeRange' },
      { source: 'src/views/user/UsageView.vue', key: 'admin.dashboard.granularity' },
      { source: 'src/views/user/UsageView.vue', key: 'admin.dashboard.day' },
      { source: 'src/views/user/UsageView.vue', key: 'admin.dashboard.hour' },
      { source: 'src/views/user/UsageView.vue', key: 'admin.users.columnSettings' },
      { source: 'src/views/user/UsageView.vue', key: 'admin.usage.billingType' },
      { source: 'src/views/user/UsageView.vue', key: 'admin.usage.billingMode' },
      { source: 'src/views/user/UsageView.vue', key: 'admin.usage.group' },
      { source: 'src/views/user/UsageView.vue', key: 'admin.usage.allModels' },
      { source: 'src/views/user/UsageView.vue', key: 'admin.usage.allGroups' },
      { source: 'src/views/user/UsageView.vue', key: 'admin.usage.allTypes' },
      { source: 'src/views/user/UsageView.vue', key: 'admin.usage.allBillingTypes' },
      { source: 'src/views/user/UsageView.vue', key: 'admin.usage.billingTypeBalance' },
      { source: 'src/views/user/UsageView.vue', key: 'admin.usage.billingTypeSubscription' },
      { source: 'src/views/user/UsageView.vue', key: 'admin.usage.allBillingModes' },
      { source: 'src/views/user/UsageView.vue', key: 'admin.usage.billingModeToken' },
      { source: 'src/views/user/UsageView.vue', key: 'admin.usage.billingModePerRequest' },
      { source: 'src/views/user/UsageView.vue', key: 'admin.usage.billingModeImage' },
      { source: 'src/views/user/UsageView.vue', key: 'admin.usage.billingModeVideo' },
      { source: 'src/views/user/UsageView.vue', key: 'admin.usage.ipAddress' },
      { source: 'src/views/user/UsageView.vue', key: 'admin.usage.inputTokens' },
      { source: 'src/views/user/UsageView.vue', key: 'admin.usage.outputTokens' },
      { source: 'src/views/user/UsageView.vue', key: 'admin.usage.cacheReadTokens' },
      { source: 'src/views/user/UsageView.vue', key: 'admin.usage.cacheCreationTokens' },
    ],
  },
  {
    routeName: 'PurchaseSubscription',
    requiredScope: 'user-misc',
    literals: [
      {
        source: 'src/views/user/PaymentView.vue',
        key: 'payment.rechargeAccount',
      },
    ],
  },
]

function readPath(messages: Record<string, unknown>, path: string): unknown {
  return path.split('.').reduce<unknown>((node, key) => {
    if (!node || typeof node !== 'object') return undefined
    return (node as Record<string, unknown>)[key]
  }, messages)
}

async function loadFreshI18n() {
  // Every assertion must start without another route's loaded scopes. The
  // production module intentionally caches loaded scopes, so a normal shared
  // singleton test can hide an omitted dependency.
  vi.resetModules()
  return import('../index')
}

describe('cold-start route locale scopes', () => {
  it('does not dynamically import a monolithic locale bundle at runtime', () => {
    const indexSource = readFileSync(resolve(process.cwd(), 'src/i18n/index.ts'), 'utf8')

    expect(indexSource).not.toMatch(/import\(\s*['"]\.\/locales\/(?:zh|en)['"]\s*\)/)
  })

  it.each(COLD_ROUTE_CASES)(
    '$routeName loads its direct view copy from a fresh zh/en i18n instance',
    async ({ routeName, requiredScope, literals }) => {
      for (const { source, key } of literals) {
        const sourceText = readFileSync(resolve(process.cwd(), source), 'utf8')
        expect(sourceText, `${source} should directly reference ${key}`).toContain(`'${key}'`)
      }

      for (const locale of ['zh', 'en'] as const satisfies readonly Locale[]) {
        const {
          ensureLocaleMessagesForRoute,
          i18n,
          localeScopesForRouteName,
        } = await loadFreshI18n()

        const scopes = localeScopesForRouteName(routeName)
        expect(scopes).toContain(requiredScope)
        expect(scopes).not.toContain('full')

        const beforeLoad = i18n.global.getLocaleMessage(locale) as Record<string, unknown>
        for (const { key } of literals) {
          expect(readPath(beforeLoad, key), `${locale}:${routeName}:${key} before route load`).toBeUndefined()
        }

        await ensureLocaleMessagesForRoute(routeName, locale)
        i18n.global.locale.value = locale

        const messages = i18n.global.getLocaleMessage(locale) as Record<string, unknown>
        for (const { key } of literals) {
          const value = readPath(messages, key)
          expect(typeof value, `${locale}:${routeName}:${key}`).toBe('string')
          expect((value as string).trim(), `${locale}:${routeName}:${key}`).not.toBe('')
          // Vitest uses the runtime-only vue-i18n build, so translating an
          // uncompiled string intentionally returns the key. Inspect the
          // loaded message tree instead of treating that test-only runtime
          // limitation as a missing route fragment.
          expect(value, `${locale}:${routeName}:${key}`).not.toBe(key)
        }
      }
    },
  )
})
