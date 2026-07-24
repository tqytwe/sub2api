import { flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { defineComponent, ref } from 'vue'
import { describe, expect, it, vi } from 'vitest'

import BaseDialog from '@/components/common/BaseDialog.vue'
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
    global: {
      plugins: [i18n],
      stubs: { Teleport: true },
    },
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

describe('TotpStepUpDialog layering', () => {
  it('escapes the inert app root and owns keyboard input above an existing dialog', async () => {
    document.body.innerHTML = '<div id="app"></div>'
    const appRoot = document.getElementById('app') as HTMLElement & { inert?: boolean }
    const baseClose = vi.fn()
    const visible = ref(true)
    const onCancel = vi.fn(() => {
      visible.value = false
    })
    const controller = {
      visible,
      onVerified: vi.fn(),
      onCancel,
    } as unknown as StepUpController
    const i18n = createI18n({
      legacy: false,
      locale: 'zh',
      messages: {
        zh: {
          common: {
            cancel: () => '取消',
            close: () => '关闭',
            verifying: () => '验证中',
          },
          stepUp: {
            title: () => '需要二次验证',
            hint: () => '请输入验证码',
            verifyFailed: () => '验证失败，请重试',
            errors: {},
          },
        },
      },
    })
    const Harness = defineComponent({
      components: { BaseDialog, TotpStepUpDialog },
      setup: () => ({ baseClose, controller }),
      template: `
        <BaseDialog :show="true" title="风险处置" @close="baseClose">
          <button type="button">底层处置操作</button>
        </BaseDialog>
        <TotpStepUpDialog :controller="controller" />
      `,
    })

    const wrapper = mount(Harness, {
      attachTo: appRoot,
      global: { plugins: [i18n] },
    })
    await flushPromises()

    const stepUpTitle = Array.from(document.body.querySelectorAll('h3'))
      .find((element) => element.textContent?.includes('需要二次验证'))
    expect(stepUpTitle).toBeTruthy()
    expect(appRoot.inert).toBe(true)
    expect(appRoot.contains(stepUpTitle!)).toBe(false)
    expect(stepUpTitle!.closest('[role="dialog"]')?.getAttribute('tabindex')).toBe('-1')

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await flushPromises()

    expect(onCancel).toHaveBeenCalledOnce()
    expect(baseClose).not.toHaveBeenCalled()
    expect(appRoot.inert).toBe(true)
    expect(document.body.textContent).not.toContain('需要二次验证')
    wrapper.unmount()
  })
})
