import { mount, flushPromises } from '@vue/test-utils'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import CouponCampaignPanel from '../CouponCampaignPanel.vue'
const api = vi.hoisted(() => ({
  getClaimPolicy: vi.fn(),
  previewCampaign: vi.fn(),
  executeCampaign: vi.fn(),
}))
vi.mock('@/api/admin/coupon', () => ({ adminCouponAPI: api }))
vi.mock('@/api/admin', () => ({ adminAPI: {} }))
vi.mock('vue-i18n', async () => ({
  ...(await vi.importActual('vue-i18n')),
  useI18n: () => ({ t: (key: string) => key }),
}))
const preview = {
  token: 'fixed-token',
  recipients: [9],
  skipped: [],
  requested_count: 1,
  remaining_stock: 10,
  expires_at: '2026-11-11T23:00:00Z',
}
function panel() {
  return mount(CouponCampaignPanel, {
    props: { templates: [{ id: 1, name: 'Test', status: 'active' }] as never },
    global: { stubs: { CouponAudienceForm: true } },
  })
}
describe('CouponCampaignPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.getClaimPolicy.mockResolvedValue({ data: { per_user_limit: 1 } })
    api.executeCampaign.mockResolvedValue({ data: { batch_id: 1, issued_count: 1, skipped: [] } })
  })
  it('does not restore an old preview after the administrator changes recipients', async () => {
    let finish!: (value: unknown) => void
    api.previewCampaign.mockReturnValue(
      new Promise((r) => {
        finish = r
      }),
    )
    const w = panel()
    await w.get('select').setValue('1')
    await flushPromises()
    await w.get('form').trigger('submit')
    w.getComponent({ name: 'CouponAudienceForm' }).vm.$emit('update:modelValue', {
      mode: 'users',
      user_ids: [10],
    })
    await flushPromises()
    finish({ data: preview })
    await flushPromises()
    expect(w.text()).not.toContain('couponCampaign.confirm')
  })
  it('confirms only the fixed preview token, then clears the confirmation', async () => {
    api.previewCampaign.mockResolvedValue({ data: preview })
    const w = panel()
    await w.get('select').setValue('1')
    await flushPromises()
    await w.get('form').trigger('submit')
    await flushPromises()
    const confirm = w.findAll('button').find((b) => b.text() === 'couponCampaign.confirm')!
    await confirm.trigger('click')
    await flushPromises()
    expect(api.executeCampaign).toHaveBeenCalledWith('fixed-token')
    expect(w.emitted('issued')).toHaveLength(1)
    expect(w.text()).not.toContain('couponCampaign.confirm')
  })
})
