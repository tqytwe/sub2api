import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AdminAffiliateUserDialog from '../AdminAffiliateUserDialog.vue'

const api = vi.hoisted(() => ({ listPromoters: vi.fn(), listInvitees: vi.fn(), listEarnings: vi.fn(), listAudits: vi.fn() }))
const settings = vi.hoisted(() => ({ getUserOverview: vi.fn(), updateUserSettings: vi.fn(), clearUserSettings: vi.fn() }))
const run = vi.hoisted(() => vi.fn())
vi.mock('@/api/admin/affiliateOperations', () => ({ default: api }))
vi.mock('@/api/admin/affiliates', () => ({ affiliatesAPI: settings, default: settings }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: vi.fn(), showError: vi.fn() }) }))
vi.mock('vue-i18n', async original => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('vue-router', () => ({ RouterLink: { template: '<a><slot /></a>' } }))
vi.mock('@/composables/useStepUp', () => ({ useStepUp: () => ({ run }), isStepUpCancelled: () => false, isStepUpBlocked: () => false }))
function render() { return mount(AdminAffiliateUserDialog, { props: { show: true, userId: 17, inviteeId: 19 }, global: { stubs: {
  BaseDialog: { props: ['show'], template: '<section v-if="show"><slot /><slot name="footer" /></section>' },
  DataTable: true, Pagination: true, TotpStepUpDialog: true, ConfirmDialog: true,
} } }) }

describe('affiliate user investigation and settings', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    run.mockImplementation((fn: () => unknown) => fn())
    settings.getUserOverview.mockResolvedValue({ user_id: 17, aff_code: 'ABC', rebate_rate_percent: 20, available_quota: 2, invited_count: 3 })
    api.listPromoters.mockResolvedValue({ items: [{ user_id: 17, email: 'owner@example.com', available_quota: 2, frozen_quota: 3 }] })
    api.listInvitees.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20 })
    api.listAudits.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20 })
  })
  it('investigates the exact inviter and invitee and links to filtered accounting', async () => {
    const wrapper = render()
    await flushPromises()
    expect(api.listInvitees).toHaveBeenCalledWith(expect.objectContaining({ user_id: 17, invitee_id: 19 }))
    expect(wrapper.find('[data-test="detail-rebates-link"]').exists()).toBe(true)
  })
  it('validates the exclusive rate and saves through existing settings and step-up', async () => {
    const wrapper = render()
    await flushPromises()
    await wrapper.get('[data-test="detail-tab-settings"]').trigger('click')
    await wrapper.get('[data-test="detail-rate"]').setValue('101')
    await wrapper.get('form').trigger('submit')
    expect(settings.updateUserSettings).not.toHaveBeenCalled()
    await wrapper.get('[data-test="detail-rate"]').setValue('25')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(run).toHaveBeenCalled()
    expect(settings.updateUserSettings).toHaveBeenCalledWith(17, { aff_rebate_rate_percent: 25 })
  })

  it('keeps an inherited global rate inherited when saving an invitation code', async () => {
    const wrapper = render()
    await flushPromises()
    await wrapper.get('[data-test="detail-tab-settings"]').trigger('click')
    expect((wrapper.get('[data-test="detail-rate"]').element as HTMLInputElement).value).toBe('')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(settings.updateUserSettings).toHaveBeenCalledWith(17, { clear_rebate_rate: true })
  })
})
