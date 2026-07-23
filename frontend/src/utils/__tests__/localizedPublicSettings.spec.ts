import { describe, expect, it } from 'vitest'

import type { SupportContactConfig } from '@/types'
import {
  JISUDENG_SITE_NAME_EN,
  JISUDENG_SITE_NAME_ZH,
  localizedSiteName,
  localizedSiteSubtitle,
  localizedSupportContactConfig,
  localizedSupportContactTypeLabel,
} from '@/utils/localizedPublicSettings'

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

function flattenSupportConfig(config: SupportContactConfig): string {
  return [
    config.title,
    config.subtitle,
    ...config.contacts.flatMap((contact) => [contact.label, contact.description]),
  ].join('\n')
}

describe('localized public settings', () => {
  it('shows the English brand when Chinese settings are rendered in English', () => {
    expect(localizedSiteName('极速蹬', 'en')).toBe(JISUDENG_SITE_NAME_EN)
    expect(localizedSiteName('Sub2API', 'en')).toBe(JISUDENG_SITE_NAME_EN)
    expect(localizedSiteName('Acme API', 'en')).toBe('Acme API')
  })

  it('keeps the Chinese brand for Chinese locale', () => {
    expect(localizedSiteName('极速蹬', 'zh')).toBe(JISUDENG_SITE_NAME_ZH)
    expect(localizedSiteName('Sub2API', 'zh')).toBe(JISUDENG_SITE_NAME_ZH)
    expect(localizedSiteName('自定义站点', 'zh')).toBe('自定义站点')
  })

  it('suppresses Chinese subtitles in English while preserving English custom subtitles', () => {
    expect(
      localizedSiteSubtitle('最安全的大模型中转平台', 'en', 'The most privacy-focused LLM relay platform'),
    ).toBe('The most privacy-focused LLM relay platform')
    expect(localizedSiteSubtitle('Private model gateway', 'en', 'fallback')).toBe('Private model gateway')
    expect(localizedSiteSubtitle('最安全的大模型中转平台', 'zh', '默认中文副标题')).toBe('最安全的大模型中转平台')
  })

  it('localizes Chinese support contact presentation for English locale', () => {
    const localized = localizedSupportContactConfig(chineseSupportContact, 'en')
    const copy = flattenSupportConfig(localized)

    expect(localized.title).toBe('Contact support')
    expect(localized.subtitle).toBe('Login, signup, billing, API, and model-call issues can all be handled by support.')
    expect(localized.contacts[0]?.label).toBe('WeChat support group')
    expect(localized.contacts[0]?.description).toBe('Recommended first contact')
    expect(localized.contacts[1]?.label).toBe('QQ support group')
    expect(localized.contacts[1]?.description).toBe('Quickly copy the QQ number to add support')
    expect(copy).not.toMatch(/[\u3400-\u9fff\uf900-\ufaff]/)
  })

  it('preserves Chinese support contact settings for Chinese locale', () => {
    const localized = localizedSupportContactConfig(chineseSupportContact, 'zh')

    expect(localized.title).toBe('联系客服')
    expect(localized.subtitle).toBe('登录、注册、充值、API 或模型调用问题都可以联系人工客服')
    expect(localized.contacts[0]?.label).toBe('微信服务群')
    expect(localized.contacts[1]?.description).toBe('适合快速复制 QQ 号添加')
  })

  it('localizes support contact type badges', () => {
    expect(localizedSupportContactTypeLabel('wechat', 'en')).toBe('WeChat')
    expect(localizedSupportContactTypeLabel('email', 'en')).toBe('Email')
    expect(localizedSupportContactTypeLabel('wechat', 'zh')).toBe('微信')
    expect(localizedSupportContactTypeLabel('email', 'zh')).toBe('邮箱')
  })
})
