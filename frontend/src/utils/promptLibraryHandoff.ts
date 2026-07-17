import type { LocationQuery } from 'vue-router'
import type {
  PromptReferenceRequirement,
  PromptVariableDefinition,
} from '@/api/prompts'

export interface PromptLibraryHandoff {
  prompt_id: string
  version: number
  title: string
  prompt_template: string
  variables: PromptVariableDefinition[]
  recommended_models: string[]
  recommended_sizes: string[]
  reference_requirement: PromptReferenceRequirement
}

function firstQueryValue(value: LocationQuery[string] | unknown): string {
  if (Array.isArray(value)) return String(value[0] ?? '')
  return typeof value === 'string' ? value : ''
}

function positiveInteger(value: unknown): number | null {
  const parsed = Number.parseInt(firstQueryValue(value), 10)
  return Number.isInteger(parsed) && parsed > 0 ? parsed : null
}

function stringArray(value: unknown): string[] {
  if (!Array.isArray(value)) return []
  return value
    .filter((item): item is string => typeof item === 'string')
    .map((item) => item.trim())
    .filter(Boolean)
}

function variables(value: unknown): PromptVariableDefinition[] {
  if (!Array.isArray(value)) return []
  return value.filter((item): item is PromptVariableDefinition => {
    if (!item || typeof item !== 'object') return false
    const variable = item as Partial<PromptVariableDefinition>
    return typeof variable.name === 'string'
      && variable.name.trim() !== ''
      && typeof variable.label === 'string'
      && variable.label.trim() !== ''
  })
}

export function loadPromptLibraryHandoff(
  query: LocationQuery | Record<string, unknown>,
): PromptLibraryHandoff | null {
  const promptID = positiveInteger(query.prompt)
  const version = positiveInteger(query.version)
  if (!promptID || !version) return null

  try {
    const raw = sessionStorage.getItem(`prompt-library:${promptID}:${version}`)
    if (!raw) return null
    sessionStorage.removeItem(`prompt-library:${promptID}:${version}`)
    const parsed = JSON.parse(raw) as Record<string, unknown>
    if (
      String(parsed.prompt_id) !== String(promptID)
      || parsed.version !== version
      || typeof parsed.prompt_template !== 'string'
      || !parsed.prompt_template.trim()
    ) {
      return null
    }
    const requirement = parsed.reference_requirement
    return {
      prompt_id: String(promptID),
      version,
      title: typeof parsed.title === 'string' && parsed.title.trim()
        ? parsed.title.trim()
        : `提示词 #${promptID}`,
      prompt_template: parsed.prompt_template,
      variables: variables(parsed.variables),
      recommended_models: stringArray(parsed.recommended_models),
      recommended_sizes: stringArray(parsed.recommended_sizes),
      reference_requirement: requirement === 'required' || requirement === 'optional'
        ? requirement
        : 'none',
    }
  } catch {
    return null
  }
}

export function initialPromptVariableValues(
  definitions: PromptVariableDefinition[],
): Record<string, string> {
  return Object.fromEntries(definitions.map((variable) => [
    variable.name,
    variable.default_value === null || variable.default_value === undefined
      ? ''
      : String(variable.default_value),
  ]))
}

function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

export function renderPromptLibraryTemplate(
  template: string,
  definitions: PromptVariableDefinition[],
  values: Record<string, string>,
): string {
  let result = template
  for (const variable of definitions) {
    const value = values[variable.name] ?? ''
    const name = escapeRegExp(variable.name)
    const patterns = [
      new RegExp(`\\{\\{\\s*${name}\\s*\\}\\}`, 'g'),
      new RegExp(`\\{\\s*${name}\\s*\\}`, 'g'),
      new RegExp(`\\[\\s*${name}\\s*\\]`, 'g'),
    ]
    for (const pattern of patterns) result = result.replace(pattern, value)
  }
  return result
}
