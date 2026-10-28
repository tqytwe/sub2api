import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import CredentialOperationFailure from '../CredentialOperationFailure.vue'
import { safeCredentialOperationFailure } from '@/utils/credentialOperationFailure'

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

describe('credential operation failure display', () => {
  it('maps allowlisted codes to a localized action and timestamp', () => {
    const wrapper = mount(CredentialOperationFailure, {
      props: { failure: { code: 'requires_relogin', occurred_at: '2026-10-28T00:00:00Z' } },
    })
    expect(wrapper.text()).toContain('admin.accounts.credentialFailure.reasons.requires_relogin')
    expect(wrapper.text()).toContain('admin.accounts.credentialFailure.actions.reauthorize')
    expect(wrapper.text()).toContain('admin.accounts.credentialFailure.code: requires_relogin')
  })

  it('never exposes arbitrary error strings or invalid timestamps', () => {
    const safe = safeCredentialOperationFailure({ code: 'provider response contains a token', occurred_at: 'not-a-date' })
    expect(safe).toEqual({ code: 'unknown', action: 'reviewDiagnostics' })
    const wrapper = mount(CredentialOperationFailure, {
      props: { failure: { code: 'provider response contains a token', occurred_at: 'not-a-date' } },
    })
    expect(wrapper.text()).not.toContain('provider response contains a token')
    expect(wrapper.text()).toContain('admin.accounts.credentialFailure.reasons.unknown')
  })
})
