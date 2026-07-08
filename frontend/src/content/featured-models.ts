/** Static preview rows for guests on /models (full list requires auth). */
export interface FeaturedModelRow {
  name: string
  platform: string
  useCaseKey: string
}

/** Curated guest preview — align with pricing JSON & 2026-07 mainstream lineup. */
export const FEATURED_PUBLIC_MODELS: FeaturedModelRow[] = [
  { name: 'gpt-5.6-sol', platform: 'OpenAI', useCaseKey: 'models.preview.useCases.reasoning' },
  { name: 'gpt-5.6-terra', platform: 'OpenAI', useCaseKey: 'models.preview.useCases.code' },
  { name: 'gpt-5.5', platform: 'OpenAI', useCaseKey: 'models.preview.useCases.code' },
  { name: 'gpt-5-mini', platform: 'OpenAI', useCaseKey: 'models.preview.useCases.chat' },
  { name: 'gpt-4.1', platform: 'OpenAI', useCaseKey: 'models.preview.useCases.code' },
  { name: 'o4-mini', platform: 'OpenAI', useCaseKey: 'models.preview.useCases.reasoning' },
  { name: 'claude-sonnet-4-6', platform: 'Anthropic', useCaseKey: 'models.preview.useCases.code' },
  { name: 'claude-opus-4-8', platform: 'Anthropic', useCaseKey: 'models.preview.useCases.reasoning' },
  { name: 'claude-haiku-4-5', platform: 'Anthropic', useCaseKey: 'models.preview.useCases.chat' },
  { name: 'gemini-2.5-flash', platform: 'Google', useCaseKey: 'models.preview.useCases.chat' },
  { name: 'gemini-3-flash', platform: 'Google', useCaseKey: 'models.preview.useCases.reasoning' },
]
