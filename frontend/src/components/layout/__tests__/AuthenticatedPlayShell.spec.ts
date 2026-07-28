import { enableAutoUnmount, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useAuthStore } from '@/stores/auth'
import AuthenticatedPlayShell from '@/components/layout/AuthenticatedPlayShell.vue'

vi.mock('@/stores/auth', async () => {
  const { reactive } = await import('vue')
  const state = reactive({ isAuthenticated: false })
  return { useAuthStore: () => state }
})

const authState = useAuthStore() as unknown as { isAuthenticated: boolean }

enableAutoUnmount(afterEach)

function mountShell() {
  return mount(AuthenticatedPlayShell, {
    slots: { default: '<div data-testid="play-content">play content</div>' },
    global: {
      stubs: {
        AppLayout: { template: '<section data-testid="app-layout"><slot /></section>' },
      },
    },
  })
}

describe('AuthenticatedPlayShell', () => {
  beforeEach(() => {
    authState.isAuthenticated = false
  })

  it('keeps the existing public page shell for guests', () => {
    const wrapper = mountShell()

    expect(wrapper.get('[data-testid="play-content"]').text()).toBe('play content')
    expect(wrapper.find('[data-testid="app-layout"]').exists()).toBe(false)
  })

  it('uses the application layout for signed-in players', () => {
    authState.isAuthenticated = true
    const wrapper = mountShell()

    expect(wrapper.get('[data-testid="app-layout"]').text()).toContain('play content')
  })
})
