import { flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { describe, expect, it, vi } from 'vitest'
import PromptLibraryPanel from '@/components/prompt/PromptLibraryPanel.vue'

const { appStore, authState } = vi.hoisted(() => ({
  appStore: { showError: vi.fn(), showSuccess: vi.fn() },
  authState: { isAuthenticated: false },
}))

vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => authState }))
vi.mock('vue-router', () => ({
  useRoute: () => ({ fullPath: '/ai-creation-space' }),
  useRouter: () => ({ push: vi.fn() }),
}))
vi.mock('@/api/prompts', () => ({
  favoritePrompt: vi.fn(),
  getPrompt: vi.fn(),
  listPromptCategories: vi.fn().mockResolvedValue([]),
  listPrompts: vi.fn().mockResolvedValue({ items: [], total: 0, page: 1, page_size: 12, pages: 0 }),
  unfavoritePrompt: vi.fn(),
  usePrompt: vi.fn(),
}))

function runtimeMessages(value: unknown): unknown {
  if (typeof value === 'string') {
    return (context: { named?: (name: string) => unknown }) =>
      value.replace(/\{(\w+)\}/g, (_match, name) => String(context.named?.(name) ?? ''))
  }
  if (Array.isArray(value)) return value.map(runtimeMessages)
  if (value && typeof value === 'object') {
    return Object.fromEntries(Object.entries(value).map(([key, item]) => [key, runtimeMessages(item)]))
  }
  return value
}

function promptI18n(locale: 'zh' | 'en') {
  return createI18n({
    legacy: false,
    locale,
    fallbackLocale: false,
    messages: runtimeMessages({
      zh: {
        promptLibrary: {
          panel: {
            title: '选提示词', description: '从极速蹬提示词库选择一个创作起点，带入 AI创作空间继续编辑。',
            featured: '极速蹬精选', latest: '最新收录', popular: '热门使用', noReference: '无需参考图', favorites: '我的收藏', quickLinks: '快捷入口',
            loading: '正在加载提示词', resultCount: '共找到 {count} 条提示词', sort: '排序方式', sortFeatured: '精选优先', sortLatest: '最新收录', sortPopular: '热门使用',
            loadingLabel: '正在加载', list: '提示词列表', loadFailed: '提示词加载失败', loadFailedHint: '网络暂时不可用，请稍后重试。', retry: '重新加载',
            empty: '没有找到匹配的提示词', emptyHint: '可以清除筛选，或换一个用途、风格继续查找。', reset: '清除筛选',
          },
        },
      },
      en: {
        promptLibrary: {
          panel: {
            title: 'Choose a prompt', description: 'Choose a starting point from the Jisudeng prompt library and keep editing in AI Creation Space.',
            featured: 'Jisudeng curated', latest: 'Latest additions', popular: 'Most used', noReference: 'No reference image', favorites: 'My favorites', quickLinks: 'Quick links',
            loading: 'Loading prompts', resultCount: '{count} prompts found', sort: 'Sort prompts', sortFeatured: 'Curated first', sortLatest: 'Latest additions', sortPopular: 'Most used',
            loadingLabel: 'Loading', list: 'Prompt list', loadFailed: 'Could not load prompts', loadFailedHint: 'The network is temporarily unavailable. Try again shortly.', retry: 'Reload',
            empty: 'No matching prompts', emptyHint: 'Clear filters or try another purpose or style.', reset: 'Clear filters',
          },
        },
      },
    }) as any,
  })
}

describe('PromptLibraryPanel', () => {
  it('renders all system labels in English while retaining data-driven prompt names unchanged', async () => {
    const wrapper = mount(PromptLibraryPanel, {
      global: {
        plugins: [promptI18n('en')],
        stubs: {
          Icon: true,
          Pagination: true,
          PromptCard: true,
          PromptFilters: true,
        },
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('Choose a prompt')
    expect(wrapper.text()).toContain('Jisudeng curated')
    expect(wrapper.text()).toContain('No matching prompts')
    expect(wrapper.text()).not.toContain('选提示词')
    expect(wrapper.get('[aria-label="Quick links"]').exists()).toBe(true)
    expect(wrapper.get('[aria-label="Sort prompts"]').exists()).toBe(true)
  })
})
