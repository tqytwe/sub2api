// Live contract test: started by the Go integration test against its disposable
// PostgreSQL-backed HTTP server. Never substitute a hand-authored response here.
import { enableAutoUnmount, mount } from '@vue/test-utils'
import { afterAll, afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createI18n } from 'vue-i18n'
import { createPinia, setActivePinia } from 'pinia'
import { useAuthStore } from '@/stores/auth'
import CheckInView from '@/views/user/CheckInView.vue'
import PlayHubView from '@/views/user/PlayHubView.vue'
import playAPI from '@/api/play'
import apiClient from '@/api/client'
import zh from '@/i18n/locales/zh'

// Match the production Vite JIT flag before importing the runtime i18n build.
vi.hoisted(() => vi.stubGlobal('__INTLIFY_JIT_COMPILATION__', true))
afterAll(() => vi.unstubAllGlobals())

const { showSuccess, showError, showInfo } = vi.hoisted(() => ({
  showSuccess: vi.fn(), showError: vi.fn(), showInfo: vi.fn(),
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess, showError, showInfo }) }))
vi.mock('vue-router', () => ({ useRouter: () => ({ push: vi.fn() }) }))
vi.mock('@/utils/growthAnalytics', () => ({ trackQuestCompleteOnce: vi.fn(), trackGrowthEvent: vi.fn() }))
vi.mock('@/utils/featureFlags', async importOriginal => {
  const actual = await importOriginal<typeof import('@/utils/featureFlags')>()
  return { ...actual, isFeatureFlagEnabled: (flag: unknown) => flag === actual.FeatureFlags.playCheckin }
})

setActivePinia(createPinia())
const authStore = useAuthStore()
const refreshUser = vi.spyOn(authStore, 'refreshUser')
beforeEach(() => { authStore.token = 'fixture-only'; authStore.user = null })
enableAutoUnmount(afterEach)

