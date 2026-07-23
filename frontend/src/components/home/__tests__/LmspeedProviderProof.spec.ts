import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { createI18n } from 'vue-i18n'

import LmspeedProviderProof from '../LmspeedProviderProof.vue'

const OG_URLS = [
  'https://lmspeed.net/api/og/provider/jisudeng/health',
  'https://lmspeed.net/api/og/provider/jisudeng/models',
  'https://lmspeed.net/api/og/provider/jisudeng/recent',
]

const messages = {
  zh: {
    home: {
      jisudeng: {
        lmspeedProof: {
          tag: () => 'LMSPEED STATUS',
          title: () => '第三方测速与状态',
          lede: () => '公开展示 LMSpeed 对极速蹬的健康检查、模型支持与最近测试记录。',
          gridLabel: () => '极速蹬 LMSpeed 第三方状态卡片',
          providerLink: () => '查看 LMSpeed 供应商页面',
          items: {
            health: {
              label: () => '健康检查',
              alt: () => '极速蹬 健康检查',
            },
            models: {
              label: () => '支持的模型',
              alt: () => '极速蹬 支持的模型',
            },
            recent: {
              label: () => '最近测试记录',
              alt: () => '极速蹬 最近测试记录',
            },
          },
        },
      },
    },
  },
  en: {
    home: {
      jisudeng: {
        lmspeedProof: {
          tag: () => 'LMSPEED STATUS',
          title: () => 'Third-party Speed and Status',
          lede: () => 'Public LMSpeed proof for Jisudeng health checks, supported models, and recent tests.',
          gridLabel: () => 'Jisudeng LMSpeed third-party status cards',
          providerLink: () => 'View LMSpeed provider page',
          items: {
            health: {
              label: () => 'Health Check',
              alt: () => 'Jisudeng Health Check',
            },
            models: {
              label: () => 'Supported Models',
              alt: () => 'Jisudeng Supported Models',
            },
            recent: {
              label: () => 'Recent Tests',
              alt: () => 'Jisudeng Recent Tests',
            },
          },
        },
      },
    },
  },
}

function mountProof(locale: 'zh' | 'en') {
  const i18n = createI18n({
    legacy: false,
    locale,
    messages,
  })

  return mount(LmspeedProviderProof, {
    global: { plugins: [i18n] },
  })
}

describe('LmspeedProviderProof', () => {
  it.each([
    ['zh', 'https://lmspeed.net/zh/provider/jisudeng', '第三方测速与状态', '查看 LMSpeed 供应商页面'],
    ['en', 'https://lmspeed.net/en/provider/jisudeng', 'Third-party Speed and Status', 'View LMSpeed provider page'],
  ] as const)('renders public provider proof cards with safe links in %s', (locale, providerUrl, title, providerLabel) => {
    const wrapper = mountProof(locale)

    expect(wrapper.text()).toContain(title)
    expect(wrapper.text()).toContain(providerLabel)

    const cardLinks = wrapper.findAll('[data-testid="lmspeed-provider-proof-card"]')
    expect(cardLinks).toHaveLength(3)
    for (const link of cardLinks) {
      expect(link.attributes('href')).toBe(providerUrl)
      expect(link.attributes('target')).toBe('_blank')
      expect(link.attributes('rel')).toBe('noopener noreferrer nofollow')
      expect(link.attributes('referrerpolicy')).toBe('no-referrer')
    }

    const images = wrapper.findAll('img')
    expect(images.map((image) => image.attributes('src'))).toEqual(OG_URLS)
    for (const image of images) {
      expect(image.attributes('loading')).toBe('lazy')
      expect(image.attributes('decoding')).toBe('async')
      expect(image.attributes('referrerpolicy')).toBe('no-referrer')
      expect(image.attributes('width')).toBe('1200')
    }
    expect(images[0].attributes('height')).toBe('424')
    expect(images[1].attributes('height')).toBe('192')
    expect(images[2].attributes('height')).toBe('192')

    expect(wrapper.html()).not.toContain('apiKey=')
    expect(wrapper.html()).not.toContain('baseUrl=')
    expect(wrapper.html()).not.toContain('sk-')
  })
})
