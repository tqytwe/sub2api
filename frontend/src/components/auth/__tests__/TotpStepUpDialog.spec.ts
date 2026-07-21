import { flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { ref } from 'vue'
import { describe, expect, it, vi } from 'vitest'

import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
import type { StepUpController } from '@/composables/useStepUp'

const { showError, stepUp } = vi.hoisted(() => ({
  showError: vi.fn(),
  stepUp: vi.fn(),
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError }),
}))

vi.mock('@/api', () => ({
  totpAPI: { stepUp },
}))

function mountDialog(locale: 'zh' | 'en') {
  const messages = {
    zh: {
      common: { cancel: () => '取消', verifying: () => '验证中' },
      stepUp: {
        title: () => '需要二次验证',
        hint: () => '请输入验证码',
        verifyFailed: () => '验证失败，请重试',
        errors: {
          TOTP_INVALID_CODE: () => '验证码错误，请重试',
          TOTP_TOO_MANY_ATTEMPTS: () => '验证尝试过多，请稍后再试',
        },
      },
    },
    en: {
      common: { cancel: () => 'Cancel', verifying: () => 'Verifying' },
      stepUp: {
        title: () => 'Two-Factor Verification Required',
        hint: () => 'Enter the code',
        verifyFailed: () => 'Verification failed, please try again',
        errors: {
          TOTP_INVALID_CODE: () => 'Invalid verification code. Try again.',
          TOTP_TOO_MANY_ATTEMPTS: () => 'Too many verification attempts. Try again later.',
        },
      },
    },
  }
  const i18n = createI18n({ legacy: false, locale, messages })
  const controller = {
    visible: ref(true),
    onVerified: vi.fn(),
    onCancel: vi.fn(),
  } as unknown as StepUpController
  return mount(TotpStepUpDialog, {
    props: { controller },
    global: { plugins: [i18n] },
  })
}

async function enterCode(wrapper: ReturnType<typeof mountDialog>) {
  const inputs = wrapper.findAll('input:not([aria-hidden="true"])')
  expect(inputs).toHaveLength(6)
  for (let index = 0; index < inputs.length; index += 1) {
    await inputs[index].setValue(String(index + 1))
  }
  await flushPromises()
}

describe('TotpStepUpDialog localized errors', () => {
  it.each([
    ['zh', 'TOTP_INVALID_CODE', 'invalid totp code', '验证码错误，请重试'],
    ['en', 'TOTP_TOO_MANY_ATTEMPTS', 'too many attempts', 'Too many verification attempts. Try again later.'],
  ] as const)('maps %s API errors by stable code', async (locale, reason, message, expected) => {
    showError.mockReset()
    stepUp.mockReset().mockRejectedValueOnce({ reason, message })
    const wrapper = mountDialog(locale)

    await enterCode(wrapper)

    expect(showError).toHaveBeenCalledWith(expected)
    expect(showError).not.toHaveBeenCalledWith(message)
    wrapper.unmount()
  })

  it('uses the localized fallback instead of an unknown backend message', async () => {
    showError.mockReset()
    stepUp.mockReset().mockRejectedValueOnce({
      reason: 'UNMAPPED_TOTP_ERROR',
      message: 'backend English failure',
    })
    const wrapper = mountDialog('zh')

    await enterCode(wrapper)

    expect(showError).toHaveBeenCalledWith('验证失败，请重试')
    expect(showError).not.toHaveBeenCalledWith('backend English failure')
    wrapper.unmount()
  })
})
