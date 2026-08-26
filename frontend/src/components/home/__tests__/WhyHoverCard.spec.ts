import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import WhyHoverCard from '@/components/home/WhyHoverCard.vue'
import { jisudengHomeEn } from '@/i18n/locales/jisudeng-home.en'
import { jisudengHomeZh } from '@/i18n/locales/jisudeng-home.zh'

const localeState = vi.hoisted(() => ({ locale: 'zh' as 'zh' | 'en' }))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => {
      const zh: Record<string, string> = {
        'home.jisudeng.features.hoverCard.tag': '优势',
        'home.jisudeng.features.hoverCard.labels.reliability': '稳定性',
        'home.jisudeng.features.hoverCard.labels.wallet': '钱包',
        'home.jisudeng.features.hoverCard.reliability.singleRoute': '单一路由',
        'home.jisudeng.features.hoverCard.reliability.risky': '风险高',
        'home.jisudeng.features.hoverCard.reliability.fallbackRetry': '故障重试',
        'home.jisudeng.features.hoverCard.reliability.redundantRoute': '冗余路由',
        'home.jisudeng.features.hoverCard.reliability.stable': '稳定',
        'home.jisudeng.features.hoverCard.reliability.availability': '99.97% 可用性',
        'home.jisudeng.features.hoverCard.wallet.synced': '钱包已同步',
        'home.jisudeng.features.hoverCard.wallet.balance': '钱包余额',
        'home.jisudeng.features.hoverCard.wallet.topUp': '充值',
        'home.jisudeng.features.hoverCard.wallet.withdraw': '提现',
        'home.jisudeng.features.hoverCard.wallet.history': '记录',
      }
      const en: Record<string, string> = {
        'home.jisudeng.features.hoverCard.tag': 'Why',
        'home.jisudeng.features.hoverCard.labels.privacy': 'Privacy',
        'home.jisudeng.features.hoverCard.labels.instant': 'Instant access',
        'home.jisudeng.features.hoverCard.privacy.title': 'Privacy',
        'home.jisudeng.features.hoverCard.privacy.localOnly': 'Local only',
        'home.jisudeng.features.hoverCard.privacy.logs': 'Logs',
        'home.jisudeng.features.hoverCard.privacy.logsValue': 'Kept inside your stack',
        'home.jisudeng.features.hoverCard.privacy.keys': 'Keys',
        'home.jisudeng.features.hoverCard.privacy.masked': 'Masked end-to-end',
        'home.jisudeng.features.hoverCard.privacy.policy': 'Policy',
        'home.jisudeng.features.hoverCard.privacy.policyValue': 'Least privilege by default',
        'home.jisudeng.features.hoverCard.privacy.noExfiltration': 'No exfiltration',
        'home.jisudeng.features.hoverCard.privacy.auditTrail': 'Audit trail',
        'home.jisudeng.features.hoverCard.instant.step': 'Step {number}',
        'home.jisudeng.features.hoverCard.instant.openGateway': 'Open the gateway',
        'home.jisudeng.features.hoverCard.instant.pasteKey': 'Paste key',
        'home.jisudeng.features.hoverCard.instant.ready': 'Ready',
      }
      const value = (localeState.locale === 'zh' ? zh : en)[key] ?? key
      return params
        ? value.replace(/\{(\w+)\}/g, (_, name: string) => String(params[name] ?? `{${name}}`))
        : value
    },
  }),
}))

function mountCard(locale: 'zh' | 'en', active: number) {
  localeState.locale = locale

  return mount(WhyHoverCard, {
    props: { active, x: 0, y: 0 },
  })
}

describe('WhyHoverCard localization', () => {
  it('keeps the rendered labels backed by distinct zh/en locale resources', () => {
    expect(jisudengHomeZh.features.hoverCard.reliability.singleRoute).toBe('单一路由')
    expect(jisudengHomeZh.features.hoverCard.wallet.balance).toBe('钱包余额')
    expect(jisudengHomeEn.features.hoverCard.reliability.singleRoute).toBe('Single route')
    expect(jisudengHomeEn.features.hoverCard.wallet.balance).toBe('Wallet balance')
  })

  it('renders Chinese interface labels without English fallback text', async () => {
    const wrapper = mountCard('zh', 1)

    expect(wrapper.text()).toContain('单一路由')
    expect(wrapper.text()).toContain('冗余路由')
    expect(wrapper.text()).toContain('99.97% 可用性')
    expect(wrapper.text()).not.toContain('single route')

    await wrapper.setProps({ active: 5 })
    expect(wrapper.text()).toContain('钱包余额')
    expect(wrapper.text()).toContain('充值')
    expect(wrapper.text()).not.toContain('wallet balance')
  })

  it('renders English interface labels for the English route', async () => {
    const wrapper = mountCard('en', 2)

    expect(wrapper.text()).toContain('Privacy')
    expect(wrapper.text()).toContain('Kept inside your stack')
    expect(wrapper.text()).toContain('Least privilege by default')

    await wrapper.setProps({ active: 3 })
    expect(wrapper.text()).toContain('Step 01')
    expect(wrapper.text()).toContain('Open the gateway')
  })
})
