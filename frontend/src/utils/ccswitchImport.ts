import type { GroupPlatform } from '@/types'

export const OPENAI_CC_SWITCH_CODEX_MODEL = 'gpt-5.5'

export type CcSwitchClientType = 'claude' | 'gemini'

export interface CcSwitchImportConfig {
  app: string
  endpoint: string
  model?: string
}

export interface CcSwitchImportDeeplinkInput {
  baseUrl: string
  platform?: GroupPlatform | null
  clientType: CcSwitchClientType
  providerName: string
  apiKey: string
  usageScript: string
}

/** 网关根域名：去尾斜杠、去误加的 /v1（各协议再各自拼路径） */
export function normalizeGatewayRootUrl(url: string): string {
  return url.trim().replace(/\/+$/, '').replace(/\/v1$/i, '')
}

/**
 * 余额统一打 {root}/v1/usage（Antigravity 为 {root}/antigravity/v1/usage）。
 * 即使用户/导入把 Base URL 写成 .../v1，也不会变成 /v1/v1/usage。
 */
export function buildCcSwitchUsageScript(): string {
  return `({
    request: {
      url: (function(){var b=("{{baseUrl}}"||"").replace(/\\/+$/,"").replace(/\\/v1$/i,"");return b+"/v1/usage";})(),
      method: "GET",
      headers: { "Authorization": "Bearer {{apiKey}}" }
    },
    extractor: function(response) {
      const remaining = response?.remaining ?? response?.quota?.remaining ?? response?.balance;
      const unit = response?.unit ?? response?.quota?.unit ?? "USD";
      return {
        isValid: response?.is_active ?? response?.isValid ?? true,
        remaining,
        unit
      };
    }
  })`
}

/** Grok 走独立 CLI，CC Switch 无对应 app，不应走 deeplink 导入 */
export function isCcSwitchSupportedPlatform(platform: GroupPlatform | undefined | null): boolean {
  switch (platform || 'anthropic') {
    case 'anthropic':
    case 'openai':
    case 'gemini':
    case 'antigravity':
      return true
    default:
      return false
  }
}

/**
 * 按分组平台生成 CC Switch endpoint。
 *
 * 路径约定（与客户端拼接规则一致）：
 * - Claude Code：根域名，客户端拼 /v1/messages
 * - Codex：根+/v1，直接写入 config.toml base_url
 * - Gemini CLI：根域名，客户端拼 Gemini 路径
 * - Antigravity：根+/antigravity，再由客户端按 Claude/Gemini 拼协议路径
 */
export function resolveCcSwitchImportConfig(
  platform: GroupPlatform | undefined | null,
  clientType: CcSwitchClientType,
  baseUrl: string
): CcSwitchImportConfig {
  const root = normalizeGatewayRootUrl(baseUrl)
  switch (platform || 'anthropic') {
    case 'antigravity':
      return {
        app: clientType === 'gemini' ? 'gemini' : 'claude',
        endpoint: `${root}/antigravity`
      }
    case 'openai':
      return {
        app: 'codex',
        endpoint: `${root}/v1`,
        model: OPENAI_CC_SWITCH_CODEX_MODEL
      }
    case 'gemini':
      return {
        app: 'gemini',
        endpoint: root
      }
    default:
      return {
        app: 'claude',
        endpoint: root
      }
  }
}

export function buildCcSwitchImportDeeplink(input: CcSwitchImportDeeplinkInput): string {
  const root = normalizeGatewayRootUrl(input.baseUrl)
  const config = resolveCcSwitchImportConfig(input.platform, input.clientType, root)
  const entries: [string, string][] = [
    ['resource', 'provider'],
    ['app', config.app],
    ['name', input.providerName],
    ['homepage', root],
    ['endpoint', config.endpoint],
    ['apiKey', input.apiKey],
    ['configFormat', 'json'],
    ['usageEnabled', 'true'],
    ['usageScript', btoa(input.usageScript)],
    ['usageAutoInterval', '30']
  ]

  if (config.model) {
    entries.splice(2, 0, ['model', config.model])
  }

  return `ccswitch://v1/import?${new URLSearchParams(entries).toString()}`
}
