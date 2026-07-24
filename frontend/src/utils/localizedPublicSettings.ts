import type { SupportContactConfig, SupportContactMethod } from '@/types'
import { normalizeSupportContactConfig } from '@/utils/supportContact'

export const JISUDENG_SITE_NAME_ZH = cjk([0x6781, 0x901f, 0x8e6c])
export const JISUDENG_SITE_NAME_EN = 'Jisudeng'
export const UPSTREAM_SITE_NAME = 'Sub2API'
export const UPSTREAM_SITE_SUBTITLE = 'Subscription to API Conversion Platform'

const CJK_TEXT_RE = /[\u3400-\u9fff\uf900-\ufaff]/

const SUPPORT_TEXT_EN = new Map<string, string>([
  [cjk([0x8054, 0x7cfb, 0x5ba2, 0x670d]), 'Contact support'],
  [
    cjk([
      0x767b, 0x5f55, 0x3001, 0x6ce8, 0x518c, 0x3001, 0x5145, 0x503c, 0x3001,
      0x41, 0x50, 0x49, 0x20, 0x6216, 0x6a21, 0x578b, 0x8c03, 0x7528,
      0x95ee, 0x9898, 0x90fd, 0x53ef, 0x4ee5, 0x8054, 0x7cfb, 0x4eba,
      0x5de5, 0x5ba2, 0x670d,
    ]),
    'Login, signup, billing, API, and model-call issues can all be handled by support.',
  ],
  [cjk([0x5ba2, 0x670d, 0x8054, 0x7cfb, 0x65b9, 0x5f0f]), 'Support contact'],
  [cjk([0x6587, 0x6863, 0x94fe, 0x63a5]), 'Documentation'],
  [cjk([0x5fae, 0x4fe1, 0x5ba2, 0x670d]), 'WeChat support'],
  [cjk([0x5fae, 0x4fe1, 0x670d, 0x52a1, 0x7fa4]), 'WeChat support group'],
  [cjk([0x51, 0x51, 0x20, 0x5ba2, 0x670d]), 'QQ support'],
  [cjk([0x51, 0x51, 0x20, 0x5ba2, 0x670d, 0x7fa4]), 'QQ support group'],
  [cjk([0x90ae, 0x7bb1]), 'Email support'],
  [cjk([0x63a8, 0x8350, 0x4f18, 0x5148, 0x6dfb, 0x52a0, 0x5fae, 0x4fe1]), 'Recommended first contact'],
  [
    cjk([0x9002, 0x5408, 0x5feb, 0x901f, 0x590d, 0x5236, 0x20, 0x51, 0x51, 0x20, 0x53f7, 0x6dfb, 0x52a0]),
    'Quickly copy the QQ number to add support',
  ],
])

function cjk(codes: readonly number[]): string {
  return String.fromCharCode(...codes)
}

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
    contacts: normalized.contacts.map((contact) => localizedSupportContactMethod(contact, true)),
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
      return cjk([0x5fae, 0x4fe1])
    case 'qq':
      return 'QQ'
    case 'telegram':
      return 'TG'
    case 'email':
      return cjk([0x90ae, 0x7bb1])
    case 'docs':
      return cjk([0x6587, 0x6863])
    default:
      return cjk([0x5ba2, 0x670d])
  }
}

function localizedSupportContactMethod(
  contact: SupportContactMethod,
  stripQrImage: boolean,
): SupportContactMethod {
  return {
    ...contact,
    qr_image: stripQrImage ? '' : contact.qr_image,
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
