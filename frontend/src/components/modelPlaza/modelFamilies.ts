import type { PlazaModel } from '@/api/modelPlaza'

export const MODEL_FAMILY_KEYS = ['deepseek', 'qwen', 'kimi', 'glm'] as const

export type ModelFamilyKey = (typeof MODEL_FAMILY_KEYS)[number]

const modelFamilyAliases: Record<ModelFamilyKey, readonly string[]> = {
  deepseek: ['deepseek'],
  qwen: ['qwen', 'qwq', 'qvq', 'tongyi', 'dashscope'],
  kimi: ['kimi', 'moonshot'],
  glm: ['glm', 'zhipu', 'z.ai', 'z-ai', 'bigmodel'],
}

export function parseModelFamily(value: unknown): ModelFamilyKey | null {
  const normalized = String(value ?? '').trim().toLowerCase()
  return MODEL_FAMILY_KEYS.includes(normalized as ModelFamilyKey)
    ? normalized as ModelFamilyKey
    : null
}

export function modelMatchesFamily(model: Pick<PlazaModel, 'name' | 'platform'>, family: ModelFamilyKey): boolean {
  const haystack = `${model.name} ${model.platform}`.toLowerCase()
  return modelFamilyAliases[family].some((alias) => haystack.includes(alias))
}
