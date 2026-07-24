import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import PromptSquareView from '@/views/public/PromptSquareView.vue'

const {
  favoritePromptMock,
  i18nState,
  listPromptsMock,
  pushMock,
  routerBackMock,
  unfavoritePromptMock,
} = vi.hoisted(() => ({
  favoritePromptMock: vi.fn(),
  i18nState: { locale: 'zh' as 'en' | 'zh' },
  listPromptsMock: vi.fn(),
  pushMock: vi.fn(),
  routerBackMock: vi.fn(),
  unfavoritePromptMock: vi.fn(),
}))

const messages: Record<'en' | 'zh', Record<string, string>> = {
  en: {
    'promptLibrary.back': 'Back',
    'promptLibrary.backAria': 'Go back',
    'promptLibrary.metaTitle': 'Image Studio prompt library',
    'promptLibrary.metaDescription': 'Find image prompts by use case and visual attributes.',
    'promptLibrary.englishPendingEyebrow': 'Image Studio prompt library',
    'promptLibrary.englishPendingTitle': 'English prompt library is being prepared',
    'promptLibrary.englishPendingBody': 'The current prompt collection is available in the default language. The English collection will open after the content is reviewed.',
    'promptLibrary.switchToDefaultLanguage': 'View current library',
  },
  zh: {
    'promptLibrary.back': '返回',
    'promptLibrary.backAria': '返回上一页',
    'promptLibrary.metaTitle': '图像工作室 · 选提示词',
    'promptLibrary.metaDescription': '在图像工作室内按用途、风格、主体、模型和尺寸查找提示词，并用于图像创作。',
    'promptLibrary.englishPendingEyebrow': '图像工作室 · 提示词库',
    'promptLibrary.englishPendingTitle': '英文提示词库正在准备',
    'promptLibrary.englishPendingBody': '当前提示词内容先以默认语言开放，英文内容审核完成后会单独开放。',
    'promptLibrary.switchToDefaultLanguage': '查看当前提示词库',
  },
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[i18nState.locale][key] ?? key,
      locale: { value: i18nState.locale },
    }),
  }
})

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
    i18nState.locale = 'zh'
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

  it('falls back to the English home route in an English locale shell', async () => {
    i18nState.locale = 'en'
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

    await wrapper.get('[aria-label="Go back"]').trigger('click')

    expect(routerBackMock).not.toHaveBeenCalled()
    expect(pushMock).toHaveBeenCalledWith('/en')
  })

  it('does not render the Chinese prompt collection inside an English locale shell', async () => {
    i18nState.locale = 'en'

    const wrapper = mount(PromptSquareView, {
      global: {
        stubs: {
          PromptCard: {
            props: ['prompt'],
            template: '<article>{{ prompt.title }}</article>',
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

    expect(listPromptsMock).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('English prompt library is being prepared')
    expect(wrapper.text()).not.toContain('产品海报')
  })
})
