import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const {
  getStatus,
  generateAuthURL,
  bindOAuth,
  discoverCapabilities
} = vi.hoisted(() => ({
  getStatus: vi.fn(),
  generateAuthURL: vi.fn(),
  bindOAuth: vi.fn(),
  discoverCapabilities: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      getOpenAIExcelOAuthStatus: getStatus,
      generateOpenAIExcelOAuthAuthURL: generateAuthURL,
      bindOpenAIExcelOAuth: bindOAuth,
      discoverOpenAIExcelOAuthCapabilities: discoverCapabilities
    }
  }
}))

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => {
  if (params?.count !== undefined) return `${key}:${String(params.count)}`
  return key
} }) }))

import ExcelOAuthBinding from '../ExcelOAuthBinding.vue'

const connectedStatus = { account_id: 42, authorized: true, revision: 3, updated_at: '2026-11-01T10:00:00Z' }

describe('ExcelOAuthBinding', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.spyOn(window, 'open').mockReturnValue(null)
    getStatus.mockResolvedValue({ account_id: 42, authorized: false, revision: 0 })
    generateAuthURL.mockResolvedValue({
      auth_url: 'https://auth.example.test/authorize?state=bps.expected.PC&redirect_uri=http%3A%2F%2Flocalhost%2Fcallback',
      session_id: 'session-1'
    })
    bindOAuth.mockResolvedValue({ ...connectedStatus, authorized: true, revision: 1 })
    discoverCapabilities.mockResolvedValue({
      allowed: true,
      models: [{ id: 'gpt-5.6', display_name: 'GPT 5.6', reasoning_efforts: ['high'] }]
    })
  })

  afterEach(() => vi.restoreAllMocks())

  it('loads a safe status and renders discovered allowlist models without credentials', async () => {
    getStatus.mockResolvedValue(connectedStatus)
    const wrapper = mount(ExcelOAuthBinding, { props: { accountId: 42 } })
    await flushPromises()

    expect(getStatus).toHaveBeenCalledWith(42)
    expect(wrapper.get('[data-testid="excel-oauth-status"]').text()).toBe('admin.accounts.openai.excelOAuth.connected')
    await wrapper.get('[data-testid="excel-oauth-discover"]').trigger('click')
    await flushPromises()

    expect(discoverCapabilities).toHaveBeenCalledWith(42)
    expect(wrapper.get('[data-testid="excel-oauth-capabilities"]').text()).toContain('GPT 5.6')
    expect(wrapper.text()).not.toContain('access_token')
    expect(wrapper.text()).not.toContain('refresh_token')
  })

  it('binds a full callback URL to the active session and revision', async () => {
    const wrapper = mount(ExcelOAuthBinding, { props: { accountId: 42 } })
    await flushPromises()
    await wrapper.get('[data-testid="excel-oauth-start"]').trigger('click')
    await flushPromises()

    expect(generateAuthURL).toHaveBeenCalledWith(42)
    expect(wrapper.get('[data-testid="excel-oauth-open-link"]').attributes('href')).toContain('state=bps.expected.PC')
    await wrapper.get('[data-testid="excel-oauth-callback-input"]').setValue(
      'http://localhost:1455/auth/callback?code=callback-code&state=bps.expected.PC'
    )
    await wrapper.get('[data-testid="excel-oauth-bind"]').trigger('click')
    await flushPromises()

    expect(bindOAuth).toHaveBeenCalledWith(42, {
      session_id: 'session-1',
      code: 'callback-code',
      state: 'bps.expected.PC',
      redirect_uri: 'http://localhost/callback',
      expected_revision: 0
    })
    expect(wrapper.find('[data-testid="excel-oauth-callback-input"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="excel-oauth-status"]').text()).toBe('admin.accounts.openai.excelOAuth.connected')
  })

  it('accepts a raw code while binding the generated state', async () => {
    const wrapper = mount(ExcelOAuthBinding, { props: { accountId: 42 } })
    await flushPromises()
    await wrapper.get('[data-testid="excel-oauth-start"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="excel-oauth-callback-input"]').setValue('raw-code')
    await wrapper.get('[data-testid="excel-oauth-bind"]').trigger('click')
    await flushPromises()

    expect(bindOAuth).toHaveBeenCalledWith(42, expect.objectContaining({
      code: 'raw-code',
      state: 'bps.expected.PC'
    }))
  })

  it('rejects a callback with a mismatched state before sending it', async () => {
    const wrapper = mount(ExcelOAuthBinding, { props: { accountId: 42 } })
    await flushPromises()
    await wrapper.get('[data-testid="excel-oauth-start"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="excel-oauth-callback-input"]').setValue(
      'http://localhost/callback?code=code&state=wrong-state'
    )
    await wrapper.get('[data-testid="excel-oauth-bind"]').trigger('click')
    await flushPromises()

    expect(bindOAuth).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="excel-oauth-error"]').text()).toBe('admin.accounts.openai.excelOAuth.stateMismatch')
  })

  it('refreshes status after a revision conflict and requires a new authorization session', async () => {
    bindOAuth.mockRejectedValue({ status: 409, code: 'OPENAI_EXCEL_OAUTH_REVISION_CONFLICT' })
    const wrapper = mount(ExcelOAuthBinding, { props: { accountId: 42 } })
    await flushPromises()
    await wrapper.get('[data-testid="excel-oauth-start"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="excel-oauth-callback-input"]').setValue('raw-code')
    await wrapper.get('[data-testid="excel-oauth-bind"]').trigger('click')
    await flushPromises()

    expect(getStatus).toHaveBeenCalledTimes(2)
    expect(wrapper.get('[data-testid="excel-oauth-error"]').text()).toBe('admin.accounts.openai.excelOAuth.revisionConflict')
    expect(wrapper.find('[data-testid="excel-oauth-callback-input"]').exists()).toBe(false)
  })

  it('shows safe API errors without exposing values from the response', async () => {
    getStatus.mockRejectedValue({ status: 500, message: 'Bearer secret-token' })
    const wrapper = mount(ExcelOAuthBinding, { props: { accountId: 42 } })
    await flushPromises()

    expect(wrapper.get('[data-testid="excel-oauth-error"]').text()).toBe('admin.accounts.openai.excelOAuth.statusFailed')
    expect(wrapper.text()).not.toContain('secret-token')
  })
})
