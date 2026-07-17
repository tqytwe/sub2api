import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const routerSource = readFileSync(resolve(__dirname, '../index.ts'), 'utf8')
const sidebarSource = readFileSync(
  resolve(__dirname, '../../components/layout/AppSidebar.vue'),
  'utf8',
)
const zhSource = readFileSync(resolve(__dirname, '../../i18n/locales/zh.ts'), 'utf8')

describe('提示词平台入口', () => {
  it('提供公开广场、公开详情和管理员路由，并使用中文静态标题', () => {
    expect(routerSource).toContain("path: '/prompts'")
    expect(routerSource).toContain("path: '/prompts/:id'")
    expect(routerSource).toContain("path: '/admin/prompts'")
    expect(routerSource).toContain("title: '图像工作室 · 选提示词'")
    expect(routerSource).toContain("title: '提示词详情'")
    expect(routerSource).toContain("title: '提示词管理'")
  })

  it('不在用户侧新增提示词入口，仅保留管理员管理入口', () => {
    expect(sidebarSource).not.toContain("{ path: '/prompts', label: t('nav.promptSquare')")
    expect(sidebarSource).toContain("{ path: '/admin/prompts', label: t('nav.promptManagement')")
    expect(zhSource).toContain("promptSquare: '提示词广场'")
    expect(zhSource).toContain("promptManagement: '提示词管理'")
  })
})
