import type { ImageStudioCatalog, ImageStudioIntent, ImageStudioTemplate } from '@/api/imageStudio'

export interface ImageStudioTemplateSelection {
  intent: ImageStudioIntent
  template: ImageStudioTemplate
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
