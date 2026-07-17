import { beforeEach, describe, expect, it, vi } from 'vitest'

const { putMock } = vi.hoisted(() => ({
  putMock: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    put: putMock,
  },
}))

import { updateAdminPrompt } from '@/api/admin/prompts'
import { createDefaultAdminPromptDraft } from '@/utils/promptLibrary'

describe('管理员提示词接口', () => {
  beforeEach(() => {
    putMock.mockReset().mockResolvedValue({
      data: {
        id: 41,
        title_zh: '当前提示词',
        description_zh: '当前用途',
        prompt_text: 'current prompt',
        current_version: 8,
        status: 'draft',
      },
    })
  })

  it('sends both expected_version and current_version for an update', async () => {
    const draft = {
      ...createDefaultAdminPromptDraft(),
      title: '当前提示词',
      purpose_description: '当前用途',
      prompt_template: 'current prompt',
    }

    await updateAdminPrompt('41', draft, 7)

    expect(putMock).toHaveBeenCalledWith(
      '/admin/prompts/41',
      expect.objectContaining({
        expected_version: 7,
        current_version: 7,
      }),
    )
  })
})
