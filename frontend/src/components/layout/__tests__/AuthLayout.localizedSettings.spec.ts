import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import AuthLayout from '@/components/layout/AuthLayout.vue'
import SupportContactPanel from '@/components/common/SupportContactPanel.vue'
import { useAppStore } from '@/stores'
import type { SupportContactConfig } from '@/types'

const chineseSupportContact: SupportContactConfig = {
  title: '联系客服',
  subtitle: '登录、注册、充值、API 或模型调用问题都可以联系人工客服',
  contacts: [
    {
      id: 'wechat',
      type: 'wechat',
      label: '微信服务群',
      value: 'tqytwemx',
      copy_value: 'tqytwemx',
      url: '',
      qr_image: '/uploads/wechat.png',
      description: '推荐优先添加微信',
      primary: true,
      enabled: true,
      sort_order: 1,
    },
    {
      id: 'qq',
      type: 'qq',
      label: 'QQ 客服群',
      value: '1570539180',
      copy_value: '1570539180',
      url: '',
      qr_image: '/uploads/qq.png',
      description: '适合快速复制 QQ 号添加',
      primary: true,
      enabled: true,
      sort_order: 2,
    },
  ],
}

function i18nFor(locale: 'en' | 'zh') {
  return createI18n({
    legacy: false,
    locale,
    fallbackLocale: 'zh',
    messages: {
      en: {
        authAside: {
          siteSubtitleDefault: () => 'The most privacy-focused LLM relay platform',
          eyebrow: () => 'Privacy · Transparent · One thought away',
          titleLogin: () => 'A light thought',
          titleFaintLogin: () => 'Echoes every model',
          pledge1: () => 'No request storage · No training',
          pledge2: () => 'Full TLS · Fully auditable',
          pledge3: () => 'Instant access · No approval',
          backHome: () => 'Back home',
          copyrightTagline: () => 'One thought links every model',
        },
        common: {
          copy: () => 'Copy',
          open: () => 'Open',
          copiedToClipboard: () => 'Copied to clipboard',
          copyFailed: () => 'Failed to copy',
        },
        supportContactPanel: {
          moreContacts: () => 'More contact methods',
        },
      },
      zh: {
        authAside: {
          siteSubtitleDefault: () => '最安全的大模型中转平台',
          eyebrow: () => '隐私 · 透明 · 一念可达',
          titleLogin: () => '一念之轻',
          titleFaintLogin: () => '接万模之响',
          pledge1: () => '不存请求体 · 不参与训练',
          pledge2: () => '全链路 TLS · 全程可审计',
          pledge3: () => '注册即开 · 秒级接入',
          backHome: () => '返回首页',
          copyrightTagline: () => '让一念之间链接每个模型',
        },
        common: {
          copy: () => '复制',
          open: () => '打开',
          copiedToClipboard: () => '已复制到剪贴板',
          copyFailed: () => '复制失败',
        },
        supportContactPanel: {
          moreContacts: () => '更多联系方式',
        },
      },
    },
  } as any)
}

function seedPublicSettings(locale: 'en' | 'zh') {
  setActivePinia(createPinia())
  const appStore = useAppStore()
  appStore.siteName = '极速蹬'
  appStore.supportContact = chineseSupportContact
  appStore.publicSettingsLoaded = true
  appStore.cachedPublicSettings = {
    site_name: '极速蹬',
    site_logo: '',
    site_subtitle: '最安全的大模型中转平台',
  } as any
  vi.spyOn(appStore, 'fetchPublicSettings').mockResolvedValue(null)
  return { appStore, i18n: i18nFor(locale) }
}

const globalStubs = {
  Icon: { template: '<span />' },
  PublicPageToolbar: { template: '<div />' },
  RouterLink: { template: '<a><slot /></a>' },
}

describe('AuthLayout localized public settings', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('renders English auth settings without Chinese config labels', () => {
    const { i18n } = seedPublicSettings('en')
    const wrapper = mount(AuthLayout, {
      props: { asideMode: 'login' },
      slots: { default: '<div>FORM</div>' },
      global: {
        plugins: [i18n],
        stubs: globalStubs,
      },
    })

    const text = wrapper.text()
    expect(text).toContain('Jisudeng')
    expect(text).toContain('The most privacy-focused LLM relay platform')
    expect(text).toContain('Contact support')
    expect(text).toContain('WeChat support group')
    expect(text).toContain('QQ support group')
    expect(text).not.toContain('极速蹬')
    expect(text).not.toContain('最安全的大模型中转平台')
    expect(text).not.toContain('联系客服')
    expect(text).not.toContain('微信服务群')
    expect(text).not.toContain('QQ 客服群')
  })

  it('keeps Chinese auth settings in Chinese locale', () => {
    const { i18n } = seedPublicSettings('zh')
    const wrapper = mount(AuthLayout, {
      props: { asideMode: 'login' },
      slots: { default: '<div>FORM</div>' },
      global: {
        plugins: [i18n],
        stubs: globalStubs,
      },
    })

    const text = wrapper.text()
    expect(text).toContain('极速蹬')
    expect(text).toContain('最安全的大模型中转平台')
    expect(text).toContain('联系客服')
    expect(text).toContain('微信服务群')
    expect(text).toContain('QQ 客服群')
  })
})

describe('SupportContactPanel localized public settings', () => {
  it('localizes shared support panel actions and badges in English', () => {
    const wrapper = mount(SupportContactPanel, {
      props: { config: chineseSupportContact },
      global: {
        plugins: [i18nFor('en'), createPinia()],
        stubs: globalStubs,
      },
    })

    const text = wrapper.text()
    expect(text).toContain('WeChat')
    expect(text).toContain('QQ')
    expect(wrapper.find('[title="Copy"]').exists()).toBe(true)
    expect(text).not.toContain('微信')
    expect(text).not.toContain('复制')
  })
})
