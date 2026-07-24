import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { describe, expect, it } from 'vitest'

import AboutView from '@/views/public/AboutView.vue'
import { jisudengPagesEn } from '@/i18n/locales/jisudeng-pages.en'
import { jisudengPagesZh } from '@/i18n/locales/jisudeng-pages.zh'

const CJK_RE = /[\u3400-\u9fff\uf900-\ufaff]/

function mountAbout(locale: 'en' | 'zh') {
  const i18n = createI18n({
    legacy: false,
    locale,
    fallbackLocale: 'zh',
    messages: {
      en: toRuntimeMessages(jisudengPagesEn),
      zh: toRuntimeMessages(jisudengPagesZh),
    },
  })

  return mount(AboutView, {
    global: {
      plugins: [i18n],
      stubs: {
        PublicPageToolbar: true,
        SupportFloatingCard: true,
        RouterLink: { props: ['to'], template: '<a><slot /></a>' },
      },
    },
  })
}

function toRuntimeMessages(value: unknown): unknown {
  if (typeof value === 'string') return () => value
  if (Array.isArray(value)) return value.map(toRuntimeMessages)
  if (value && typeof value === 'object') {
    return Object.fromEntries(
      Object.entries(value).map(([key, item]) => [key, toRuntimeMessages(item)]),
    )
  }
  return value
}

describe('AboutView localized content', () => {
  it('renders the English about page without Chinese text leakage', () => {
    const wrapper = mountAbout('en')

    expect(wrapper.text()).toContain('Why we are a real relay')
    expect(wrapper.text()).toContain('Transparent channels')
    expect(wrapper.text()).not.toMatch(CJK_RE)
  })

  it('keeps the Chinese about page in Chinese', () => {
    const wrapper = mountAbout('zh')

    expect(wrapper.text()).toContain('为什么我们是真的中转')
    expect(wrapper.text()).toContain('渠道透明')
  })
})
