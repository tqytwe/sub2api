import { describe, expect, it } from 'vitest'
import {
  OPENAI_CC_SWITCH_CODEX_MODEL,
  buildCcSwitchImportDeeplink,
  buildCcSwitchUsageScript,
  isCcSwitchSupportedPlatform,
  normalizeGatewayRootUrl,
  resolveCcSwitchImportConfig
} from '@/utils/ccswitchImport'
import type { GroupPlatform } from '@/types'

function paramsFromDeeplink(deeplink: string): URLSearchParams {
  const query = deeplink.split('?')[1] || ''
  return new URLSearchParams(query)
}

describe('ccswitchImport utils', () => {
  it('defaults OpenAI CC Switch imports to the current Codex model', () => {
    expect(OPENAI_CC_SWITCH_CODEX_MODEL).toBe('gpt-5.5')
  })

  it.each([
    ['https://api.example.com', 'https://api.example.com'],
    ['https://api.example.com/', 'https://api.example.com'],
    ['https://api.example.com/v1', 'https://api.example.com'],
    ['https://api.example.com/v1/', 'https://api.example.com']
  ])('normalizeGatewayRootUrl(%s) -> %s', (input, expected) => {
    expect(normalizeGatewayRootUrl(input)).toBe(expected)
  })

  it.each([
    { platform: 'anthropic' as GroupPlatform, ok: true },
    { platform: 'openai' as GroupPlatform, ok: true },
    { platform: 'gemini' as GroupPlatform, ok: true },
    { platform: 'antigravity' as GroupPlatform, ok: true },
    { platform: 'grok' as GroupPlatform, ok: false }
  ])('isCcSwitchSupportedPlatform($platform) -> $ok', ({ platform, ok }) => {
    expect(isCcSwitchSupportedPlatform(platform)).toBe(ok)
  })

  it.each([
    {
      platform: 'anthropic' as GroupPlatform,
      clientType: 'claude' as const,
      endpoint: 'https://api.example.com',
      app: 'claude'
    },
    {
      platform: 'openai' as GroupPlatform,
      clientType: 'claude' as const,
      endpoint: 'https://api.example.com/v1',
      app: 'codex'
    },
    {
      platform: 'gemini' as GroupPlatform,
      clientType: 'gemini' as const,
      endpoint: 'https://api.example.com',
      app: 'gemini'
    },
    {
      platform: 'antigravity' as GroupPlatform,
      clientType: 'claude' as const,
      endpoint: 'https://api.example.com/antigravity',
      app: 'claude'
    },
    {
      platform: 'antigravity' as GroupPlatform,
      clientType: 'gemini' as const,
      endpoint: 'https://api.example.com/antigravity',
      app: 'gemini'
    }
  ])('$platform/$clientType endpoint is $endpoint', ({ platform, clientType, endpoint, app }) => {
    const config = resolveCcSwitchImportConfig(platform, clientType, 'https://api.example.com/v1')
    expect(config.app).toBe(app)
    expect(config.endpoint).toBe(endpoint)
  })

  it('strips trailing /v1 from claude endpoint but keeps homepage as root', () => {
    const params = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        baseUrl: 'https://api.example.com/v1',
        providerName: 'Sub2API',
        apiKey: 'sk-test',
        usageScript: 'return true',
        platform: 'anthropic',
        clientType: 'claude'
      })
    )

    expect(params.get('endpoint')).toBe('https://api.example.com')
    expect(params.get('homepage')).toBe('https://api.example.com')
  })

  it('usage script strips /v1 before appending /v1/usage', () => {
    const script = buildCcSwitchUsageScript()
    expect(script).toContain('/v1/usage')
    expect(script).toContain('replace(/\\/v1$/i,"")')
  })

  const baseInput = {
    baseUrl: 'https://api.example.com',
    providerName: 'Sub2API',
    apiKey: 'sk-test',
    usageScript: 'return true'
  }

  it('adds the Codex model parameter and /v1 endpoint for OpenAI imports', () => {
    const params = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        ...baseInput,
        platform: 'openai',
        clientType: 'claude'
      })
    )

    expect(params.get('resource')).toBe('provider')
    expect(params.get('app')).toBe('codex')
    expect(params.get('endpoint')).toBe(`${baseInput.baseUrl}/v1`)
    expect(params.get('homepage')).toBe(baseInput.baseUrl)
    expect(params.get('model')).toBe(OPENAI_CC_SWITCH_CODEX_MODEL)
    expect(atob(params.get('usageScript') || '')).toBe(baseInput.usageScript)
  })

  it.each([
    { platform: 'anthropic' as GroupPlatform, clientType: 'claude' as const, app: 'claude' },
    { platform: 'gemini' as GroupPlatform, clientType: 'gemini' as const, app: 'gemini' }
  ])('does not add a model parameter for $platform imports', ({ platform, clientType, app }) => {
    const params = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        ...baseInput,
        platform,
        clientType
      })
    )

    expect(params.get('app')).toBe(app)
    expect(params.get('endpoint')).toBe(baseInput.baseUrl)
    expect(params.has('model')).toBe(false)
  })

  it('keeps Antigravity imports on the selected client endpoint without a model parameter', () => {
    const params = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        ...baseInput,
        platform: 'antigravity',
        clientType: 'gemini'
      })
    )

    expect(params.get('app')).toBe('gemini')
    expect(params.get('endpoint')).toBe(`${baseInput.baseUrl}/antigravity`)
    expect(params.has('model')).toBe(false)
  })
})
