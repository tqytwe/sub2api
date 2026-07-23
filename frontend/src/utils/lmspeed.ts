export type LmspeedLocale = 'en' | 'zh' | 'ru'

export interface LmspeedSpeedTestUrlInput {
  baseUrl: string
  apiKey: string
  modelId?: string
  locale?: LmspeedLocale
}

const LMSPEED_BASE_URL_BY_LOCALE: Record<LmspeedLocale, string> = {
  en: 'https://lmspeed.net',
  zh: 'https://lmspeed.net/zh',
  ru: 'https://lmspeed.net/ru',
}

export function normalizeLmspeedBaseUrl(baseUrl: string): string {
  const trimmed = baseUrl.trim().replace(/\/+$/, '')
  if (!trimmed) return ''
  return `${trimmed.replace(/\/v1$/i, '')}/v1`
}

export function resolveLmspeedLocale(appLocale: string | null | undefined): LmspeedLocale {
  const normalized = appLocale?.trim().toLowerCase() ?? ''
  if (normalized.startsWith('zh')) return 'zh'
  if (normalized.startsWith('ru')) return 'ru'
  return 'en'
}

export function buildLmspeedSpeedTestUrl(input: LmspeedSpeedTestUrlInput): string {
  const url = new URL(LMSPEED_BASE_URL_BY_LOCALE[input.locale ?? 'en'])
  url.searchParams.set('baseUrl', normalizeLmspeedBaseUrl(input.baseUrl))
  url.searchParams.set('apiKey', input.apiKey.trim())

  const modelId = input.modelId?.trim()
  if (modelId) {
    url.searchParams.set('modelId', modelId)
  }

  return url.toString()
}
