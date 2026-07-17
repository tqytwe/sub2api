import { describe, expect, it } from 'vitest'
import {
  createDefaultAdminPromptDraft,
  promptSourceLabel,
  promptSessionStorageKey,
  readPromptFilters,
  storePromptUsePayload,
  toPromptQuery,
} from '@/utils/promptLibrary'

describe('promptLibrary utilities', () => {
  it('uses the public 极速蹬 brand labels for every source class', () => {
    expect(promptSourceLabel('original')).toBe('极速蹬原创')
    expect(promptSourceLabel('authorized')).toBe('极速蹬授权')
    expect(promptSourceLabel('curated')).toBe('极速蹬精选')
    expect(promptSourceLabel('community')).toBe('极速蹬社区精选')
  })

  it('reads every supported filter from the URL and writes it back without empty values', () => {
    const filters = readPromptFilters({
      q: '海报',
      purpose: 'marketing',
      style: 'minimal',
      subject: 'product',
      model: 'gpt-image-1',
      size: '1024x1536',
      reference: 'required',
      featured: 'true',
      sort: 'popular',
      page: '3',
      ignored: 'value',
    })

    expect(filters).toEqual({
      q: '海报',
      purpose: 'marketing',
      style: 'minimal',
      subject: 'product',
      model: 'gpt-image-1',
      size: '1024x1536',
      reference: 'required',
      featured: true,
      favorite: false,
      sort: 'popular',
      page: 3,
    })
    expect(toPromptQuery(filters)).toEqual({
      q: '海报',
      purpose: 'marketing',
      style: 'minimal',
      subject: 'product',
      model: 'gpt-image-1',
      size: '1024x1536',
      reference: 'required',
      featured: 'true',
      sort: 'popular',
      page: '3',
    })
  })

  it('stores only the public template and variable definitions for image studio handoff', () => {
    sessionStorage.clear()
    const payload = {
      prompt_id: 'prompt-8',
      version: 4,
      title: '杂志产品海报',
      prompt_template: 'Create {{subject}} in editorial lighting',
      variables: [{ name: 'subject', label: '主体', required: true }],
      recommended_models: ['gpt-image-1'],
      recommended_sizes: ['1024x1536'],
      reference_requirement: 'none' as const,
    }

    storePromptUsePayload(payload)

    const key = promptSessionStorageKey('prompt-8', 4)
    expect(JSON.parse(sessionStorage.getItem(key) || '{}')).toEqual({
      prompt_id: 'prompt-8',
      version: 4,
      title: '杂志产品海报',
      prompt_template: payload.prompt_template,
      variables: payload.variables,
      recommended_models: ['gpt-image-1'],
      recommended_sizes: ['1024x1536'],
      reference_requirement: 'none',
    })
    expect(sessionStorage.getItem(key)).not.toContain('user_input')
  })

  it('defaults new external content to curated instead of original', () => {
    const draft = createDefaultAdminPromptDraft()

    expect(draft.source_attribution).toBe('curated')
    expect(draft.source_attribution).not.toBe('original')
  })
})
