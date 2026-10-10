// Live contract test: started by the Go integration test against its disposable
// PostgreSQL-backed HTTP server. Never substitute a hand-authored response here.
import { enableAutoUnmount, mount } from '@vue/test-utils'
import { afterAll, afterEach, describe, expect, it, vi } from 'vitest'
import { createI18n } from 'vue-i18n'
import CheckInView from '@/views/user/CheckInView.vue'
import playAPI from '@/api/play'
import apiClient from '@/api/client'
import zh from '@/i18n/locales/zh'

// Match the production Vite JIT flag before importing the runtime i18n build.
vi.hoisted(() => vi.stubGlobal('__INTLIFY_JIT_COMPILATION__', true))
afterAll(() => vi.unstubAllGlobals())

const { showSuccess, showError, showInfo, refreshUser } = vi.hoisted(() => ({
  showSuccess: vi.fn(), showError: vi.fn(), showInfo: vi.fn(), refreshUser: vi.fn(),
}))
vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ user: { balance: 1 }, refreshUser }),
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess, showError, showInfo }) }))
vi.mock('vue-router', () => ({ useRouter: () => ({ push: vi.fn() }) }))
vi.mock('@/utils/growthAnalytics', () => ({ trackQuestCompleteOnce: vi.fn() }))

enableAutoUnmount(afterEach)

const baseURL = process.env.CHECKIN_CONTRACT_BASE_URL
// Default frontend runs explicitly skip this test. The dedicated PR gate runs
// it through scripts/test-checkin-contract.sh, which cannot skip the database.
describe.skipIf(!baseURL)('CheckInView with real check-in HTTP and PostgreSQL', () => {
  for (const mode of ['EXPLORER', 'CASH'] as const) {
    it(`renders the persisted ${mode.toLowerCase()} grant and reloads checked-in state`, async () => {
      expect(baseURL).toMatch(/^http:\/\/127\.0\.0\.1:\d+\/api\/v1$/)
      const userID = process.env[`CHECKIN_CONTRACT_${mode}`]
      expect(userID).toMatch(/^\d+$/)
      apiClient.defaults.baseURL = baseURL
      // Use Axios's Node HTTP transport rather than jsdom's cross-origin XHR.
      apiClient.defaults.adapter = 'http'
      apiClient.defaults.headers.common['X-Checkin-Test-User'] = userID
      vi.clearAllMocks()
      refreshUser.mockResolvedValue(undefined)
      const i18n = createI18n({ legacy: false, locale: 'zh', messages: { zh } })
      const mountView = () => mount(CheckInView, {
        global: { plugins: [i18n], stubs: { AppLayout: { template: '<div><slot /></div>' } } },
      })
      const wrapper = mountView()
      await vi.waitFor(() => expect(wrapper.find('button.gw-btn-primary').exists()).toBe(true))
      expect(wrapper.get('button.gw-btn-primary').attributes('disabled')).toBeUndefined()
      await wrapper.get('button.gw-btn-primary').trigger('click')
      const expected = mode === 'EXPLORER'
        ? i18n.global.t('checkin.energySuccess', { amount: 1 })
        : i18n.global.t('checkin.success', { amount: '0.50' })
      expect(expected).toMatch(/[\u4e00-\u9fff]/)
      await vi.waitFor(() => expect(showSuccess).toHaveBeenCalledWith(expected))
      expect(showSuccess).toHaveBeenCalledOnce()
      expect(showSuccess).not.toHaveBeenCalledWith(i18n.global.t('checkin.success', { amount: '0.00' }))
      await vi.waitFor(() => expect(wrapper.text()).toContain(i18n.global.t('checkin.alreadyDone')))
      expect(refreshUser).toHaveBeenCalledOnce()
      expect(showError).not.toHaveBeenCalled()
      wrapper.unmount()

      // A fresh component must read the committed state through the real API.
      const reloaded = mountView()
      await vi.waitFor(() => expect(reloaded.text()).toContain(i18n.global.t('checkin.alreadyDone')))
      expect(reloaded.get('button.gw-btn-primary').attributes('disabled')).toBeDefined()
      await expect(playAPI.checkin()).rejects.toMatchObject({
        status: 409, reason: 'PLAY_CHECKIN_ALREADY_DONE',
      })
      expect((await playAPI.getCheckinStatus()).checked_in_today).toBe(true)
    }, 15000)
  }
})
