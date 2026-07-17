import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import PromptFilters from '@/components/prompt/PromptFilters.vue'
import { DEFAULT_PROMPT_FILTERS } from '@/utils/promptLibrary'

describe('PromptFilters', () => {
  afterEach(() => {
    document.body.innerHTML = ''
    document.body.style.overflow = ''
  })

  it('opens and closes the mobile filter drawer with accessible Chinese controls', async () => {
    const wrapper = mount(PromptFilters, {
      attachTo: document.body,
      props: {
        modelValue: { ...DEFAULT_PROMPT_FILTERS },
        categories: [],
      },
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
    const wrapper = mount(PromptFilters, {
      attachTo: document.body,
      props: {
        modelValue: { ...DEFAULT_PROMPT_FILTERS },
        categories: [{
          id: 1,
          name: '极简',
          slug: 'minimal',
          dimension: 'style',
        }],
      },
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
    const wrapper = mount(PromptFilters, {
      attachTo: document.body,
      props: {
        modelValue: { ...DEFAULT_PROMPT_FILTERS },
        categories: [],
      },
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
})
