import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { reactive } from 'vue'
import { createI18n } from 'vue-i18n'
import VersionBadge from '../VersionBadge.vue'
import zh from '@/i18n/locales/zh/legacy/core'
import en from '@/i18n/locales/en/legacy/core'

const { state, auth, api } = vi.hoisted(() => ({
  state: { currentVersion: '0.2.14', latestVersion: '2.10.3', upstreamBaseline: '2.10.3', hasUpdate: false, buildType: 'release', versionLoading: false, versionWarning: '', releaseInfo: { html_url: 'https://github.com/ranxi2001/sub2api/releases/tag/v2.10.3' }, fetchVersion: vi.fn() },
  auth: { isAdmin: true },
  api: { performUpdate: vi.fn(), restartService: vi.fn(), getRollbackVersions: vi.fn(), rollback: vi.fn() }
}))
vi.mock('@/stores', () => ({ useAppStore: () => reactive(state), useAuthStore: () => reactive(auth) }))
vi.mock('@/api/admin/system', () => api)
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copied: false, copyToClipboard: vi.fn() }) }))

function runtimeMessages(version: Record<string, string>) {
  return { version: Object.fromEntries(Object.entries(version).map(([key, text]) => [key, () => text])), common: { loading: () => 'Loading' } }
}

function mountBadge(locale = 'zh') {
  return mount(VersionBadge, { global: { plugins: [createI18n({ legacy: false, locale, messages: { zh: runtimeMessages(zh.version), en: runtimeMessages(en.version) } })] } })
}
beforeEach(() => { vi.clearAllMocks(); auth.isAdmin = true; state.hasUpdate = false; state.versionLoading = false; state.versionWarning = ''; state.releaseInfo.html_url = 'https://github.com/ranxi2001/sub2api/releases/tag/v2.10.3' })

describe('VersionBadge fork source policy', () => {
  it.each(['source', 'release'])('never offers upstream installation or rollback for %s builds', async (buildType) => {
    state.buildType = buildType
    state.hasUpdate = true
    const wrapper = mountBadge()
    await wrapper.find('button').trigger('click')
    expect(wrapper.text()).toContain('查看 ranxi 上游新版本')
    expect(wrapper.text()).toContain('安装已验证的 tqytwe 构建')
    expect(wrapper.text()).toContain('当前暂无已验证的在线安装产物')
    expect(wrapper.text()).toContain('在线回退及旧二进制备份恢复已停用')
    expect(wrapper.findAll('a').map(a => a.attributes('href'))).toEqual([
      'https://github.com/ranxi2001/sub2api/releases/tag/v2.10.3',
      'https://github.com/tqytwe/sub2api/blob/play/main/deploy/FORK_SOURCE_BUILD.md'
    ])
    expect(wrapper.text()).not.toMatch(/curl|weishaw\/|docker compose|立即更新/)
    for (const button of wrapper.findAll('button').slice(1)) await button.trigger('click')
    expect(state.fetchVersion).toHaveBeenCalledWith(true)
    Object.values(api).forEach(fn => expect(fn).not.toHaveBeenCalled())
    wrapper.unmount()
  })
  it('keeps source guidance with no newer release and rejects cached foreign links', async () => {
    state.releaseInfo.html_url = 'https://github.com/Wei-Shaw/sub2api/releases/tag/v9.9.9'
    const wrapper = mountBadge('en')
    await wrapper.find('button').trigger('click')
    expect(wrapper.text()).toContain('No verified artifact')
    expect(wrapper.find('a').attributes('href')).toBe('https://github.com/ranxi2001/sub2api/releases')
    expect(wrapper.text()).not.toContain('Already up to date')
    await wrapper.find('button').trigger('keydown.esc')
    expect(wrapper.find('a').exists()).toBe(false)
    wrapper.unmount()
  })
  it('disables refresh while loading and shows query warnings without install controls', async () => {
    state.versionLoading = true
    const wrapper = mountBadge()
    await wrapper.find('button').trigger('click')
    expect(wrapper.findAll('button')[1].attributes('disabled')).toBeDefined()
    wrapper.unmount()
    state.versionLoading = false; state.versionWarning = 'offline'
    const warningWrapper = mountBadge()
    await warningWrapper.find('button').trigger('click')
    expect(warningWrapper.find('[role="status"]').text()).toContain('查询失败')
    warningWrapper.unmount()
  })
  it('does not query or offer admin actions to regular users', async () => {
    auth.isAdmin = false
    const wrapper = mountBadge()
    await flushPromises()
    expect(wrapper.find('button').exists()).toBe(false)
    expect(state.fetchVersion).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})
