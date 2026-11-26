import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import { beforeEach, describe, expect, it } from 'vitest'
import NewFeatureBadge from '../NewFeatureBadge.vue'
import en from '@/i18n/locales/en/common'
import { NEW_FEATURE_NOTICES } from '@/constants/newFeatureNotices'
import { useNewFeatureNoticeStore } from '@/stores/newFeatureNotices'

describe('NewFeatureBadge', () => {
  beforeEach(() => {
    localStorage.clear()
    setActivePinia(createPinia())
  })

  const mountBadge = (userId = 41) => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const i18n = createI18n({ legacy: false, locale: 'en', messages: { en } })
    return mount(NewFeatureBadge, {
      props: { noticeId: NEW_FEATURE_NOTICES.adminUsagePlatform, userId },
      global: { plugins: [pinia, i18n] },
    })
  }

  it('dismisses only for the current administrator and persists the notice locally', async () => {
    const wrapper = mountBadge()
    expect(wrapper.get('[data-testid="new-feature-badge"]').text()).toBe('New')
    await wrapper.get('[data-testid="new-feature-badge"]').trigger('click')
    expect(wrapper.find('[data-testid="new-feature-badge"]').exists()).toBe(false)
    expect(useNewFeatureNoticeStore().isDismissed(41, NEW_FEATURE_NOTICES.adminUsagePlatform)).toBe(true)
    expect(mountBadge().find('[data-testid="new-feature-badge"]').exists()).toBe(false)
    expect(mountBadge(42).find('[data-testid="new-feature-badge"]').exists()).toBe(true)
    wrapper.unmount()
  })
})
