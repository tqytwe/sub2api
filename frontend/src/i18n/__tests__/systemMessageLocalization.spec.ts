import { readdirSync, readFileSync } from 'node:fs'
import { extname, join, resolve } from 'node:path'

import { describe, expect, it } from 'vitest'

const SOURCE_ROOT = resolve(process.cwd(), 'src')
const DIRECT_TOAST_LITERAL = /\b(?:showError|showSuccess|showWarning|showInfo)\s*\(\s*(['\"])[\s\S]*?\1/g
const TOAST_ERROR_FALLBACK = /\b(?:showError|showSuccess|showWarning|showInfo)\s*\(\s*[A-Za-z_$][\w$]*(?:\?\.)?[\w$]+\s*(?:\|\||\?\?)\s*(['\"])[A-Za-z][^'\"]*\1/g
const LITERAL_LOCALE_ARGUMENT = /\b(?:\$?t|i18n\.global\.t)\s*\(\s*(['\"])[A-Za-z][A-Za-z0-9_.-]*\1\s*,\s*(['\"])[^'\"\n]+\2/g

function sourceFiles(directory: string): string[] {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const file = join(directory, entry.name)
    if (entry.isDirectory()) return entry.name === '__tests__' ? [] : sourceFiles(file)
    return entry.isFile() && ['.ts', '.vue'].includes(extname(entry.name)) ? [file] : []
  })
}

describe('system message localization', () => {
  it('does not pass a hard-coded message directly to a user-visible toast', () => {
    const violations = sourceFiles(SOURCE_ROOT).flatMap((file) => {
      const content = readFileSync(file, 'utf8')
      return [...content.matchAll(DIRECT_TOAST_LITERAL), ...content.matchAll(TOAST_ERROR_FALLBACK)].map((match) => {
        const line = content.slice(0, match.index).split('\n').length
        return `${file.slice(SOURCE_ROOT.length + 1)}:${line}`
      })
    })

    expect(violations).toEqual([])
  })

  it('does not mistake a literal fallback message for the i18n locale argument', () => {
    const violations = sourceFiles(SOURCE_ROOT).flatMap((file) => {
      const content = readFileSync(file, 'utf8')
      return [...content.matchAll(LITERAL_LOCALE_ARGUMENT)].map((match) => {
        const line = content.slice(0, match.index).split('\n').length
        return `${file.slice(SOURCE_ROOT.length + 1)}:${line}`
      })
    })

    expect(violations).toEqual([])
  })

  it('keeps the WeChat native-app restriction in the route locale fragments', () => {
    const callbackSource = readFileSync(resolve(SOURCE_ROOT, 'views/auth/WechatCallbackView.vue'), 'utf8')
    const entrySource = readFileSync(resolve(SOURCE_ROOT, 'components/auth/WechatOAuthSection.vue'), 'utf8')

    expect(callbackSource).toContain("t('auth.oauthFlow.wechatNativeAppOnly')")
    expect(entrySource).toContain("t('auth.oauthFlow.wechatNativeAppOnly')")
    expect(callbackSource).not.toContain('This WeChat sign-in flow is only available from the native mobile app.')
    expect(entrySource).not.toContain('当前仅配置微信移动应用登录，需要在原生 App 中通过微信 SDK 发起授权。')
  })
})
