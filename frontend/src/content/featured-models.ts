/** Static preview rows for guests on /models (full list requires auth). */
export interface FeaturedModelRow {
  name: string
  platform: string
  useCaseKey: string
}

export const FEATURED_PUBLIC_MODELS: FeaturedModelRow[] = [
  { name: 'gpt-4o-mini', platform: 'OpenAI', useCaseKey: 'models.preview.useCases.chat' },
  { name: 'gpt-4o', platform: 'OpenAI', useCaseKey: 'models.preview.useCases.reasoning' },
  { name: 'claude-3-5-haiku-latest', platform: 'Anthropic', useCaseKey: 'models.preview.useCases.chat' },
  { name: 'claude-sonnet-4-20250514', platform: 'Anthropic', useCaseKey: 'models.preview.useCases.code' },
  { name: 'claude-opus-4-20250514', platform: 'Anthropic', useCaseKey: 'models.preview.useCases.reasoning' },
  { name: 'gemini-2.0-flash', platform: 'Google', useCaseKey: 'models.preview.useCases.chat' },
  { name: 'o3-mini', platform: 'OpenAI', useCaseKey: 'models.preview.useCases.reasoning' },
  { name: 'gpt-image-1', platform: 'OpenAI', useCaseKey: 'models.preview.useCases.image' },
]
