import { describe, expect, it } from 'vitest'
import { buildCcSwitchImportDeeplink } from '../ccswitchImport'

describe('Antigravity CC Switch endpoint', () => {
  it.each([
    // FORK: homepage is the normalized gateway root (normalizeGatewayRootUrl), not the raw input.
    ['https://api.example.com', 'https://api.example.com/antigravity', 'https://api.example.com'],
    ['https://api.example.com/', 'https://api.example.com/antigravity', 'https://api.example.com'],
    ['https://api.example.com///', 'https://api.example.com/antigravity', 'https://api.example.com'],
    ['https://api.example.com/sub2api/', 'https://api.example.com/sub2api/antigravity', 'https://api.example.com/sub2api']
  ])('joins the platform path onto %s for both clients', (baseUrl, endpoint, homepage) => {
    for (const clientType of ['claude', 'gemini'] as const) {
      const url = new URL(buildCcSwitchImportDeeplink({
        baseUrl, clientType, platform: 'antigravity', providerName: 'Sub2API',
        apiKey: 'sk-test', usageScript: 'return true'
      }))
      expect(url.searchParams.get('endpoint')).toBe(endpoint)
      expect(url.searchParams.get('app')).toBe(clientType)
      expect(url.searchParams.get('homepage')).toBe(homepage)
      expect(url.searchParams.has('model')).toBe(false)
    }
  })
})
