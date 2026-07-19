import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

const publicPagesCss = readFileSync(
  resolve(process.cwd(), 'src/styles/public-pages.css'),
  'utf8',
)
const supportFloatingCss = readFileSync(
  resolve(process.cwd(), 'src/styles/support-floating.css'),
  'utf8',
)
const publicDocsData = readFileSync(
  resolve(process.cwd(), 'src/content/public-docs-data.zh.ts'),
  'utf8',
)

describe('public docs mobile layout contracts', () => {
  it('allows long inline API URLs inside tip blocks to wrap without widening the page', () => {
    expect(publicPagesCss).toMatch(
      /\.docs-prose \.docs-tip\s*\{[^}]*max-width:\s*100%;[^}]*min-width:\s*0;[^}]*overflow-wrap:\s*anywhere;/s,
    )
    expect(publicPagesCss).toMatch(
      /\.docs-prose \.docs-tip code\s*\{[^}]*white-space:\s*normal;[^}]*max-width:\s*100%;/s,
    )
  })

  it('keeps the support control hidden on docs-sized mobile and tablet viewports', () => {
    expect(supportFloatingCss).toMatch(
      /@media \(max-width:\s*1279px\)[\s\S]*\.support-fab\.support-fab--mobile-hidden\s*\{[\s\S]*display:\s*none;/,
    )
  })

  it('wraps the compact async endpoint list while preserving scrollable command blocks', () => {
    expect(publicDocsData).toContain('<pre class="docs-endpoint-list"><code>POST https://api.jisudeng.com/v1/images/generations/async')
    expect(publicPagesCss).toMatch(
      /\.docs-prose pre\.docs-endpoint-list code\s*\{[^}]*white-space:\s*pre-wrap;[^}]*overflow-wrap:\s*anywhere;[^}]*word-break:\s*break-word;/s,
    )
  })

  it('keeps recharge VIP docs on the balance bonus wording', () => {
    expect(publicDocsData).toContain('VIP 分级 · 充值到账加赠')
    expect(publicDocsData).toContain('V0-V5 六档会员等级')
    expect(publicDocsData).toContain('VIP 不改变 API 计费公式，只影响余额充值到账加赠')
    expect(publicDocsData).toContain('本单按充值前 VIP 等级计算')
    expect(publicDocsData).not.toContain('VIP 分级 · 享受折扣')
    expect(publicDocsData).not.toContain('4 档会员等级')
    expect(publicDocsData).not.toContain('充值的钱会 1:1')
    expect(publicDocsData).not.toContain('折扣永久生效')
    expect(publicDocsData).not.toContain('盲盒奖池升级（暂未启用）')
    expect(publicDocsData).toContain('VIP 专属奖池')
    expect(publicDocsData).toContain('团队奖池 = 团队月消费 × 当前达标比例')
    expect(publicDocsData).toContain('活动倍率只影响展示积分和排名')
  })
})
