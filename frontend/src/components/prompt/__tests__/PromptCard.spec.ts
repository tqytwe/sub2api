import { flushPromises, mount } from '@vue/test-utils'
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

describe('PromptCard', () => {
  beforeEach(() => {
    authState.isAuthenticated = false
    pushMock.mockReset()
  })

  it('renders Chinese controls and only the 极速蹬 curated brand label', () => {
    const wrapper = mount(PromptCard, { props: { prompt } })

    expect(wrapper.text()).toContain('极速蹬精选')
    expect(wrapper.text()).not.toContain('curated')
    expect(wrapper.get('[aria-label="收藏提示词"]').exists()).toBe(true)
    expect(wrapper.get('[aria-label="复制提示词"]').exists()).toBe(true)
    expect(wrapper.get('[aria-label="查看详情"]').exists()).toBe(true)
    expect(wrapper.get('[aria-label="用于创作"]').exists()).toBe(true)
  })

  it('sends guests to login with the complete current URL as redirect', async () => {
    const wrapper = mount(PromptCard, { props: { prompt } })

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
    const wrapper = mount(PromptCard, { props: { prompt } })

    await wrapper.get('[aria-label="收藏提示词"]').trigger('click')

    expect(wrapper.emitted('favorite')).toEqual([[prompt]])
    expect(pushMock).not.toHaveBeenCalled()
  })

  it('uses a generated cover instead of repeated generic Image Studio template art', () => {
    const wrapper = mount(PromptCard, {
      props: {
        prompt: {
          ...prompt,
          title: '电影感视频封面',
          preview_image_url: '/image-studio/templates/free-create.webp',
          purpose: 'youtube-thumbnail',
          style: 'cinematic-film-still',
        },
      },
    })

    expect(wrapper.find('.prompt-card-media img').exists()).toBe(false)
    expect(wrapper.get('.prompt-generated-cover').text()).toContain('电影感视频封面')
    expect(wrapper.get('.prompt-generated-cover').text()).toContain('视频封面')
  })
})
