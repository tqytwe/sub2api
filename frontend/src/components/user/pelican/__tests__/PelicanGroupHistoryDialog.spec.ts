import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import PelicanGroupHistoryDialog from '../PelicanGroupHistoryDialog.vue'

const { getShowcaseMock, getShowcaseItemMock } = vi.hoisted(() => ({
  getShowcaseMock: vi.fn(),
  getShowcaseItemMock: vi.fn(),
}))

vi.mock('@/api/pelicanShowcase', () => ({
  getShowcase: getShowcaseMock,
  getShowcaseItem: getShowcaseItemMock,
}))

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key, locale: { value: 'zh-CN' } }) }))

describe('PelicanGroupHistoryDialog', () => {
  beforeEach(() => {
    getShowcaseMock.mockReset().mockResolvedValue({
      enabled: true,
      groups: [{
        id: 7,
        name: 'OpenAI Pro',
        platform: 'openai',
        items: [
          { id: 21, group_id: 7, model_id: 'gpt-6-astra', reasoning_effort: 'high', latency_ms: 12466, generated_at: '2026-11-03T10:17:57Z' },
          { id: 20, group_id: 7, model_id: 'gpt-6-astra', reasoning_effort: 'high', latency_ms: 10519, generated_at: '2026-11-03T10:03:18Z' },
          { id: 19, group_id: 7, model_id: 'gpt-5', reasoning_effort: 'medium', latency_ms: 9400, generated_at: '2026-11-03T09:48:58Z' },
        ],
      }],
    })
    getShowcaseItemMock.mockResolvedValue({
      id: 21,
      group_id: 7,
      // design-governance-allow: inline-svg - model artwork fixture exercises the isolated preview path.
      response_text: '<!doctype html><html><body><svg><text>work</text></svg></body></html>',
    })
  })

  it('loads only the selected group history and lazy-loads the latest artwork into a sandbox', async () => {
    const wrapper = mount(PelicanGroupHistoryDialog, {
      props: { show: true, groupId: 7, groupName: 'OpenAI Pro' },
      global: {
        stubs: {
          BaseDialog: { props: ['show', 'width', 'contentClass', 'bodyClass'], template: '<div v-if="show"><section data-testid="dialog-content" :class="contentClass"><main data-testid="dialog-body" :class="bodyClass"><slot /></main></section></div>' },
          Icon: true,
          PelicanArtworkPreview: { props: ['html'], template: '<div data-testid="artwork-preview">{{ html }}</div>' },
        },
      },
    })
    await flushPromises()

    expect(getShowcaseMock).toHaveBeenCalledWith({ groupId: 7 })
    expect(wrapper.get('[data-testid="dialog-content"]').classes()).toContain('h-[90dvh]')
    expect(wrapper.get('[data-testid="dialog-body"]').classes()).toContain('min-h-0')
    expect(wrapper.text()).toContain('gpt-6-astra')
    expect(wrapper.text()).toContain('12.47 s')
    expect(getShowcaseItemMock).toHaveBeenCalledTimes(1)
    expect(getShowcaseItemMock).toHaveBeenCalledWith(21)

    await wrapper.find('[data-result-id="20"]').trigger('click')
    await flushPromises()

    expect(getShowcaseItemMock).toHaveBeenCalledWith(20)
    // design-governance-allow: inline-svg - assertion checks model artwork inside the isolated preview.
    expect(wrapper.find('[data-testid="artwork-preview"]').text()).toContain('<svg>')
  })

  it('does not request or render another group’s results', async () => {
    const wrapper = mount(PelicanGroupHistoryDialog, {
      props: { show: true, groupId: 8, groupName: 'Anthropic' },
      global: { stubs: { BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' }, Icon: true } },
    })
    await flushPromises()

    expect(getShowcaseMock).toHaveBeenCalledWith({ groupId: 8 })
    expect(wrapper.find('[data-result-id="21"]').exists()).toBe(false)
  })
})
