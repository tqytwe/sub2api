import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { describe, expect, it } from 'vitest'
import PublicContentLayout from '@/components/layout/PublicContentLayout.vue'

function runtimeMessages(value: unknown): unknown {
  if (typeof value === 'string') return () => value
  if (value && typeof value === 'object') {
    return Object.fromEntries(Object.entries(value).map(([key, item]) => [key, runtimeMessages(item)]))
  }
  return value
}

function layoutI18n(locale: 'zh' | 'en') {
  return createI18n({
    legacy: false,
    locale,
    fallbackLocale: false,
    messages: runtimeMessages({
      zh: { nav: { publicActions: '公共页面操作' }, home: { viewDocs: '查看文档', docs: '文档', footer: { allRightsReserved: '保留所有权利' } } },
      en: { nav: { publicActions: 'Public page actions' }, home: { viewDocs: 'View docs', docs: 'Docs', footer: { allRightsReserved: 'All rights reserved' } } },
    }) as any,
  })
}

describe('PublicContentLayout localized branding', () => {
  it('uses the English Jisudeng fallback when no configured site name is available', () => {
    const wrapper = mount(PublicContentLayout, {
      global: {
        plugins: [layoutI18n('en')],
        stubs: { RouterLink: { template: '<a><slot /></a>' }, Icon: true, PublicPageToolbar: true },
      },
    })

    expect(wrapper.text()).toContain('Jisudeng')
    expect(wrapper.text()).not.toContain('极速蹬')
    const logo = wrapper.get('img')
    expect(logo.attributes('src')).toBe('/logo.png')
    expect(logo.attributes('alt')).toBe('Jisudeng')
    expect(logo.classes()).toContain('brand-logo-asset--deng')
  })

  it('uses the Chinese 极速蹬 fallback on the default Chinese surface', () => {
    const wrapper = mount(PublicContentLayout, {
      global: {
        plugins: [layoutI18n('zh')],
        stubs: { RouterLink: { template: '<a><slot /></a>' }, Icon: true, PublicPageToolbar: true },
      },
    })

    expect(wrapper.text()).toContain('极速蹬')
    expect(wrapper.get('img').attributes('alt')).toBe('极速蹬')
  })

  it('keeps a configured logo at its native object-contain scale', () => {
    const wrapper = mount(PublicContentLayout, {
      props: { siteLogo: '/tenant-logo.png' },
      global: {
        plugins: [layoutI18n('zh')],
        stubs: { RouterLink: { template: '<a><slot /></a>' }, Icon: true, PublicPageToolbar: true },
      },
    })

    const logo = wrapper.get('img')
    expect(logo.attributes('src')).toBe('/tenant-logo.png')
    expect(logo.classes()).not.toContain('brand-logo-asset--deng')
  })
})
