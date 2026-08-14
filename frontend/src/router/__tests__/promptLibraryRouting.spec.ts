import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const routerSource = readFileSync(resolve(__dirname, '../index.ts'), 'utf8')
const publicNavigationSource = readFileSync(resolve(__dirname, '../publicNavigation.ts'), 'utf8')
const homeZhSource = readFileSync(resolve(__dirname, '../../i18n/locales/jisudeng-home.zh.ts'), 'utf8')

describe('旧提示词广场入口', () => {
  it('移除公开广场和详情路由，但保留管理员提示词管理', () => {
    expect(routerSource).not.toContain("path: '/prompts'")
    expect(routerSource).not.toContain("path: '/prompts/:id'")
    expect(routerSource).toContain("path: '/admin/prompts'")
    expect(routerSource).toContain("title: '提示词管理'")
  })

  it('首页公开导航不再暴露提示词广场', () => {
    expect(publicNavigationSource).not.toContain("key: 'prompts'")
    expect(publicNavigationSource).not.toContain('PUBLIC_ROUTE_NAMES.promptSquare')
    expect(homeZhSource).not.toContain("prompts: '提示词广场'")
  })
})
