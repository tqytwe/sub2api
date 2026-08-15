import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import MobileReleaseManager from '../MobileReleaseManager.vue'

const api = vi.hoisted(() => ({
  listMobileReleases: vi.fn(),
  publishMobileRelease: vi.fn(),
  pauseMobileRelease: vi.fn(),
  retireMobileRelease: vi.fn(),
  uploadMobileRelease: vi.fn(),
}))

vi.mock('@/api/admin/play', () => ({ default: api }))
vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }),
}))
vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
}))

function release(id: number, version: string, versionCode: number, status: 'ready' | 'published' | 'paused') {
  return {
    id,
    distribution: 'direct' as const,
    package_name: 'com.jisudeng.chat',
    artifact_type: 'apk' as const,
    version,
    version_code: versionCode,
    bytes: 1024,
    sha256: 'a'.repeat(64),
    signing_certificate_sha256: 'b'.repeat(64),
    notes: ['note'],
    notes_i18n: { zh: ['zh'], en: ['en'], ja: ['ja'], ko: ['ko'] },
    manifest: {
      platform: 'android' as const,
      version,
      versionCode,
      artifactType: 'apk' as const,
      packageName: 'com.jisudeng.chat',
      bytes: 1024,
      sha256: 'a'.repeat(64),
      signingCertificateSha256: 'b'.repeat(64),
      notes: ['note'],
      notes_i18n: { zh: ['zh'], en: ['en'], ja: ['ja'], ko: ['ko'] },
    },
    status,
    rollout_percent: 100,
    created_at: '2026-08-15T00:00:00Z',
    updated_at: '2026-08-15T00:00:00Z',
  }
}

describe('MobileReleaseManager', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('refreshes both rows after publishing pauses the previous channel release', async () => {
    const oldPublished = release(1, '3.0.0', 300, 'published')
    const nextReady = release(2, '3.0.1', 301, 'ready')
    const oldPaused = { ...oldPublished, status: 'paused' as const }
    const nextPublished = { ...nextReady, status: 'published' as const }
    api.listMobileReleases
      .mockResolvedValueOnce({ items: [nextReady, oldPublished] })
      .mockResolvedValueOnce({ items: [nextPublished, oldPaused] })
    api.publishMobileRelease.mockResolvedValue(nextPublished)

    const wrapper = mount(MobileReleaseManager, {
      global: { stubs: { Icon: true } },
    })
    await flushPromises()

    const publish = wrapper.findAll('button').find((button) => button.text() === 'admin.playOps.release.publish')
    expect(publish).toBeDefined()
    await publish!.trigger('click')
    await flushPromises()

    expect(api.publishMobileRelease).toHaveBeenCalledWith(2)
    expect(api.listMobileReleases).toHaveBeenCalledTimes(2)
    const rows = wrapper.findAll('article')
    expect(rows.find((row) => row.text().includes('3.0.0'))?.text()).toContain('admin.playOps.release.statuses.paused')
    expect(rows.find((row) => row.text().includes('3.0.1'))?.text()).toContain('admin.playOps.release.statuses.published')
  })
})
