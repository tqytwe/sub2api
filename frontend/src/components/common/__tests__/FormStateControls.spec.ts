import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import Input from '../Input.vue'
import SearchInput from '../SearchInput.vue'
import Select from '../Select.vue'
import Skeleton from '../Skeleton.vue'
import Toast from '../Toast.vue'
import Toggle from '../Toggle.vue'
import { useAppStore } from '@/stores/app'

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

describe('shared form and state controls', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    document.body.innerHTML = ''
  })

  it('links Input labels, hints, and errors to the native field', () => {
    const hinted = mount(Input, {
      props: {
        id: 'quota-name',
        modelValue: '',
        label: '名称',
        hint: '用于后台识别'
      }
    })

    const hintedField = hinted.get('input')
    expect(hinted.get('label').attributes('for')).toBe('quota-name')
    expect(hintedField.attributes('aria-describedby')).toBe('quota-name-hint')
    expect(hinted.get('#quota-name-hint').text()).toBe('用于后台识别')

    const invalid = mount(Input, {
      props: {
        id: 'quota-limit',
        modelValue: '',
        error: '请输入有效额度'
      }
    })

    const invalidField = invalid.get('input')
    expect(invalidField.attributes('aria-invalid')).toBe('true')
    expect(invalidField.attributes('aria-describedby')).toBe('quota-limit-error')
    expect(invalid.get('#quota-limit-error').text()).toBe('请输入有效额度')
  })

  it('adds clear, disabled, and accessible-name behavior to SearchInput', async () => {
    const wrapper = mount(SearchInput, {
      props: {
        modelValue: 'target',
        placeholder: '搜索 API Key'
      }
    })

    const input = wrapper.get('input')
    expect(input.attributes('aria-label')).toBe('搜索 API Key')

    await wrapper.get('button[aria-label="common.clearSearch"]').trigger('click')

    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([''])
    expect(wrapper.emitted('search')?.[0]).toEqual([''])
    expect(wrapper.emitted('clear')).toHaveLength(1)

    const disabled = mount(SearchInput, {
      props: {
        modelValue: 'locked',
        disabled: true
      }
    })

    expect(disabled.find('button').exists()).toBe(false)
    await disabled.get('input').trigger('input')
    expect(disabled.emitted('update:modelValue')).toBeUndefined()
  })

  it('keeps Toggle model updates keyboard-friendly and blocks disabled changes', async () => {
    const wrapper = mount(Toggle, {
      props: {
        id: 'risk-toggle',
        modelValue: false,
        label: '风险控制',
        description: '命中后进入审核',
        error: '当前配置不可用'
      }
    })

    const button = wrapper.get('button[role="switch"]')
    expect(button.attributes('aria-checked')).toBe('false')
    expect(button.attributes('aria-label')).toBe('风险控制')
    expect(button.attributes('aria-describedby')).toBe('risk-toggle-description risk-toggle-error')

    await button.trigger('click')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([true])

    const disabled = mount(Toggle, {
      props: {
        modelValue: true,
        disabled: true
      }
    })

    await disabled.get('button').trigger('click')
    expect(disabled.emitted('update:modelValue')).toBeUndefined()
  })

  it('exposes Select trigger/listbox relationship without changing the value contract', async () => {
    const wrapper = mount(Select, {
      props: {
        modelValue: 'enabled',
        options: [
          { value: 'enabled', label: '已启用' },
          { value: 'disabled', label: '已停用' }
        ],
        ariaLabel: '状态筛选'
      },
      global: {
        stubs: {
          Teleport: true,
          Transition: false
        }
      }
    })

    const trigger = wrapper.get('button.select-trigger')
    expect(trigger.attributes('aria-label')).toBe('状态筛选')
    expect(trigger.attributes('aria-expanded')).toBe('false')

    await trigger.trigger('click')

    expect(trigger.attributes('aria-expanded')).toBe('true')
    const listboxId = trigger.attributes('aria-controls')
    expect(listboxId).toBeTruthy()
    expect(wrapper.get(`#${listboxId}`).attributes('role')).toBe('listbox')

    await wrapper.findAll('[role="option"]')[1].trigger('click')

    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual(['disabled'])
    expect(wrapper.emitted('change')?.[0]?.[0]).toBe('disabled')
  })

  it('marks Skeleton as decorative loading structure', () => {
    const wrapper = mount(Skeleton)
    expect(wrapper.get('div').attributes('aria-hidden')).toBe('true')
    expect(wrapper.get('div').classes()).toContain('skeleton')
  })

  it('announces error toasts assertively and keeps the close action accessible', async () => {
    const store = useAppStore()
    store.showError('保存失败', undefined)

    const wrapper = mount(Toast, {
      global: {
        stubs: {
          Teleport: true,
          TransitionGroup: false
        }
      }
    })

    const toast = wrapper.get('[role="alert"]')
    expect(toast.attributes('aria-live')).toBe('assertive')
    expect(toast.text()).toContain('保存失败')

    await wrapper.get('button[aria-label="common.close"]').trigger('click')
    expect(store.toasts).toHaveLength(0)
  })
})
