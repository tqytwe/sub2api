import { describe, expect, it, vi, beforeEach } from 'vitest'
import { ref } from 'vue'

const isAuthenticated = ref(false)

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ isAuthenticated: isAuthenticated.value }),
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

describe('usePlayPageBackNav', () => {
  beforeEach(() => {
    isAuthenticated.value = false
  })

  it('routes guests back to home', async () => {
    const { usePlayPageBackNav } = await import('@/composables/usePlayPageBackNav')
    const { backTarget, backLabel } = usePlayPageBackNav()
    expect(backTarget.value).toBe('/home')
    expect(backLabel.value).toBe('play.backHome')
  })

  it('routes signed-in users back to play hub', async () => {
    isAuthenticated.value = true
    const { usePlayPageBackNav } = await import('@/composables/usePlayPageBackNav')
    const { backTarget, backLabel } = usePlayPageBackNav()
    expect(backTarget.value).toBe('/play')
    expect(backLabel.value).toBe('play.backPlayHub')
  })
})
