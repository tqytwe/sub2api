import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import AdminAffiliateOverviewView from '../AdminAffiliateOverviewView.vue'

const api = vi.hoisted(() => ({ getOverview: vi.fn(), getLeaderboard: vi.fn(), listExceptions: vi.fn() }))
vi.mock('@/api/admin/affiliateOperations', () => ({ default: api }))
vi.mock('vue-i18n', async importOriginal => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key, locale: { value: 'en' } }),
}))

async function render(selectedMonth = '2026-10') {
  const router = createRouter({ history: createMemoryHistory(), routes: [
    { path: '/admin/affiliates/overview', component: AdminAffiliateOverviewView },
    { path: '/admin/affiliates/invites', component: { template: '<div />' } },
  ] })
  await router.push(`/admin/affiliates/overview?month=${selectedMonth}`)
  await router.isReady()
  const wrapper = mount(AdminAffiliateOverviewView, { global: { plugins: [router], stubs: {
    AppLayout: { template: '<main><slot /></main>' },
    AffiliateTrendChart: true,
    DataTable: { props: ['columns', 'data'], template: '<div><div v-for="row in data" :key="row.user_id || row.invitee_id"><slot v-for="column in columns" :name="`cell-${column.key}`" :row="row">{{ row[column.key] }}</slot></div></div>' },
  } } })
  await flushPromises()
  return { wrapper, router }
}

describe('administrator affiliate overview', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    api.getOverview.mockResolvedValue({ month: '2026-10', currency: 'USD', monetary_scope: 'all_time', invited_count: 8, effective_paid_count: 3,
      pending_count: 1, conversion_rate: .375, available_quota: 2, frozen_quota: 4, gross_earned: 9, revoked_amount: 1, net_earned: 8,
      transferred_amount: 1, withdrawn_amount: 1, exceptions_count: 1, trend: [], updated_at: '2026-11-08T01:00:00Z' })
    api.getLeaderboard.mockResolvedValue({ month: '2026-10', timezone: 'Asia/Shanghai', items: [{ rank: 1, user_id: 17, email: 'owner@example.com', username: 'owner', effective_paid_count: 3 }], updated_at: '2026-11-08T01:00:00Z' })
    api.listExceptions.mockResolvedValue({ items: [{ inviter_id: 17, inviter_email: 'owner@example.com', invitee_id: 19, invitee_email: 'new@example.com', status: 'pending_evidence', reason: 'evidence_missing' }], total: 1 })
  })

  it('uses one selected cohort month and carries exact identity into the existing invitation page', async () => {
    const { wrapper } = await render()
    expect(api.getOverview).toHaveBeenCalledWith({ month: '2026-10' })
    expect(api.getLeaderboard).toHaveBeenCalledWith({ month: '2026-10' }, expect.any(AbortSignal))
    expect(wrapper.get('[data-test="ranking-user-17"]').attributes('href')).toContain('user_id=17')
    expect(wrapper.get('[data-test="exception-user-19"]').attributes('href')).toContain('invitee_id=19')
    expect(wrapper.text()).toContain('admin.affiliateOperations.allTimeFunds')
    expect(wrapper.text()).toContain('admin.affiliateOperations.cohortExplanation')
  })

  it('keeps a failed section explicit while other overview sections remain usable', async () => {
    api.getLeaderboard.mockRejectedValue(new Error('unavailable'))
    const { wrapper } = await render()
    expect(wrapper.get('[data-test="ranking-error"]').text()).toContain('common.retry')
    expect(wrapper.find('[data-test="exception-user-19"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="overview-stats"]').exists()).toBe(true)
  })

  it('distinguishes the complete outstanding total from the five-row preview', async () => {
    api.listExceptions.mockResolvedValue({
      items: Array.from({ length: 5 }, (_, index) => ({
        inviter_id: 17,
        inviter_email: 'owner@example.com',
        invitee_id: index + 19,
        invitee_email: `new-${index}@example.com`,
        status: 'pending_evidence',
        reason: 'evidence_missing',
      })),
      total: 404,
    })
    const { wrapper } = await render()

    const count = wrapper.get('[data-test="exceptions-count"]')
    expect(count.attributes('data-total')).toBe('404')
    expect(count.attributes('data-preview')).toBe('5')
  })

  it('keeps the 2026-08 administrator leaderboard identity complete for investigation', async () => {
    api.getLeaderboard.mockResolvedValue({ month: '2026-08', timezone: 'Asia/Shanghai', items: [{ rank: 1, user_id: 17, email: 'guolingzheng@gmail.com', username: 'guolingzheng', effective_paid_count: 1 }], updated_at: '2026-11-08T01:00:00Z' })
    const { wrapper } = await render('2026-08')

    expect(api.getOverview).toHaveBeenCalledWith({ month: '2026-08' })
    expect(api.getLeaderboard).toHaveBeenCalledWith({ month: '2026-08' }, expect.any(AbortSignal))
    expect(wrapper.get('[data-test="ranking-user-17"]').text()).toContain('guolingzheng@gmail.com')
  })
})
