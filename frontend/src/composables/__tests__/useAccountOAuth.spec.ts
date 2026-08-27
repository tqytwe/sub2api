import { beforeEach, describe, expect, it, vi } from 'vitest'

let locale: 'zh' | 'en' = 'zh'
const showError = vi.fn()

const messages = {
  zh: {
    'admin.accounts.oauth.failedToGenerateAuthUrl': '生成授权链接失败',
    'admin.accounts.oauth.missingAuthCodeOrSession': '缺少授权码或会话标识',
    'admin.accounts.oauth.failedToExchangeAuthCode': '兑换授权码失败',
    'admin.accounts.oauth.pleaseEnterSessionKey': '请输入至少一个有效的 sessionKey',
    'admin.accounts.oauth.cookieAuthFailed': 'Cookie 授权失败'
  },
  en: {
    'admin.accounts.oauth.failedToGenerateAuthUrl': 'Failed to generate authorization URL',
    'admin.accounts.oauth.missingAuthCodeOrSession': 'Missing authorization code or session ID',
    'admin.accounts.oauth.failedToExchangeAuthCode': 'Failed to exchange authorization code',
    'admin.accounts.oauth.pleaseEnterSessionKey': 'Please enter at least one valid sessionKey',
    'admin.accounts.oauth.cookieAuthFailed': 'Cookie authorization failed'
  }
} as const

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: keyof typeof messages.zh) => messages[locale][key] ?? key
  })
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      generateAuthUrl: vi.fn(),
      exchangeCode: vi.fn()
    }
  }
}))

import { adminAPI } from '@/api/admin'
import { useAccountOAuth } from '@/composables/useAccountOAuth'

describe('useAccountOAuth localized fallbacks', () => {
  beforeEach(() => {
    locale = 'zh'
    showError.mockReset()
    vi.mocked(adminAPI.accounts.generateAuthUrl).mockReset()
    vi.mocked(adminAPI.accounts.exchangeCode).mockReset()
  })

  it('uses Chinese fallback text when generating the authorization URL fails', async () => {
    vi.mocked(adminAPI.accounts.generateAuthUrl).mockRejectedValueOnce(new Error('network'))
    const oauth = useAccountOAuth()

    await expect(oauth.generateAuthUrl('oauth')).resolves.toBe(false)

    expect(oauth.error.value).toBe('生成授权链接失败')
    expect(showError).toHaveBeenCalledWith('生成授权链接失败')
  })

  it('uses the active English locale for code validation and exchange failures', async () => {
    locale = 'en'
    const oauth = useAccountOAuth()

    await expect(oauth.exchangeAuthCode(' ', 'session-id')).resolves.toBeNull()
    expect(oauth.error.value).toBe('Missing authorization code or session ID')

    oauth.authCode.value = 'code'
    oauth.sessionId.value = 'session-id'
    vi.mocked(adminAPI.accounts.exchangeCode).mockRejectedValueOnce(new Error('network'))
    await expect(oauth.exchangeAuthCode('oauth')).resolves.toBeNull()
    expect(oauth.error.value).toBe('Failed to exchange authorization code')
    expect(showError).toHaveBeenCalledWith('Failed to exchange authorization code')
  })

  it('uses localized session-key and cookie-authorization fallbacks', async () => {
    const oauth = useAccountOAuth()

    await expect(oauth.cookieAuth('oauth', ' ')).resolves.toBeNull()
    expect(oauth.error.value).toBe('请输入至少一个有效的 sessionKey')

    vi.mocked(adminAPI.accounts.exchangeCode).mockRejectedValueOnce(new Error('network'))
    await expect(oauth.cookieAuth('oauth', 'session-key')).resolves.toBeNull()
    expect(oauth.error.value).toBe('Cookie 授权失败')
  })
})
