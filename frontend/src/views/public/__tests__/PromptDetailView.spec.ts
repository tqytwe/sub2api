import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import PromptDetailView from '@/views/public/PromptDetailView.vue'
import { promptSessionStorageKey } from '@/utils/promptLibrary'

const {
  favoritePromptMock,
  getPromptMock,
  unfavoritePromptMock,
  usePromptMock,
  pushMock,
  showErrorMock,
  showSuccessMock,
} = vi.hoisted(() => ({
  favoritePromptMock: vi.fn(),
  getPromptMock: vi.fn(),
  unfavoritePromptMock: vi.fn(),
  usePromptMock: vi.fn(),
  pushMock: vi.fn(),
  showErrorMock: vi.fn(),
  showSuccessMock: vi.fn(),
}))

vi.mock('@/api/prompts', () => ({
  getPrompt: (...args: unknown[]) => getPromptMock(...args),
  usePrompt: (...args: unknown[]) => usePromptMock(...args),
  favoritePrompt: (...args: unknown[]) => favoritePromptMock(...args),
  unfavoritePrompt: (...args: unknown[]) => unfavoritePromptMock(...args),
  reportPrompt: vi.fn(),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ isAuthenticated: true }),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: showErrorMock,
    showSuccess: showSuccessMock,
  }),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: 'prompt-1' }, fullPath: '/prompts/prompt-1' }),
  useRouter: () => ({ push: pushMock }),
}))

describe('PromptDetailView', () => {
  beforeEach(() => {
    sessionStorage.clear()
    getPromptMock.mockReset().mockResolvedValue({
      id: 'prompt-1',
      title: '电影感人像',
      purpose_description: '用于制作电影感人物视觉。',
      prompt_template: 'Cinematic portrait of {{subject}}',
      variables: [{ name: 'subject', label: '人物主体', required: true }],
      preview_image_url: 'https://images.example.com/portrait.jpg',
      recommended_models: ['gpt-image-1'],
      recommended_sizes: ['1024x1536'],
      reference_requirement: 'optional',
      source_attribution: 'curated',
      featured: true,
      version: 2,
      favorite_count: 2,
      use_count: 8,
      is_favorited: false,
    })
    favoritePromptMock.mockReset()
    unfavoritePromptMock.mockReset()
    usePromptMock.mockReset().mockResolvedValue({
      prompt_id: 'prompt-1',
      version: 5,
      title: '电影感人像',
      prompt_template: 'Cinematic portrait of {{subject}}, soft rim light',
      variables: [{ name: 'subject', label: '人物主体', required: true }],
      recommended_models: ['gpt-image-1'],
      recommended_sizes: ['1024x1536'],
      reference_requirement: 'optional',
    })
    pushMock.mockReset()
    showErrorMock.mockReset()
    showSuccessMock.mockReset()
  })

  it('stores the returned current public version before navigating to image studio', async () => {
    const wrapper = mount(PromptDetailView, {
      global: {
        stubs: {
          PublicPageToolbar: true,
          RouterLink: { template: '<a><slot /></a>' },
        },
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('生成提示词（英文）')
    expect(wrapper.text()).toContain('极速蹬精选')
    await wrapper.get('[aria-label="用于创作"]').trigger('click')
    await flushPromises()

    expect(usePromptMock).toHaveBeenCalledWith('prompt-1')
    expect(JSON.parse(sessionStorage.getItem(promptSessionStorageKey('prompt-1', 5)) || '{}')).toEqual({
      prompt_id: 'prompt-1',
      version: 5,
      title: '电影感人像',
      prompt_template: 'Cinematic portrait of {{subject}}, soft rim light',
      variables: [{ name: 'subject', label: '人物主体', required: true }],
      recommended_models: ['gpt-image-1'],
      recommended_sizes: ['1024x1536'],
      reference_requirement: 'optional',
    })
    expect(pushMock).toHaveBeenCalledWith('/image-studio?prompt=prompt-1&version=5')
  })

  it('uses the final server favorite state and count without guessing', async () => {
    favoritePromptMock.mockResolvedValueOnce({
      favorited: true,
      favorite_count: 11,
    })
    unfavoritePromptMock.mockResolvedValueOnce({
      favorited: false,
    })
    const wrapper = mount(PromptDetailView, {
      global: {
        stubs: {
          PublicPageToolbar: true,
          RouterLink: { template: '<a><slot /></a>' },
          Icon: true,
        },
      },
    })
    await flushPromises()

    await wrapper.get('[aria-label="收藏提示词"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[aria-label="取消收藏"]').exists()).toBe(true)
    expect((wrapper.vm as unknown as { prompt: { favorite_count: number } }).prompt.favorite_count)
      .toBe(11)

    await wrapper.get('[aria-label="取消收藏"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[aria-label="收藏提示词"]').exists()).toBe(true)
    expect((wrapper.vm as unknown as { prompt: { favorite_count: number } }).prompt.favorite_count)
      .toBe(11)
  })
})
