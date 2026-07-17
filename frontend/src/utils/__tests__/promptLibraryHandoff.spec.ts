import { beforeEach, describe, expect, it } from 'vitest'
import { loadPromptLibraryHandoff } from '@/utils/promptLibraryHandoff'

describe('提示词库交接数据', () => {
  beforeEach(() => {
    sessionStorage.clear()
  })

  it('removes valid handoff data immediately after reading it', () => {
    const key = 'prompt-library:12:3'
    sessionStorage.setItem(key, JSON.stringify({
      prompt_id: '12',
      version: 3,
      prompt_template: 'Create {{subject}}',
      variables: [],
    }))

    expect(loadPromptLibraryHandoff({ prompt: '12', version: '3' })?.prompt_id).toBe('12')
    expect(sessionStorage.getItem(key)).toBeNull()
  })

  it('also removes malformed handoff data after a failed read', () => {
    const key = 'prompt-library:12:3'
    sessionStorage.setItem(key, '{broken json')

    expect(loadPromptLibraryHandoff({ prompt: '12', version: '3' })).toBeNull()
    expect(sessionStorage.getItem(key)).toBeNull()
  })
})
