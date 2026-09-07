import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import AnnouncementTargetingEditor from '../AnnouncementTargetingEditor.vue'
import type { AnnouncementTargeting } from '@/types'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

const SelectStub = {
  props: ['modelValue', 'options'],
  emits: ['update:modelValue'],
  template: `
    <select :value="modelValue" @change="$emit('update:modelValue', $event.target.value)">
      <option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option>
    </select>
  `,
}

function mountEditor(modelValue: AnnouncementTargeting) {
  return mount(AnnouncementTargetingEditor, {
    props: { modelValue, groups: [] },
    global: {
      stubs: {
        Select: SelectStub,
        GroupSelector: true,
        Icon: true,
      },
      mocks: {
        $t: (key: string) => key,
      },
    },
  })
}

describe('AnnouncementTargetingEditor Play membership targeting', () => {
  it('turns a condition into an ordinary-user membership condition with a fixed in operator', async () => {
    const wrapper = mountEditor({
      any_of: [{ all_of: [{ type: 'subscription', operator: 'in', group_ids: [42] }] }],
    })

    await wrapper.find('select').setValue('play_membership')
    const emitted = wrapper.emitted('update:modelValue')?.at(-1)?.[0] as AnnouncementTargeting

    expect(emitted).toEqual({
      any_of: [{ all_of: [{ type: 'play_membership', operator: 'in', play_membership: 'ordinary' }] }],
    })
    await wrapper.setProps({ modelValue: emitted })
    expect(wrapper.findAll('select')).toHaveLength(2)
    expect(wrapper.findAll('select')[1].text()).toContain('admin.announcements.form.playMembershipMember')
  })

  it('allows only the member segment after membership targeting is selected', async () => {
    const wrapper = mountEditor({
      any_of: [{ all_of: [{ type: 'play_membership', operator: 'in', play_membership: 'ordinary' }] }],
    })

    const selects = wrapper.findAll('select')
    expect(selects).toHaveLength(2)
    await selects[1].setValue('member')
    const emitted = wrapper.emitted('update:modelValue')?.at(-1)?.[0] as AnnouncementTargeting

    expect(emitted.any_of[0].all_of[0]).toEqual({
      type: 'play_membership',
      operator: 'in',
      play_membership: 'member',
    })
  })

  it('keeps existing balance and subscription conditions intact', () => {
    const targeting: AnnouncementTargeting = {
      any_of: [{
        all_of: [
          { type: 'balance', operator: 'gte', value: 50 },
          { type: 'subscription', operator: 'in', group_ids: [12] },
        ],
      }],
    }
    const wrapper = mountEditor(targeting)

    expect(wrapper.props('modelValue')).toEqual(targeting)
    expect(wrapper.findAll('select')).toHaveLength(3)
  })
})
