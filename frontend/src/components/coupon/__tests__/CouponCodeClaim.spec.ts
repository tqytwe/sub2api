import { mount, flushPromises } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import CouponCodeClaim from '../CouponCodeClaim.vue'
import { useAuthStore } from '@/stores/auth'
const claimCode = vi.hoisted(() => vi.fn())
vi.mock('@/stores/auth', async () => {
  const { reactive } = await import('vue')
  const state = reactive({ isAuthenticated: true, user: { id: 9 }, token: 'fixture' })
  return { useAuthStore: () => state }
})
vi.mock('@/api/coupon', () => ({ default: { claimCode } }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
describe('CouponCodeClaim', () => {
  it('ignores a reply from the previous authenticated account', async () => {
    let finish!: (value: unknown) => void
    claimCode.mockReturnValue(
      new Promise((resolve) => {
        finish = resolve
      }),
    )
    const w = mount(CouponCodeClaim)
    await w.get('input').setValue('SAVE-TEST')
    await w.get('form').trigger('submit')
    useAuthStore().user!.id = 10
    await flushPromises()
    finish({ data: { id: 42, status: 'available' } })
    await flushPromises()
    expect(w.emitted('claimed')).toBeUndefined()
    expect((w.get('input').element as HTMLInputElement).value).toBe('')
  })
  it('claims once and delivers the owned coupon for checkout', async () => {
    claimCode.mockResolvedValue({ data: { id: 42, status: 'available' } })
    const w = mount(CouponCodeClaim)
    await w.get('input').setValue(' save-test ')
    await w.get('form').trigger('submit')
    await flushPromises()
    expect(claimCode).toHaveBeenCalledWith('SAVE-TEST')
    expect(w.emitted('claimed')?.[0]).toEqual([{ id: 42, status: 'available' }])
  })
  it('retains input and explains an exhausted code', async () => {
    claimCode.mockRejectedValue({ code: 'COUPON_CODE_EXHAUSTED' })
    const w = mount(CouponCodeClaim)
    await w.get('input').setValue('SAVE-TEST')
    await w.get('form').trigger('submit')
    await flushPromises()
    expect(w.text()).toContain('couponCampaign.errors.COUPON_CODE_EXHAUSTED')
    expect((w.get('input').element as HTMLInputElement).value).toBe('SAVE-TEST')
  })
})
