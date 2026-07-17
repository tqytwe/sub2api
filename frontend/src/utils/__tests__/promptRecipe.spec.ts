import { beforeEach, describe, expect, it } from 'vitest'
import {
  listPromptRecipes,
  savePromptRecipe,
} from '@/utils/promptRecipe'

describe('prompt creation recipes', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('stores only the public template reference, generation spec, and variable structure', () => {
    savePromptRecipe({
      promptId: '123',
      promptVersion: 4,
      title: '夏日饮品海报',
      model: 'gpt-image-1',
      size: '1024x1024',
      quality: 'high',
      variables: [
        { name: 'product', label: '产品名称', required: true },
      ],
      variableValues: {
        product: '不应保存的私密商品名',
      },
      finalPrompt: 'This plaintext must not be stored.',
    })

    const raw = localStorage.getItem('jisudeng-prompt-recipes:v1') ?? ''
    expect(raw).not.toContain('不应保存的私密商品名')
    expect(raw).not.toContain('This plaintext must not be stored.')
    expect(listPromptRecipes()).toEqual([
      expect.objectContaining({
        prompt_id: '123',
        prompt_version: 4,
        title: '夏日饮品海报',
        model: 'gpt-image-1',
        size: '1024x1024',
        quality: 'high',
        variables: [{ name: 'product', label: '产品名称', required: true }],
      }),
    ])
  })
})
