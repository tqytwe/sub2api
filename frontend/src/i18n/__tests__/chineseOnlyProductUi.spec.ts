import { readFileSync, readdirSync } from 'node:fs'
import { resolve } from 'node:path'

import { describe, expect, it } from 'vitest'

import { availableLocales } from '@/i18n'
import { CHINESE_PRODUCT_TERMS } from '@/i18n/chineseProductTerms'

const srcRoot = resolve(process.cwd(), 'src')

function source(path: string): string {
  return readFileSync(resolve(srcRoot, path), 'utf8')
}

function quotedUiTerm(term: string): RegExp {
  const escaped = term.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  return new RegExp(`['"\`][^'"\`\\n]*\\b${escaped}\\b[^'"\`\\n]*['"\`]`, 'i')
}

function renderedUiTerm(term: string): RegExp {
  const escaped = term.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  return new RegExp(
    `(?:>[^<\\n]*\\b${escaped}\\b[^<\\n]*<|\\s(?:aria-label|alt|title|placeholder)="[^"\\n]*\\b${escaped}\\b[^"\\n]*")`,
    'i',
  )
}

function template(path: string): string {
  const content = source(path)
  return (content.match(/<template>[\s\S]*?<\/template>/)?.[0] ?? '')
    .replace(/\{\{[\s\S]*?\}\}/g, '')
}

function productUiTemplates(): string[] {
  const promptComponents = readdirSync(resolve(srcRoot, 'components/prompt'))
    .filter((name) => name.endsWith('.vue'))
    .map((name) => template(`components/prompt/${name}`))

  return [
    readFileSync(resolve(process.cwd(), 'index.html'), 'utf8'),
    template('components/home/LmspeedBadge.vue'),
    template('components/layout/AppHeader.vue'),
    template('components/layout/AppSidebar.vue'),
    template('views/public/PromptSquareView.vue'),
    template('views/public/PromptDetailView.vue'),
    template('views/admin/PromptsView.vue'),
    template('views/user/ImageStudioView.vue'),
    ...promptComponents,
  ]
}

describe('极速蹬正式界面语言', () => {
  it('只向用户提供简体中文', () => {
    expect(availableLocales).toEqual([
      { code: 'zh', name: '简体中文', flag: '中' },
    ])
    expect(source('i18n/index.ts')).toContain("const DEFAULT_LOCALE: LocaleCode = 'zh'")
    expect(source('i18n/index.ts')).toContain("setAttribute('lang', 'zh-CN')")
  })

  it('产品界面不出现统一术语表中的英文名称', () => {
    const messageAndLabelSources = [
      source('i18n/locales/jisudeng-pages.zh.ts'),
      source('router/index.ts'),
      source('utils/featureFlags.ts'),
    ]
    const templates = productUiTemplates()

    for (const [english, chinese] of Object.entries(CHINESE_PRODUCT_TERMS)) {
      expect(chinese).toMatch(/[\u4e00-\u9fff]/)
      for (const content of messageAndLabelSources) {
        expect(content, `${english} 应显示为 ${chinese}`).not.toMatch(quotedUiTerm(english))
      }
      for (const content of templates) {
        expect(content, `${english} 应显示为 ${chinese}`).not.toMatch(renderedUiTerm(english))
      }
    }
  })

  it('中文公开页不显示 SUPPORT、ABOUT 或 Contact 英文眉题', () => {
    const chinesePublicCopy = source('i18n/locales/jisudeng-pages.zh.ts')
    expect(chinesePublicCopy).not.toMatch(/eyebrow:\s*['"`][^'"`\n]*(?:SUPPORT|ABOUT|Contact)/)
  })
})
