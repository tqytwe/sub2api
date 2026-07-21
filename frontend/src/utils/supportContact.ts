import type { SupportContactConfig, SupportContactMethod } from '@/types'
import { sanitizeUrl } from '@/utils/url'

export const defaultSupportContactTitle = '联系客服'
export const defaultSupportContactSubtitle = '登录、注册、充值、API 或模型调用问题都可以联系人工客服'

export function emptySupportContactConfig(): SupportContactConfig {
  return {
    title: defaultSupportContactTitle,
    subtitle: defaultSupportContactSubtitle,
    contacts: [],
  }
}

export function normalizeSupportContactConfig(
  config?: SupportContactConfig | null,
  legacyContactInfo = '',
  legacyDocUrl = '',
): SupportContactConfig {
  const normalized: SupportContactConfig = {
    title: config?.title?.trim() || defaultSupportContactTitle,
    subtitle: config?.subtitle?.trim() || defaultSupportContactSubtitle,
    contacts: Array.isArray(config?.contacts)
      ? config!.contacts
          .map((contact, index) => normalizeSupportContactMethod(contact, index))
          .filter((contact): contact is SupportContactMethod => !!contact)
          .sort((a, b) => a.sort_order - b.sort_order)
      : [],
  }

  if (normalized.contacts.length > 0) return normalized

  const fallbackContacts: SupportContactMethod[] = []
  const contactInfo = legacyContactInfo.trim()
  const docUrl = legacyDocUrl.trim()
  if (contactInfo) {
    fallbackContacts.push({
      id: 'legacy-contact',
      type: 'custom',
      label: '客服联系方式',
      value: contactInfo,
      copy_value: contactInfo,
      url: '',
      qr_image: '',
      description: '',
      primary: false,
      enabled: true,
      sort_order: 1,
    })
  }
  if (docUrl) {
    fallbackContacts.push({
      id: 'legacy-docs',
      type: 'docs',
      label: '文档链接',
      value: docUrl,
      copy_value: '',
      url: docUrl,
      qr_image: '',
      description: '',
      primary: false,
      enabled: true,
      sort_order: 2,
    })
  }
  return { ...normalized, contacts: fallbackContacts }
}

export function enabledSupportContacts(config?: SupportContactConfig | null): SupportContactMethod[] {
  return normalizeSupportContactConfig(config).contacts
    .filter((contact) => contact.enabled)
    .sort((a, b) => a.sort_order - b.sort_order)
}

export function primaryQRCodeContacts(config?: SupportContactConfig | null): SupportContactMethod[] {
  return enabledSupportContacts(config)
    .filter((contact) => contact.primary && !!sanitizeSupportContactImage(contact.qr_image))
    .slice(0, 2)
}

export function supportContactCopyValue(contact: SupportContactMethod): string {
  return (contact.copy_value || contact.value || contact.url || '').trim()
}

export function supportContactDisplayValue(contact: SupportContactMethod): string {
  return (contact.value || contact.copy_value || contact.url || '').trim()
}

export function supportContactActionUrl(contact: SupportContactMethod): string {
  const explicitUrl = sanitizeSupportContactUrl(contact.url)
  if (explicitUrl) return explicitUrl

  const value = supportContactDisplayValue(contact)
  if (contact.type === 'email' && value && /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value)) {
    return `mailto:${value}`
  }
  if (contact.type === 'telegram' && value) {
    const handle = value.replace(/^@/, '').trim()
    if (/^[a-zA-Z0-9_]{5,32}$/.test(handle)) {
      return `https://t.me/${handle}`
    }
  }
  return ''
}

export function supportContactDocsUrl(config?: SupportContactConfig | null): string {
  const docsContact = enabledSupportContacts(config).find((contact) => contact.type === 'docs')
  return docsContact ? supportContactActionUrl(docsContact) : ''
}

export function sanitizeSupportContactUrl(value: string): string {
  const trimmed = value.trim()
  if (!trimmed) return ''
  if (trimmed.startsWith('mailto:')) {
    return /^mailto:[^\s@]+@[^\s@]+\.[^\s@]+$/i.test(trimmed) ? trimmed : ''
  }
  return sanitizeUrl(trimmed, { allowRelative: true })
}

export function sanitizeSupportContactImage(value: string): string {
  const trimmed = value.trim()
  if (!trimmed) return ''
  if (trimmed.startsWith('/') && !trimmed.startsWith('//')) return trimmed
  if (/^https:\/\//i.test(trimmed)) return sanitizeUrl(trimmed)
  if (/^data:image\/(png|jpe?g|webp|gif);base64,/i.test(trimmed)) {
    return trimmed
  }
  return ''
}

function normalizeSupportContactMethod(contact: SupportContactMethod, index: number): SupportContactMethod | null {
  if (!contact) return null
  const type = (contact.type || 'custom').trim().toLowerCase()
  const normalized: SupportContactMethod = {
    id: contact.id?.trim() || `${type}-${index + 1}`,
    type,
    label: contact.label?.trim() || fallbackContactLabel(type),
    value: contact.value?.trim() || '',
    copy_value: contact.copy_value?.trim() || '',
    url: contact.url?.trim() || '',
    qr_image: contact.qr_image?.trim() || '',
    description: contact.description?.trim() || '',
    primary: !!contact.primary,
    enabled: contact.enabled !== false,
    sort_order: Number.isFinite(contact.sort_order) && contact.sort_order > 0 ? contact.sort_order : index + 1,
  }
  if (!normalized.value && !normalized.copy_value && !normalized.url && !normalized.qr_image) {
    return null
  }
  return normalized
}

function fallbackContactLabel(type: string): string {
  switch (type) {
    case 'wechat':
      return '微信客服'
    case 'qq':
      return 'QQ 客服'
    case 'telegram':
      return 'Telegram'
    case 'email':
      return '邮箱'
    case 'docs':
      return '文档'
    default:
      return '客服'
  }
}
