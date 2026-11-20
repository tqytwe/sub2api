export const QWEN_DEFAULT_BASE_URL = 'https://dashscope.aliyuncs.com/compatible-mode/v1'

export function applyQwenCacheSettings(credentials: Record<string, unknown>, sessionCache: boolean, strictAffinity: boolean): void {
  for (const [key, enabled] of [
    ['qwen_session_cache_enabled', sessionCache],
    ['qwen_strict_session_affinity', strictAffinity]
  ] as const) {
    if (enabled) credentials[key] = true
    else delete credentials[key]
  }
}
