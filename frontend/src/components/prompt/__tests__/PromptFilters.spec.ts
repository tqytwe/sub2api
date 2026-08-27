import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { afterEach, describe, expect, it } from 'vitest'
import PromptFilters from '@/components/prompt/PromptFilters.vue'
import { DEFAULT_PROMPT_FILTERS } from '@/utils/promptLibrary'

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
        common: { filter: '筛选' },
        promptLibrary: {
          filters: {
            searchLabel: '搜索提示词', searchPlaceholder: '搜索标题、用途或画面描述', open: '打开筛选', title: '筛选提示词', close: '关闭筛选',
            groupLabel: '提示词筛选', purpose: '用途', style: '风格', subject: '主体', model: '模型', size: '尺寸', reference: '参考图',
            all: '全部', none: '无需参考图', optional: '可选参考图', required: '需要参考图', reset: '清除筛选', apply: '查看结果',
          },
        },
      },
      en: {
        common: { filter: 'Filter' },
        promptLibrary: {
          filters: {
            searchLabel: 'Search prompts', searchPlaceholder: 'Search title, purpose, or scene description', open: 'Open filters', title: 'Filter prompts', close: 'Close filters',
            groupLabel: 'Prompt filters', purpose: 'Purpose', style: 'Style', subject: 'Subject', model: 'Model', size: 'Size', reference: 'Reference image',
            all: 'All', none: 'No reference image', optional: 'Reference image optional', required: 'Reference image required', reset: 'Clear filters', apply: 'View results',
          },
        },
      },
    }) as any,
  })
}

function mountFilters(locale: 'zh' | 'en', props: Record<string, unknown>) {
  return mount(PromptFilters, {
    attachTo: document.body,
    props: props as any,
    global: { plugins: [promptI18n(locale)] },
  })
}

describe('PromptFilters', () => {
  afterEach(() => {
    document.body.innerHTML = ''
    document.body.style.overflow = ''
  })

  it('opens and closes the mobile filter drawer with accessible Chinese controls', async () => {
    const wrapper = mountFilters('zh', {
      modelValue: { ...DEFAULT_PROMPT_FILTERS },
      categories: [],
    })

    expect(document.querySelector('[data-testid="prompt-filter-drawer"]')).toBeNull()
    await wrapper.get('[aria-label="打开筛选"]').trigger('click')
    const drawer = document.querySelector('[data-testid="prompt-filter-drawer"]')
    expect(drawer?.textContent).toContain('筛选提示词')
    expect(document.querySelector('[aria-label="关闭筛选"]')).not.toBeNull()

    ;(document.querySelector('[aria-label="关闭筛选"]') as HTMLButtonElement).click()
    await wrapper.vm.$nextTick()
    expect(document.querySelector('[data-testid="prompt-filter-drawer"]')).toBeNull()
    wrapper.unmount()
  })

  it('keeps mobile changes in a draft until viewing results and discards them on close', async () => {
    const wrapper = mountFilters('zh', {
      modelValue: { ...DEFAULT_PROMPT_FILTERS },
      categories: [{
        id: 1,
        name: '极简',
        slug: 'minimal',
        dimension: 'style',
      }],
    })

    await wrapper.get('[aria-label="打开筛选"]').trigger('click')
    const firstDrawer = document.querySelector('[data-testid="prompt-filter-drawer"]')!
    const styleSelect = firstDrawer.querySelectorAll('select')[1] as HTMLSelectElement
    styleSelect.value = 'minimal'
    styleSelect.dispatchEvent(new Event('change', { bubbles: true }))
    await wrapper.vm.$nextTick()

    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    ;(document.querySelector('[aria-label="关闭筛选"]') as HTMLButtonElement).click()
    await wrapper.vm.$nextTick()
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()

    await wrapper.get('[aria-label="打开筛选"]').trigger('click')
    const secondDrawer = document.querySelector('[data-testid="prompt-filter-drawer"]')!
    const secondStyleSelect = secondDrawer.querySelectorAll('select')[1] as HTMLSelectElement
    secondStyleSelect.value = 'minimal'
    secondStyleSelect.dispatchEvent(new Event('change', { bubbles: true }))
    ;(Array.from(secondDrawer.querySelectorAll('button'))
      .find((button) => button.textContent?.includes('查看结果')) as HTMLButtonElement).click()
    await wrapper.vm.$nextTick()

    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([{
      ...DEFAULT_PROMPT_FILTERS,
      style: 'minimal',
    }])
    expect(wrapper.emitted('apply')).toEqual([[]])
    wrapper.unmount()
  })

  it('locks scrolling while open and closes without applying on Escape', async () => {
    document.body.style.overflow = 'auto'
    const wrapper = mountFilters('zh', {
      modelValue: { ...DEFAULT_PROMPT_FILTERS },
      categories: [],
    })

    await wrapper.get('[aria-label="打开筛选"]').trigger('click')
    expect(document.body.style.overflow).toBe('hidden')

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await wrapper.vm.$nextTick()

    expect(document.querySelector('[data-testid="prompt-filter-drawer"]')).toBeNull()
    expect(document.body.style.overflow).toBe('auto')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    wrapper.unmount()
  })

  it('localizes filter labels, options, and accessible drawer controls in English', async () => {
    const wrapper = mountFilters('en', {
      modelValue: { ...DEFAULT_PROMPT_FILTERS },
      categories: [],
    })

    expect(wrapper.get('input').attributes('placeholder')).toBe('Search title, purpose, or scene description')
    expect(wrapper.get('[aria-label="Open filters"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('Reference image')
    expect(wrapper.text()).not.toContain('筛选')

    await wrapper.get('[aria-label="Open filters"]').trigger('click')
    const drawer = document.querySelector('[data-testid="prompt-filter-drawer"]')
    expect(drawer?.textContent).toContain('Filter prompts')
    expect(document.querySelector('[aria-label="Close filters"]')).not.toBeNull()
    wrapper.unmount()
  })
})
