import type { PromptVariableDefinition } from '@/api/prompts'

const PROMPT_RECIPE_STORAGE_KEY = 'jisudeng-prompt-recipes:v1'
const MAX_PROMPT_RECIPES = 50

export interface PromptCreationRecipe {
  id: string
  prompt_id: string
  prompt_version: number
  title: string
  model: string
  size: string
  quality: string
  variables: PromptVariableDefinition[]
  created_at: string
}

export interface SavePromptRecipeInput {
  promptId: string
  promptVersion: number
  title: string
  model: string
  size: string
  quality: string
  variables: PromptVariableDefinition[]
  variableValues?: Record<string, string>
  finalPrompt?: string
}

function isPromptCreationRecipe(value: unknown): value is PromptCreationRecipe {
  if (!value || typeof value !== 'object') return false
  const recipe = value as Partial<PromptCreationRecipe>
  return (
    typeof recipe.id === 'string'
    && typeof recipe.prompt_id === 'string'
    && Number.isInteger(recipe.prompt_version)
    && Number(recipe.prompt_version) > 0
    && typeof recipe.title === 'string'
    && typeof recipe.model === 'string'
    && typeof recipe.size === 'string'
    && typeof recipe.quality === 'string'
    && Array.isArray(recipe.variables)
    && typeof recipe.created_at === 'string'
  )
}

function publicVariableStructure(variable: PromptVariableDefinition): PromptVariableDefinition {
  const structure: PromptVariableDefinition = {
    name: variable.name,
    label: variable.label,
  }
  if (variable.description) structure.description = variable.description
  if (variable.type) structure.type = variable.type
  if (variable.required !== undefined) structure.required = variable.required
  if (variable.options) structure.options = variable.options
  return structure
}

export function listPromptRecipes(): PromptCreationRecipe[] {
  try {
    const raw = localStorage.getItem(PROMPT_RECIPE_STORAGE_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    return parsed.filter(isPromptCreationRecipe)
  } catch {
    return []
  }
}

export function savePromptRecipe(input: SavePromptRecipeInput): PromptCreationRecipe {
  const recipe: PromptCreationRecipe = {
    id: `${input.promptId}:${input.promptVersion}:${Date.now()}`,
    prompt_id: input.promptId,
    prompt_version: input.promptVersion,
    title: input.title.trim() || '未命名创作配方',
    model: input.model,
    size: input.size,
    quality: input.quality,
    variables: input.variables.map(publicVariableStructure),
    created_at: new Date().toISOString(),
  }
  const recipes = [
    recipe,
    ...listPromptRecipes().filter((item) =>
      item.prompt_id !== recipe.prompt_id
      || item.prompt_version !== recipe.prompt_version
      || item.model !== recipe.model
      || item.size !== recipe.size
      || item.quality !== recipe.quality),
  ].slice(0, MAX_PROMPT_RECIPES)
  try {
    localStorage.setItem(PROMPT_RECIPE_STORAGE_KEY, JSON.stringify(recipes))
  } catch {
    // 配方保存失败不应阻断当前创作。
  }
  return recipe
}
