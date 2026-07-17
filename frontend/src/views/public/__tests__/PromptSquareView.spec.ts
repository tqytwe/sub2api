import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import PromptSquareView from '@/views/public/PromptSquareView.vue'

const {
  favoritePromptMock,
  listPromptsMock,
  pushMock,
  routerBackMock,
  unfavoritePromptMock,
} = vi.hoisted(() => ({
  favoritePromptMock: vi.fn(),
  listPromptsMock: vi.fn(),
  pushMock: vi.fn(),
  routerBackMock: vi.fn(),
  unfavoritePromptMock: vi.fn(),
}))

vi.mock('@/api/prompts', () => ({
  favoritePrompt: (...args: unknown[]) => favoritePromptMock(...args),
  unfavoritePrompt: (...args: unknown[]) => unfavoritePromptMock(...args),
  listPrompts: (...args: unknown[]) => listPromptsMock(...args),
  listPromptCategories: vi.fn().mockResolvedValue([]),
  getPrompt: vi.fn(),
  usePrompt: vi.fn(),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ isAuthenticated: true }),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
  }),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {}, fullPath: '/prompts' }),
  useRouter: () => ({ back: routerBackMock, replace: vi.fn(), push: pushMock }),
}))

describe('PromptSquareView', () => {
  beforeEach(() => {
    favoritePromptMock.mockReset()
    unfavoritePromptMock.mockReset()
    pushMock.mockReset()
    routerBackMock.mockReset()
    listPromptsMock.mockReset().mockResolvedValue({
      items: [{
        id: 'prompt-1',
        title: '产品海报',
        purpose_description: '制作产品海报',
        prompt_template: 'Product poster',
        variables: [],
        recommended_models: [],
        recommended_sizes: [],
        reference_requirement: 'none',
        source_attribution: 'curated',
        featured: false,
        favorite_count: 4,
        use_count: 2,
        is_favorited: false,
        version: 1,
      }],
      total: 1,
      page: 1,
      page_size: 24,
      pages: 1,
    })
  })

  it('applies the favorite state and count returned by the server', async () => {
    favoritePromptMock.mockResolvedValueOnce({
      favorited: true,
      favorite_count: 20,
    })
    const wrapper = mount(PromptSquareView, {
      global: {
        stubs: {
          PromptCard: {
            props: ['prompt'],
            emits: ['favorite'],
            template: '<button data-testid="favorite" @click="$emit(\'favorite\', prompt)">{{ prompt.favorite_count }}-{{ prompt.is_favorited }}</button>',
          },
          PromptFilters: true,
          Pagination: true,
          PublicPageToolbar: true,
          RouterLink: { template: '<a><slot /></a>' },
          Icon: true,
        },
      },
    })
    await flushPromises()

    await wrapper.get('[data-testid="favorite"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="favorite"]').text()).toBe('20-true')
  })

  it('returns to the previous page when browser history exists', async () => {
    Object.defineProperty(window.history, 'length', { configurable: true, value: 2 })
    const wrapper = mount(PromptSquareView, {
      global: {
        stubs: {
          PromptCard: true,
          PromptFilters: true,
          Pagination: true,
          PublicPageToolbar: true,
          Icon: true,
        },
      },
    })
    await flushPromises()

    await wrapper.get('[aria-label="返回上一页"]').trigger('click')

    expect(routerBackMock).toHaveBeenCalledTimes(1)
    expect(pushMock).not.toHaveBeenCalled()
  })

  it('falls back to home when opened directly', async () => {
    Object.defineProperty(window.history, 'length', { configurable: true, value: 1 })
    const wrapper = mount(PromptSquareView, {
      global: {
        stubs: {
          PromptCard: true,
          PromptFilters: true,
          Pagination: true,
          PublicPageToolbar: true,
          Icon: true,
        },
      },
    })
    await flushPromises()

    await wrapper.get('[aria-label="返回上一页"]').trigger('click')

    expect(routerBackMock).not.toHaveBeenCalled()
    expect(pushMock).toHaveBeenCalledWith('/home')
  })
})
