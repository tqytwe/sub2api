import { mount, flushPromises } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import CouponAudienceForm from '../CouponAudienceForm.vue'
vi.mock('@/api/admin', () => ({
  adminAPI: { users: { list: vi.fn() }, userAttributes: { listEnabledDefinitions: vi.fn() } },
}))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
describe('CouponAudienceForm', () => {
  it('invalid pasted IDs clear the previous valid audience instead of retaining it', async () => {
    const w = mount(CouponAudienceForm, { props: { modelValue: { mode: 'users', user_ids: [9] } } })
    expect((w.get('input').element as HTMLInputElement).value).toBe('9')
    await w.get('input').setValue('9,invalid')
    expect(w.emitted('update:modelValue')?.at(-1)).toEqual([{ mode: 'users', user_ids: [] }])
    expect(w.get('[role="alert"]').text()).toBe('couponCampaign.invalidIds')
  })
  it('preserves a reopened selected audience and clears old modes', async () => {
    const w = mount(CouponAudienceForm, { props: { modelValue: { mode: 'all' } } })
    await w.setProps({ modelValue: { mode: 'users', user_ids: [9, 10] } })
    await flushPromises()
    expect((w.get('input').element as HTMLInputElement).value).toBe('9, 10')
    await w.get('select').setValue('all')
    expect(w.emitted('update:modelValue')?.at(-1)).toEqual([{ mode: 'all' }])
  })
})
