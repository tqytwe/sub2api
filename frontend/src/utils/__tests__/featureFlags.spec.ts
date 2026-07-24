import { afterEach, describe, expect, it, vi } from 'vitest'

import type { PublicSettings } from '@/types'

const appStore = vi.hoisted(() => ({
  cachedPublicSettings: null as PublicSettings | null | undefined,
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStore,
}))

import { FeatureFlags, isFeatureFlagEnabled } from '../featureFlags'

describe('marketplace feature flag', () => {
  afterEach(() => {
    appStore.cachedPublicSettings = null
  })

  it('registers marketplace as opt-in and defaults hidden', () => {
    expect(FeatureFlags.marketplace).toMatchObject({
      key: 'marketplace_enabled',
      mode: 'opt-in',
      label: 'AI Model Marketplace',
    })

    appStore.cachedPublicSettings = null
    expect(isFeatureFlagEnabled(FeatureFlags.marketplace)).toBe(false)

    appStore.cachedPublicSettings = undefined
    expect(isFeatureFlagEnabled(FeatureFlags.marketplace)).toBe(false)

    appStore.cachedPublicSettings = {} as PublicSettings
    expect(isFeatureFlagEnabled(FeatureFlags.marketplace)).toBe(false)

    appStore.cachedPublicSettings = {
      marketplace_enabled: false,
    } as PublicSettings
    expect(isFeatureFlagEnabled(FeatureFlags.marketplace)).toBe(false)

    appStore.cachedPublicSettings = {
      marketplace_enabled: true,
    } as PublicSettings
    expect(isFeatureFlagEnabled(FeatureFlags.marketplace)).toBe(true)
  })
})
