import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, nextTick, ref } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useDialogAccessibility } from '@/composables/useDialogAccessibility'

const Harness = defineComponent({
  setup() {
    const open = ref(false)
    const dialogRef = ref<HTMLElement | null>(null)
    const close = vi.fn(() => {
      open.value = false
    })

    useDialogAccessibility(open, dialogRef, {
      onClose: close,
    })

    return {
      close,
      dialogRef,
      open,
    }
  },
  template: `
    <button data-testid="launcher" @click="open = true">Open</button>
    <section v-if="open" ref="dialogRef" role="dialog" tabindex="-1">
      <button data-testid="first">First</button>
      <button data-testid="last">Last</button>
    </section>
  `,
})

const StackedHarness = defineComponent({
  setup() {
    const firstOpen = ref(false)
    const secondOpen = ref(false)
    const firstDialogRef = ref<HTMLElement | null>(null)
    const secondDialogRef = ref<HTMLElement | null>(null)
    const closeFirst = vi.fn(() => {
      firstOpen.value = false
    })
    const closeSecond = vi.fn(() => {
      secondOpen.value = false
    })

    useDialogAccessibility(firstOpen, firstDialogRef, {
      onClose: closeFirst,
    })
    useDialogAccessibility(secondOpen, secondDialogRef, {
      onClose: closeSecond,
    })

    return {
      closeFirst,
      closeSecond,
      firstDialogRef,
      firstOpen,
      secondDialogRef,
      secondOpen,
    }
  },
  template: `
    <button data-testid="open-first" @click="firstOpen = true">Open first</button>
    <section v-if="firstOpen" ref="firstDialogRef" role="dialog" tabindex="-1">
      <button data-testid="first-action">First action</button>
      <button data-testid="open-second" @click="secondOpen = true">Open second</button>
    </section>
    <section v-if="secondOpen" ref="secondDialogRef" role="dialog" tabindex="-1">
      <button data-testid="second-first">Second first</button>
      <button data-testid="second-last">Second last</button>
    </section>
  `,
})

describe('useDialogAccessibility', () => {
  beforeEach(() => {
    document.body.className = ''
    document.body.innerHTML = '<main id="app"><button id="outside">Outside</button></main>'
  })

  it('locks background, traps Tab, closes on Escape, and restores focus', async () => {
    const wrapper = mount(Harness, { attachTo: document.body })
    const launcher = wrapper.get('[data-testid="launcher"]').element as HTMLButtonElement
    const appRoot = document.getElementById('app') as HTMLElement & { inert?: boolean }

    launcher.focus()
    await wrapper.get('[data-testid="launcher"]').trigger('click')
    await nextTick()
    await flushPromises()

    const first = wrapper.get('[data-testid="first"]').element as HTMLButtonElement
    const last = wrapper.get('[data-testid="last"]').element as HTMLButtonElement
    expect(document.body.classList.contains('modal-open')).toBe(true)
    expect(appRoot.getAttribute('aria-hidden')).toBe('true')
    expect(appRoot.inert).toBe(true)
    expect(document.activeElement).toBe(first)

    last.focus()
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true }))
    expect(document.activeElement).toBe(first)

    first.focus()
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', shiftKey: true, bubbles: true }))
    expect(document.activeElement).toBe(last)

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await nextTick()
    expect(wrapper.vm.close).toHaveBeenCalledOnce()
    expect(document.body.classList.contains('modal-open')).toBe(false)
    expect(appRoot.getAttribute('aria-hidden')).toBeNull()
    expect(appRoot.inert).toBe(false)
    expect(document.activeElement).toBe(launcher)

    wrapper.unmount()
  })

  it('lets only the topmost dialog handle Escape and Tab', async () => {
    const wrapper = mount(StackedHarness, { attachTo: document.body })
    const appRoot = document.getElementById('app') as HTMLElement & { inert?: boolean }

    await wrapper.get('[data-testid="open-first"]').trigger('click')
    await nextTick()
    await flushPromises()
    const firstAction = wrapper.get('[data-testid="first-action"]').element as HTMLButtonElement
    const openSecond = wrapper.get('[data-testid="open-second"]').element as HTMLButtonElement
    expect(document.activeElement).toBe(firstAction)

    openSecond.focus()
    await wrapper.get('[data-testid="open-second"]').trigger('click')
    await nextTick()
    await flushPromises()

    const secondFirst = wrapper.get('[data-testid="second-first"]').element as HTMLButtonElement
    const secondLast = wrapper.get('[data-testid="second-last"]').element as HTMLButtonElement
    expect(document.activeElement).toBe(secondFirst)

    secondLast.focus()
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true }))
    expect(document.activeElement).toBe(secondFirst)

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await nextTick()
    expect(wrapper.vm.closeSecond).toHaveBeenCalledOnce()
    expect(wrapper.vm.closeFirst).not.toHaveBeenCalled()
    expect(document.body.classList.contains('modal-open')).toBe(true)
    expect(appRoot.getAttribute('aria-hidden')).toBe('true')
    expect(appRoot.inert).toBe(true)
    expect(document.activeElement).toBe(openSecond)

    wrapper.unmount()
    expect(document.body.classList.contains('modal-open')).toBe(false)
    expect(appRoot.inert).toBe(false)
  })
})
