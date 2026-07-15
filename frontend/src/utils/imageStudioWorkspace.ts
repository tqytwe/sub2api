import type {
  ImageStudioCatalog,
  ImageStudioIntent,
  ImageStudioModelOption,
  ImageStudioTemplate,
} from '@/api/imageStudio'

export interface ImageStudioTemplateSelection {
  intent: ImageStudioIntent
  template: ImageStudioTemplate
}

export interface ImageStudioKeyOption {
  id: number
  name: string
}

export interface ImageStudioKeySelection {
  key: ImageStudioKeyOption
  models: ImageStudioModelOption[]
}

export async function findFirstImageStudioKeyWithModels(
  keys: ImageStudioKeyOption[],
  loadModels: (keyId: number) => Promise<ImageStudioModelOption[]>,
): Promise<ImageStudioKeySelection | null> {
  for (const key of keys) {
    try {
      const models = await loadModels(key.id)
      if (models.length > 0) return { key, models }
    } catch {
      // A key may belong to a group where image generation is unavailable.
    }
  }
  return null
}

export function flattenImageStudioTemplates(catalog: ImageStudioCatalog | null): ImageStudioTemplateSelection[] {
  return (catalog?.intents ?? []).flatMap((intent) =>
    intent.templates.map((template) => ({ intent, template })),
  )
}

export function resolveInitialImageStudioTemplate(
  catalog: ImageStudioCatalog | null,
  preferredTemplateId?: string | null,
): ImageStudioTemplateSelection | null {
  const options = flattenImageStudioTemplates(catalog)
  if (!options.length) return null
  return options.find((option) => option.template.id === preferredTemplateId) ?? options[0]
}

export function isImageStudioPromptValid(prompt: string): boolean {
  return prompt.trim().length > 0
}
