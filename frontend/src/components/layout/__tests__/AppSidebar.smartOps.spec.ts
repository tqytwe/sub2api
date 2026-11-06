import { mount, flushPromises, enableAutoUnmount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import { createI18n } from 'vue-i18n'
import { createMemoryHistory, createRouter } from 'vue-router'
import AppSidebar from '../AppSidebar.vue'

const state = vi.hoisted(() => ({ app: {} as Record<string, unknown>, auth: {} as Record<string, unknown> }))
vi.mock('@/stores', () => ({ useAppStore: () => state.app, useAuthStore: () => state.auth, useAdminSettingsStore: () => ({ customMenuItems: [], fetch: vi.fn() }), useOnboardingStore: () => ({ isCurrentStep: () => false }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => state.app }))
vi.mock('@/composables/useBatchImageAccess', () => ({ useBatchImageAccess: () => ({ canUseBatchImage: { value: false }, refreshBatchImageAccess: vi.fn() }) }))
vi.mock('@/composables/useRoutePrefetch', () => ({ useRoutePrefetch: () => ({ prefetchRoute: vi.fn() }) }))
const paths = ['/admin/dashboard', '/admin/account-quality', '/admin/priority-scheduling', '/admin/token-guard-v2', '/admin/pelican-tests']
enableAutoUnmount(afterEach)
beforeEach(() => {
  state.app = reactive({ sidebarCollapsed: false, mobileOpen: true, cachedPublicSettings: {}, siteName: 'fixture', sidebarScrollTop: 0, toggleSidebar: () => { state.app.sidebarCollapsed = !state.app.sidebarCollapsed }, setMobileOpen: (open: boolean) => { state.app.mobileOpen = open } })
  state.auth = reactive({ isAdmin: true, isSimpleMode: false })
})
async function createView(path = paths[0]) {
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/:pathMatch(.*)*', component: { template: '<div />' } }] })
  await router.push(path); await router.isReady()
  const view = mount(AppSidebar, { global: { plugins: [router, createI18n({ legacy: false, locale: 'zh', missingWarn: false, fallbackWarn: false, messages: { zh: {} } })], stubs: { VersionBadge: true } } })
  await flushPromises()
  return { view, router }
}
describe('smart operations navigation', () => {
  it('groups existing routes without navigating to the group path', async () => {
    const { view, router } = await createView()
    const group = view.findAll('button').find(b => b.text().includes('nav.smartOps'))!
    expect(group.attributes('aria-expanded')).toBe('false')
    await group.trigger('click')
    expect(router.currentRoute.value.path).toBe('/admin/dashboard')
    for (const path of paths.slice(1)) expect(view.find(`a[href="${path}"]`).exists()).toBe(true)
    expect(group.attributes('aria-expanded')).toBe('true')
  })
  it('opens an icon-only sidebar so grouped functions remain reachable', async () => {
    state.app.sidebarCollapsed = true
    const { view } = await createView()
    await view.findAll('button').find(b => b.attributes('title') === 'nav.smartOps')!.trigger('click')
    expect(state.app.sidebarCollapsed).toBe(false)
    expect(view.find('a[href="/admin/account-quality"]').exists()).toBe(true)
  })
  it('expands on direct routes and preserves the simple-mode credential entry', async () => {
    state.auth.isSimpleMode = true
    const { view } = await createView('/admin/token-guard-v2')
    expect(view.get('a[href="/admin/token-guard-v2"]').classes()).toContain('sidebar-link-active')
    await view.findAll('button').find(b => b.text().includes('nav.smartOps'))!.trigger('click')
    expect(view.find('a[href="/admin/token-guard-v2"]').exists()).toBe(false)
  })
  it('closes mobile navigation after selecting a child and handles browser back', async () => {
    const { view, router } = await createView()
    await view.findAll('button').find(b => b.text().includes('nav.smartOps'))!.trigger('click')
    await view.get('a[href="/admin/account-quality"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/admin/account-quality')
    await vi.waitFor(() => expect(state.app.mobileOpen).toBe(false))
    await router.push('/admin/pelican-tests')
    expect(view.get('a[href="/admin/pelican-tests"]').classes()).toContain('sidebar-link-active')
    router.back()
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/admin/account-quality')
    expect(view.get('a[href="/admin/account-quality"]').classes()).toContain('sidebar-link-active')
  })
  it('does not show admin operations to ordinary users', async () => {
    state.auth.isAdmin = false
    const { view } = await createView('/dashboard')
    expect(view.text()).not.toContain('nav.smartOps')
    for (const path of paths.slice(1)) expect(view.find(`a[href="${path}"]`).exists()).toBe(false)
  })
})
