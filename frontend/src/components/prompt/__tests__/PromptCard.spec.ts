import { flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import PromptCard from '@/components/prompt/PromptCard.vue'
import type { PromptSummary } from '@/api/prompts'

const { pushMock, authState } = vi.hoisted(() => ({
  pushMock: vi.fn(),
  authState: { isAuthenticated: false },
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authState,
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ fullPath: '/prompts?q=海报&page=2' }),
  useRouter: () => ({ push: pushMock }),
}))

const prompt: PromptSummary = {
  id: 'prompt-1',
  slug: 'editorial-poster',
  title: '杂志感产品海报',
  purpose_description: '适合新品发布与品牌视觉。',
  prompt_template: 'Editorial product poster for {{product}}',
  variables: [],
  preview_image_url: 'https://images.example.com/poster.jpg',
  recommended_models: ['gpt-image-1'],
  recommended_sizes: ['1024x1536'],
  reference_requirement: 'none',
  source_attribution: 'curated',
  featured: true,
  favorite_count: 12,
  use_count: 35,
  is_favorited: false,
  version: 3,
}

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
          card: {
            use: '用于创作',
            previewAlt: '{title}示例效果',
            favorite: '收藏提示词',
            unfavorite: '取消收藏',
            stats: '提示词数据',
            useCount: '使用 {count}',
            favoriteCount: '收藏 {count}',
            copy: '复制提示词',
            generatedCoverAlt: '{title}生成封面',
          },
          source: { original: '极速蹬原创', authorized: '极速蹬授权', curated: '极速蹬精选', community: '极速蹬社区精选' },
          reference: { none: '无需参考图', optional: '可选参考图', required: '需要参考图' },
          cover: { purpose: { youtubeThumbnail: '视频封面' }, style: { cinematicFilmStill: '电影感' }, defaultKicker: '精选提示词', defaultBadge: '极速蹬' },
        },
      },
      en: {
        promptLibrary: {
          card: {
            use: 'Use in creation',
            previewAlt: '{title} preview',
            favorite: 'Favorite prompt',
            unfavorite: 'Remove prompt from favorites',
            stats: 'Prompt statistics',
            useCount: '{count} uses',
            favoriteCount: '{count} favorites',
            copy: 'Copy prompt',
            generatedCoverAlt: '{title} generated cover',
          },
          source: { original: 'Jisudeng original', authorized: 'Jisudeng licensed', curated: 'Jisudeng curated', community: 'Jisudeng community curated' },
          reference: { none: 'No reference image', optional: 'Reference image optional', required: 'Reference image required' },
          cover: { purpose: { youtubeThumbnail: 'Video cover' }, style: { cinematicFilmStill: 'Cinematic' }, defaultKicker: 'Curated prompt', defaultBadge: 'Jisudeng' },
        },
      },
    }) as any,
  })
}

function mountCard(locale: 'zh' | 'en', cardPrompt = prompt) {
  return mount(PromptCard, {
    props: { prompt: cardPrompt },
    global: { plugins: [promptI18n(locale)] },
  })
}

describe('PromptCard', () => {
  beforeEach(() => {
    authState.isAuthenticated = false
    pushMock.mockReset()
  })

  it('renders Chinese controls and only the 极速蹬 curated brand label', () => {
    const wrapper = mountCard('zh')

    expect(wrapper.text()).toContain('极速蹬精选')
    expect(wrapper.text()).not.toContain('curated')
    expect(wrapper.get('[aria-label="收藏提示词"]').exists()).toBe(true)
    expect(wrapper.get('[aria-label="复制提示词"]').exists()).toBe(true)
    expect(wrapper.findAll('[aria-label="用于创作"]')).toHaveLength(2)
    expect(wrapper.get('[aria-label="用于创作"]').exists()).toBe(true)
  })

  it('renders the entire prompt-card system surface in English without Chinese controls', () => {
    const wrapper = mountCard('en')

    expect(wrapper.text()).toContain('Jisudeng curated')
    expect(wrapper.text()).toContain('35 uses')
    expect(wrapper.text()).toContain('12 favorites')
    expect(wrapper.text()).not.toContain('极速蹬精选')
    expect(wrapper.get('[aria-label="Favorite prompt"]').exists()).toBe(true)
    expect(wrapper.get('[aria-label="Copy prompt"]').exists()).toBe(true)
    expect(wrapper.findAll('[aria-label="Use in creation"]')).toHaveLength(2)
  })

  it('uses the prompt when its cover is selected', async () => {
    const wrapper = mountCard('zh')

    await wrapper.get('.prompt-card-media').trigger('click')

    expect(wrapper.emitted('use')).toEqual([[prompt]])
  })

  it('sends guests to login with the complete current URL as redirect', async () => {
    const wrapper = mountCard('zh')

    await wrapper.get('[aria-label="收藏提示词"]').trigger('click')
    await flushPromises()

    expect(pushMock).toHaveBeenCalledWith({
      path: '/login',
      query: { redirect: '/prompts?q=海报&page=2' },
    })
    expect(wrapper.emitted('favorite')).toBeUndefined()
  })

  it('emits favorite directly for signed-in users', async () => {
    authState.isAuthenticated = true
    const wrapper = mountCard('zh')

    await wrapper.get('[aria-label="收藏提示词"]').trigger('click')

    expect(wrapper.emitted('favorite')).toEqual([[prompt]])
    expect(pushMock).not.toHaveBeenCalled()
  })

  it('uses a generated cover instead of repeated generic Image Studio template art', () => {
    const wrapper = mountCard('zh', {
      ...prompt,
      title: '电影感视频封面',
      preview_image_url: '/image-studio/templates/free-create.webp',
      purpose: 'youtube-thumbnail',
      style: 'cinematic-film-still',
    })

    expect(wrapper.find('.prompt-card-media img').exists()).toBe(false)
    expect(wrapper.get('.prompt-generated-cover').text()).toContain('电影感视频封面')
    expect(wrapper.get('.prompt-generated-cover').text()).toContain('视频封面')
  })

  it('localizes generated-cover system labels while preserving the user prompt title', () => {
    const wrapper = mountCard('en', {
      ...prompt,
      title: '电影感视频封面',
      preview_image_url: '/image-studio/templates/free-create.webp',
      purpose: 'youtube-thumbnail',
      style: 'cinematic-film-still',
    })

    expect(wrapper.get('.prompt-generated-cover').text()).toContain('电影感视频封面')
    expect(wrapper.get('.prompt-generated-cover').text()).toContain('Video cover')
    expect(wrapper.get('.prompt-generated-cover').text()).toContain('Cinematic')
    expect(wrapper.get('.prompt-generated-cover').attributes('aria-label')).toBe('电影感视频封面 generated cover')
  })
})
