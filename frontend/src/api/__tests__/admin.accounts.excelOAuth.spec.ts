import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post } = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))
vi.mock('../client', () => ({ apiClient: { get, post } }))

import {
  bindOpenAIExcelOAuth,
  discoverOpenAIExcelOAuthCapabilities,
  generateOpenAIExcelOAuthAuthURL,
  getOpenAIExcelOAuthStatus
} from '../admin/accounts'

describe('account-scoped Excel OAuth API', () => {
  beforeEach(() => vi.clearAllMocks())

  it('uses the account-scoped status endpoint', async () => {
    const status = { account_id: 42, authorized: false, revision: 0 }
    get.mockResolvedValue({ data: status })
    await expect(getOpenAIExcelOAuthStatus(42)).resolves.toEqual(status)
    expect(get).toHaveBeenCalledWith('/admin/openai/accounts/42/excel-oauth')
  })

  it('starts authorization for the existing account', async () => {
    const auth = { auth_url: 'https://auth.example.test', session_id: 'session-1' }
    post.mockResolvedValue({ data: auth })
    await expect(generateOpenAIExcelOAuthAuthURL(42)).resolves.toEqual(auth)
    expect(post).toHaveBeenCalledWith('/admin/openai/accounts/42/excel-oauth/auth-url')
  })

  it('binds only the callback material and expected revision', async () => {
    const input = {
      session_id: 'session-1',
      code: 'authorization-code',
      state: 'bps.state.PC',
      redirect_uri: 'http://localhost/callback',
      expected_revision: 3
    }
    const status = { account_id: 42, authorized: true, revision: 4 }
    post.mockResolvedValue({ data: status })
    await expect(bindOpenAIExcelOAuth(42, input)).resolves.toEqual(status)
    expect(post).toHaveBeenCalledWith('/admin/openai/accounts/42/excel-oauth/bind', input)
  })

  it('reads only the safe capability projection', async () => {
    const capabilities = { allowed: true, models: [{ id: 'gpt-5.6', reasoning_efforts: ['high'] }] }
    get.mockResolvedValue({ data: capabilities })
    await expect(discoverOpenAIExcelOAuthCapabilities(42)).resolves.toEqual(capabilities)
    expect(get).toHaveBeenCalledWith('/admin/openai/accounts/42/excel-oauth/capabilities')
  })
})
