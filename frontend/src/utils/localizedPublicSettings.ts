import type { SupportContactConfig, SupportContactMethod } from '@/types'
import { normalizeSupportContactConfig } from '@/utils/supportContact'

export const JISUDENG_SITE_NAME_ZH = '极速蹬'
export const JISUDENG_SITE_NAME_EN = 'Jisudeng'
export const UPSTREAM_SITE_NAME = 'Sub2API'
export const UPSTREAM_SITE_SUBTITLE = 'Subscription to API Conversion Platform'

const CJK_TEXT_RE = /[\u3400-\u9fff\uf900-\ufaff]/

const SUPPORT_TEXT_EN = new Map<string, string>([
  ['联系客服', 'Contact support'],
  ['登录、注册、充值、API 或模型调用问题都可以联系人工客服', 'Login, signup, billing, API, and model-call issues can all be handled by support.'],
  ['客服联系方式', 'Support contact'],
  ['文档链接', 'Documentation'],
  ['微信客服', 'WeChat support'],
  ['微信服务群', 'WeChat support group'],
  ['QQ 客服', 'QQ support'],
  ['QQ 客服群', 'QQ support group'],
  ['邮箱', 'Email support'],
  ['推荐优先添加微信', 'Recommended first contact'],
  ['适合快速复制 QQ 号添加', 'Quickly copy the QQ number to add support'],
])

export function isEnglishLocale(locale?: string | null): boolean {
  return (locale || '').toLowerCase().startsWith('en')
}

export function hasCjkText(value?: string | null): boolean {
  return CJK_TEXT_RE.test((value || '').trim())
}

export function localizedSiteName(value: string | null | undefined, locale?: string | null): string {
  const trimmed = value?.trim() || ''
  if (isEnglishLocale(locale)) {
    if (!trimmed || trimmed === UPSTREAM_SITE_NAME || hasCjkText(trimmed)) {
      return JISUDENG_SITE_NAME_EN
    }
    return trimmed
  }

  if (!trimmed || trimmed === UPSTREAM_SITE_NAME) {
    return JISUDENG_SITE_NAME_ZH
  }
  return trimmed
}

export function localizedSiteSubtitle(
  value: string | null | undefined,
  locale: string | null | undefined,
  fallback: string,
): string {
  const trimmed = value?.trim() || ''
  const fallbackText = fallback.trim()

  if (isEnglishLocale(locale)) {
    if (!trimmed || trimmed === UPSTREAM_SITE_SUBTITLE || hasCjkText(trimmed)) {
      return fallbackText
    }
    return trimmed
  }

  if (trimmed && trimmed !== UPSTREAM_SITE_SUBTITLE) {
    return trimmed
  }
  return fallbackText
}

export function localizedSupportContactConfig(
  config: SupportContactConfig | null | undefined,
  locale?: string | null,
): SupportContactConfig {
  const normalized = normalizeSupportContactConfig(config)
  if (!isEnglishLocale(locale)) {
    return normalized
  }

  return {
    ...normalized,
    title: localizedSupportText(normalized.title, 'Contact support'),
    subtitle: localizedSupportText(
      normalized.subtitle,
      'Login, signup, billing, API, and model-call issues can all be handled by support.',
    ),
    contacts: normalized.contacts.map(localizedSupportContactMethod),
  }
}

export function localizedSupportContactTypeLabel(type: string, locale?: string | null): string {
  const normalizedType = type.trim().toLowerCase()
  if (isEnglishLocale(locale)) {
    switch (normalizedType) {
      case 'wechat':
        return 'WeChat'
      case 'qq':
        return 'QQ'
      case 'telegram':
        return 'Telegram'
      case 'email':
        return 'Email'
      case 'docs':
        return 'Docs'
      default:
        return 'Support'
    }
  }

  switch (normalizedType) {
    case 'wechat':
      return '微信'
    case 'qq':
      return 'QQ'
    case 'telegram':
      return 'TG'
    case 'email':
      return '邮箱'
    case 'docs':
      return '文档'
    default:
      return '客服'
  }
}

function localizedSupportContactMethod(contact: SupportContactMethod): SupportContactMethod {
  return {
    ...contact,
    label: localizedSupportText(contact.label, fallbackContactLabel(contact.type)),
    description: localizedSupportDescription(contact),
  }
}

function localizedSupportDescription(contact: SupportContactMethod): string {
  return localizedSupportText(contact.description, fallbackContactDescription(contact.type))
}

function localizedSupportText(value: string, fallback: string): string {
  const trimmed = value.trim()
  if (!trimmed) return fallback
  if (!hasCjkText(trimmed)) return trimmed
  return SUPPORT_TEXT_EN.get(trimmed.replace(/\s+/g, ' ')) || fallback
}

function fallbackContactLabel(type: string): string {
  switch (type.trim().toLowerCase()) {
    case 'wechat':
      return 'WeChat support group'
    case 'qq':
      return 'QQ support group'
    case 'telegram':
      return 'Telegram support'
    case 'email':
      return 'Email support'
    case 'docs':
      return 'Documentation'
    default:
      return 'Support contact'
  }
}

function fallbackContactDescription(type: string): string {
  switch (type.trim().toLowerCase()) {
    case 'wechat':
      return 'Recommended first contact'
    case 'qq':
      return 'Quickly copy the QQ number to add support'
    case 'telegram':
      return 'Message support on Telegram'
    case 'email':
      return 'Send an email to support'
    case 'docs':
      return 'Open the documentation'
    default:
      return ''
  }
}