const baseURL = process.env.CHECKIN_CONTRACT_BASE_URL
// Default frontend runs explicitly skip this test. The dedicated PR gate runs
// it through scripts/test-checkin-contract.sh, which cannot skip the database.
describe.skipIf(!baseURL)('CheckInView with real check-in HTTP and PostgreSQL', () => {
  it('recovers 503 and permission failures, then reads actual disabled and reopened service state without grants', async () => {
    apiClient.defaults.baseURL = baseURL
    apiClient.defaults.adapter = 'http'
    apiClient.defaults.headers.common['X-Checkin-Test-User'] = process.env.CHECKIN_CONTRACT_READONLY
    await authStore.refreshUser()
    const i18n = createI18n({ legacy: false, locale: 'zh', messages: { zh } })
    try {
      for (const hub of [false, true]) {
        apiClient.defaults.headers.common['X-Checkin-Test-Fault'] = '503'
        const wrapper = mount(hub ? PlayHubView : CheckInView, {
          global: { plugins: [i18n], stubs: { AppLayout: { template: '<div><slot /></div>' }, Icon: true, RouterLink: { template: '<a><slot /></a>' } } },
        })
        await vi.waitFor(() => expect(wrapper.find('[role="alert"]').exists()).toBe(true))
        expect(wrapper.get('[role="alert"]').text()).toContain(i18n.global.t(`${hub ? 'playHub' : 'checkin'}.loadFailed`))
        apiClient.defaults.headers.common['X-Checkin-Test-Fault'] = '403'
        await wrapper.get('[role="alert"] button').trigger('click')
        await vi.waitFor(() => expect(wrapper.get('[role="alert"]').text()).toContain(i18n.global.t(`${hub ? 'playHub' : 'checkin'}.loadForbidden`)))
        delete apiClient.defaults.headers.common['X-Checkin-Test-Fault']
        apiClient.defaults.baseURL = process.env.CHECKIN_CONTRACT_DISABLED_URL
        await wrapper.get('[role="alert"] button').trigger('click')
        await vi.waitFor(() => expect(wrapper.text()).toContain(i18n.global.t(hub ? 'playHub.empty' : 'checkin.disabled')))
        apiClient.defaults.baseURL = baseURL
        await wrapper.get('button[data-testid="refresh-status"]').trigger('click')
        await vi.waitFor(() => expect(wrapper.text()).toContain(i18n.global.t(hub ? 'playHub.actionCheckin' : 'checkin.button')))
        wrapper.unmount()
      }
      expect((await playAPI.getPlayHub()).growth.balance).toBe(1)
      expect((await playAPI.getCheckinStatus()).checked_in_today).toBe(false)
    } finally {
      delete apiClient.defaults.headers.common['X-Checkin-Test-Fault']
      apiClient.defaults.baseURL = baseURL
    }
  }, 15000)

  for (const mode of ['EXPLORER', 'CASH', 'MILESTONE'] as const) {
    it(`renders the persisted ${mode.toLowerCase()} grant and reloads checked-in state`, async () => {
      expect(baseURL).toMatch(/^http:\/\/127\.0\.0\.1:\d+\/api\/v1$/)
      const userID = process.env[`CHECKIN_CONTRACT_${mode}`]
      expect(userID).toMatch(/^\d+$/)
      apiClient.defaults.baseURL = baseURL
      // Use Axios's Node HTTP transport rather than jsdom's cross-origin XHR.
      apiClient.defaults.adapter = 'http'
      apiClient.defaults.headers.common['X-Checkin-Test-User'] = userID
      vi.clearAllMocks()
      await authStore.refreshUser()
      const i18n = createI18n({ legacy: false, locale: 'zh', messages: { zh } })
      const mountView = () => mount(CheckInView, {
        global: { plugins: [i18n], stubs: { AppLayout: { template: '<div><slot /></div>' } } },
      })
      const wrapper = mountView()
      await vi.waitFor(() => expect(wrapper.find('button.gw-btn-primary').exists()).toBe(true))
      const before = await playAPI.getPlayHub()
      const beforeStatus = await playAPI.getCheckinStatus()
      expect(before.checkin?.checked_in_today).toBe(beforeStatus.checked_in_today)
      expect(before.checkin?.streak_count ?? 0).toBe(beforeStatus.streak_count ?? 0)
      expect(before.pending_actions).toBe(1)
      if (mode === 'MILESTONE') {
        expect(beforeStatus.next_milestone_days).toBe(7)
        expect(beforeStatus.streak_count).toBe(6)
        expect(wrapper.text()).toContain('再签 1 天可额外获得 $1.00')
      }
      // A second tab keeps its pre-grant status while the first commits a reward.
      const staleTab = mountView()
      await vi.waitFor(() => expect(staleTab.find('button.gw-btn-primary').exists()).toBe(true))
      refreshUser.mockClear()
      expect(wrapper.get('button.gw-btn-primary').attributes('disabled')).toBeUndefined()
      await wrapper.get('button.gw-btn-primary').trigger('click')
      const expected = mode === 'EXPLORER'
        ? i18n.global.t('checkin.energySuccess', { amount: 1 })
        : mode === 'MILESTONE'
          ? `${i18n.global.t('checkin.success', { amount: '1.50' })} · ${i18n.global.t('checkin.streak', { days: 7 })} · ${i18n.global.t('checkin.milestoneBonus', { amount: '1.00' })}`
          : i18n.global.t('checkin.success', { amount: '0.50' })
      expect(expected).toMatch(/[\u4e00-\u9fff]/)
      await vi.waitFor(() => expect(showSuccess).toHaveBeenCalledWith(expected))
      expect(showSuccess).toHaveBeenCalledOnce()
      expect(showSuccess).not.toHaveBeenCalledWith(i18n.global.t('checkin.success', { amount: '0.00' }))
      await vi.waitFor(() => expect(wrapper.text()).toContain(i18n.global.t('checkin.alreadyDone')))
      expect(refreshUser).toHaveBeenCalledOnce()
      expect(showError).not.toHaveBeenCalled()
      wrapper.unmount()

      await staleTab.get('button.gw-btn-primary').trigger('click')
      await vi.waitFor(() => expect(showInfo).toHaveBeenCalledWith(i18n.global.t('checkin.alreadyDone')))
      await vi.waitFor(() => expect(staleTab.get('button.gw-btn-primary').attributes('disabled')).toBeDefined())
      expect(showSuccess).toHaveBeenCalledOnce()
      expect(showError).not.toHaveBeenCalled()
      staleTab.unmount()

      // A fresh component must read the committed state through the real API.
      const reloaded = mountView()
      await vi.waitFor(() => expect(reloaded.text()).toContain(i18n.global.t('checkin.alreadyDone')))
      expect(reloaded.get('button.gw-btn-primary').attributes('disabled')).toBeDefined()
      await expect(playAPI.checkin()).rejects.toMatchObject({
        status: 409, reason: 'PLAY_CHECKIN_ALREADY_DONE',
      })
      expect((await playAPI.getCheckinStatus()).checked_in_today).toBe(true)
      const after = await playAPI.getPlayHub()
      expect(after.checkin?.checked_in_today).toBe(true)
      const committedStatus = await playAPI.getCheckinStatus()
      expect(after.checkin?.streak_count ?? 0).toBe(committedStatus.streak_count ?? 0)
      expect(after.checkin?.streak_count ?? 0).toBe(mode === 'EXPLORER' ? 0 : mode === 'MILESTONE' ? 7 : 1)
      expect(after.pending_actions).toBe(0)
      expect(after.growth.balance).toBe(mode === 'EXPLORER' ? 1 : mode === 'MILESTONE' ? 2.5 : 1.5)
      // Equivalent to a separate tab: this store keeps the pre-grant balance.
      authStore.user!.balance = 1
      await reloaded.get('button[data-testid="refresh-status"]').trigger('click')
      await vi.waitFor(() => expect(reloaded.get('.gw-balance-value').text()).toBe(`$${after.growth.balance.toFixed(2)}`))
      // Leaving check-in and opening the hub must read the committed PG state.
      const hubView = mount(PlayHubView, {
        global: { plugins: [i18n], stubs: { AppLayout: { template: '<div><slot /></div>' }, Icon: true, RouterLink: { template: '<a><slot /></a>' } } },
      })
      await vi.waitFor(() => expect(hubView.text()).toContain(i18n.global.t('playHub.checkinDone', { streak: after.checkin?.streak_count || 0 })))
      expect(hubView.text()).toContain(`$${after.growth.balance.toFixed(2)}`)
      expect(hubView.text()).not.toContain(i18n.global.t('playHub.actionCheckin'))
    }, 15000)
  }

  it('renders a persisted makeup milestone and refreshes a stale second tab after rejection', async () => {
    const userID = process.env.CHECKIN_CONTRACT_MAKEUP
    expect(userID).toMatch(/^\d+$/)
    apiClient.defaults.baseURL = baseURL
    apiClient.defaults.adapter = 'http'
    apiClient.defaults.headers.common['X-Checkin-Test-User'] = userID
    vi.clearAllMocks()
    await authStore.refreshUser()
    const i18n = createI18n({ legacy: false, locale: 'zh', messages: { zh } })
    const mountView = () => mount(CheckInView, {
      global: { plugins: [i18n], stubs: { AppLayout: { template: '<div><slot /></div>' } } },
    })
    const first = mountView()
    const stale = mountView()
    await vi.waitFor(() => expect(first.findAll('button.gw-btn-primary')).toHaveLength(2))
    await vi.waitFor(() => expect(stale.findAll('button.gw-btn-primary')).toHaveLength(2))
    refreshUser.mockClear()
    await first.findAll('button.gw-btn-primary')[1].trigger('click')
    await vi.waitFor(() => expect(showSuccess).toHaveBeenCalledWith(i18n.global.t('checkin.makeupSuccess', { amount: '1.50' })))
    await vi.waitFor(() => expect(first.findAll('button.gw-btn-primary')).toHaveLength(1))
    await stale.findAll('button.gw-btn-primary')[1].trigger('click')
    await vi.waitFor(() => expect(showError).toHaveBeenCalledWith(i18n.global.t('checkin.makeupUnavailable')))
    await vi.waitFor(() => expect(stale.findAll('button.gw-btn-primary')).toHaveLength(1))
    expect(showSuccess).toHaveBeenCalledOnce()
    expect(refreshUser).toHaveBeenCalledOnce()
    const reloaded = mountView()
    await vi.waitFor(() => expect(reloaded.findAll('button.gw-btn-primary')).toHaveLength(1))
    expect((await playAPI.getCheckinStatus()).can_makeup).toBeFalsy()
  }, 15000)
})
