import { flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { describe, expect, it } from 'vitest'

import BaseDialog from '@/components/common/BaseDialog.vue'

function mountDialog(locale: 'zh' | 'en', showCloseButton = true, attachTo?: HTMLElement) {
  const i18n = createI18n({
    legacy: false,
    locale,
    messages: {
      zh: { common: { close: () => '关闭' } },
      en: { common: { close: () => 'Close' } },
    },
  })
  return mount(BaseDialog, {
    props: {
      show: true,
      title: locale === 'zh' ? '测试弹窗' : 'Test dialog',
      showCloseButton,
    },
    global: {
      plugins: [i18n],
      stubs: {
        Teleport: true,
        Transition: false,
      },
    },
    slots: {
      default: `
        <button data-testid="first-action">First</button>
        <button data-testid="last-action">Last</button>
      `,
    },
    attachTo,
  })
}

describe('BaseDialog locale labels', () => {
  it.each([
    ['zh', '关闭'],
    ['en', 'Close'],
  ] as const)('uses the active %s locale for the default close label', (locale, expected) => {
    const wrapper = mountDialog(locale)
    expect(wrapper.get('button[aria-label]').attributes('aria-label')).toBe(expected)
    wrapper.unmount()
  })

  it('traps forward and backward Tab navigation inside the dialog', async () => {
    const wrapper = mountDialog('en', false, document.body)
    await flushPromises()
    const first = wrapper.get('[data-testid="first-action"]').element as HTMLButtonElement
    const last = wrapper.get('[data-testid="last-action"]').element as HTMLButtonElement

    last.focus()
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true }))
    expect(document.activeElement).toBe(first)

    first.focus()
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', shiftKey: true, bubbles: true }))
    expect(document.activeElement).toBe(last)
    wrapper.unmount()
  })
})
