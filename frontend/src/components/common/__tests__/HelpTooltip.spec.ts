import { afterEach, describe, expect, it } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'

enableAutoUnmount(afterEach)

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key === 'common.close' ? 'Close' : key }),
}))

function getTooltipElement(): HTMLDivElement {
  const tooltip = document.body.querySelector('[role="tooltip"]')
  if (!(tooltip instanceof HTMLDivElement)) {
    throw new Error('tooltip element not found')
  }
  return tooltip
}

describe('HelpTooltip', () => {
  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
    document.body.innerHTML = ''
  })

  function mountPositionedTooltip() {
    vi.stubGlobal('innerWidth', 1280)
    vi.stubGlobal('innerHeight', 900)
    const wrapper = mount(HelpTooltip, {
      attachTo: document.body,
      props: { trigger: 'click', content: 'Output TPS definition' },
      slots: { trigger: '<button type="button">Explain TPS</button>' },
    })
    const tooltip = getTooltipElement()
    const rect = { left: 345, top: 578, width: 20, height: 20 }
    vi.spyOn(wrapper.get('.group').element, 'getBoundingClientRect').mockImplementation(() => ({
      ...rect, right: rect.left + rect.width, bottom: rect.top + rect.height, x: rect.left, y: rect.top, toJSON: () => ({}),
    }))
    vi.spyOn(tooltip, 'getBoundingClientRect').mockReturnValue({
      left: 0, top: 0, right: 256, bottom: 120, x: 0, y: 0, width: 256, height: 120, toJSON: () => ({}),
    })
    return { wrapper, tooltip, rect }
  }

  it('uses viewport coordinates after document scroll, repositions on captured scroll and retains focus on Escape', async () => {
    vi.stubGlobal('scrollY', 870)
    vi.stubGlobal('scrollX', 93)
    const { wrapper, tooltip, rect } = mountPositionedTooltip()
    const button = wrapper.get('button')
    button.element.focus()
    await button.trigger('click')
    await nextTick()
    expect(tooltip.style.top).toBe('450px')
    expect(tooltip.style.left).toBe('227px')

    rect.top = 480
    wrapper.element.dispatchEvent(new Event('scroll'))
    await nextTick()
    expect(tooltip.style.top).toBe('352px')
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await nextTick()
    expect(tooltip.style.display).toBe('none')
    expect(document.activeElement).toBe(button.element)
  })

  it('keeps the tooltip inside the viewport near either horizontal edge and after resize', async () => {
    const { wrapper, tooltip, rect } = mountPositionedTooltip()
    rect.left = 0
    await wrapper.get('button').trigger('click')
    await nextTick()
    expect(tooltip.style.left).toBe('8px')
    rect.left = 1260
    window.dispatchEvent(new Event('scroll'))
    await nextTick()
    expect(tooltip.style.left).toBe('1016px')
    vi.stubGlobal('innerWidth', 800)
    rect.left = 780
    window.dispatchEvent(new Event('resize'))
    await nextTick()
    expect(tooltip.style.left).toBe('536px')
    expect(tooltip.style.display).not.toBe('none')
  })

  it('places the tooltip below a trigger near the top and above a trigger near the bottom', async () => {
    const { wrapper, tooltip, rect } = mountPositionedTooltip()
    rect.top = 10
    await wrapper.get('button').trigger('click')
    await nextTick()
    expect(tooltip.style.top).toBe('38px')
    expect(tooltip.classList.contains('before:bottom-full')).toBe(true)
    rect.top = 875
    window.dispatchEvent(new Event('scroll'))
    await nextTick()
    expect(tooltip.style.top).toBe('747px')
    expect(tooltip.classList.contains('before:top-full')).toBe(true)
  })

  it('keeps the existing hover interaction by default', async () => {
    const wrapper = mount(HelpTooltip, {
      attachTo: document.body,
      props: {
        content: 'hover details',
      },
    })

    const trigger = wrapper.get('.group')
    const tooltip = getTooltipElement()

    expect(tooltip.style.display).toBe('none')

    await trigger.trigger('mouseenter')
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')

    await trigger.trigger('mouseleave')
    await nextTick()
    expect(tooltip.style.display).toBe('none')

    wrapper.unmount()
  })

  it('keeps a hover tooltip open while the pointer moves between the trigger and the tooltip', async () => {
    const wrapper = mount(HelpTooltip, {
      attachTo: document.body,
      props: {
        content: 'copyable details',
      },
    })

    const trigger = wrapper.get('.group')
    const tooltip = getTooltipElement()

    await trigger.trigger('mouseenter')
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')

    await trigger.trigger('mouseleave', { relatedTarget: tooltip })
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')

    tooltip.dispatchEvent(new MouseEvent('mouseleave', { relatedTarget: trigger.element }))
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')

    tooltip.dispatchEvent(new MouseEvent('mouseleave', { relatedTarget: null }))
    await nextTick()
    expect(tooltip.style.display).toBe('none')

    wrapper.unmount()
  })

  it('supports click-to-toggle details and closes on outside click', async () => {
    const wrapper = mount(HelpTooltip, {
      attachTo: document.body,
      props: {
        content: 'click details',
        trigger: 'click',
      },
    })

    const trigger = wrapper.get('.group')
    const tooltip = getTooltipElement()

    expect(tooltip.style.display).toBe('none')

    await trigger.trigger('click')
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')
    expect(tooltip.textContent).toContain('click details')

    const closeButton = tooltip.querySelector('button[aria-label="Close"]')
    if (!(closeButton instanceof HTMLButtonElement)) {
      throw new Error('close button not found')
    }
    closeButton.click()
    await nextTick()
    expect(tooltip.style.display).toBe('none')

    await trigger.trigger('click')
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')

    document.body.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await nextTick()
    expect(tooltip.style.display).toBe('none')

    wrapper.unmount()
  })
})
