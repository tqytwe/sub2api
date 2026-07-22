import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const testDir = dirname(fileURLToPath(import.meta.url))
const pageFrameSource = readFileSync(resolve(testDir, '../PageFrame.vue'), 'utf8')
const routerSource = readFileSync(resolve(testDir, '../../../router/index.ts'), 'utf8')

function styleBlock(selector: string) {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  return [...pageFrameSource.matchAll(new RegExp(`${escaped}\\s*\\{[\\s\\S]*?\\n\\}`, 'g'))]
    .map((match) => match[0])
    .join('\n')
}

function routeMetaBlock(routeName: string) {
  return routerSource.match(new RegExp(`name: '${routeName}',[\\s\\S]*?meta: \\{[\\s\\S]*?\\n    \\}`))?.[0] ?? ''
}

describe('PageFrame route width contract', () => {
  it('keeps console content and workspace frames full width inside AppLayout gutters', () => {
    expect(styleBlock('.page-frame--content')).toContain('max-width: none')
    expect(styleBlock('.page-frame--workspace')).toContain('max-width: none')
    expect(pageFrameSource).not.toMatch(/\\.page-frame--(?:content|workspace)[\\s\\S]*?margin-inline:\\s*auto/)
    expect(pageFrameSource).not.toMatch(/\\.page-frame--(?:content|workspace)[\\s\\S]*?max-width:\\s*(?:72|100)rem/)
  })

  it('limits only compact reading and form flows', () => {
    expect(styleBlock('.page-frame--compact')).toContain('max-width: 42rem')
    expect(styleBlock('.page-frame--reading')).toContain('max-width: 50rem')
    expect(styleBlock('.page-frame--form')).toContain('max-width: 60rem')
  })

  it('classifies user and admin dashboards as workspace pages', () => {
    expect(routeMetaBlock('Dashboard')).toContain("frame: 'workspace'")
    expect(routeMetaBlock('AdminDashboard')).toContain("frame: 'workspace'")
  })
})
