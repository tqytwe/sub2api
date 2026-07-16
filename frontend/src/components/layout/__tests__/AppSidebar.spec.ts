import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

import en from '../../../i18n/locales/en'
import zh from '../../../i18n/locales/zh'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppSidebar.vue')
const componentSource = readFileSync(componentPath, 'utf8')
const stylePath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../style.css')
const styleSource = readFileSync(stylePath, 'utf8')

function sidebarNavKeys(): string[] {
  return [...componentSource.matchAll(/t\('nav\.([^']+)'\)/g)]
    .map((match) => match[1])
    .filter((key, index, keys) => keys.indexOf(key) === index)
    .sort()
}

describe('AppSidebar custom SVG styles', () => {
  it('does not override uploaded SVG fill or stroke colors', () => {
    expect(componentSource).toContain('.sidebar-svg-icon {')
    expect(componentSource).toContain('color: currentColor;')
    expect(componentSource).toContain('display: block;')
    expect(componentSource).not.toContain('stroke: currentColor;')
    expect(componentSource).not.toContain('fill: none;')
  })
})

describe('AppSidebar custom docs menu icon', () => {
  it('uses the built-in sidebar book icon for docs custom menu items', () => {
    expect(componentSource).toContain('const BookIcon = {')
    expect(componentSource).toContain('function isDocsCustomMenuItem')
    expect(componentSource).toContain('function buildCustomMenuNavItem')
    expect(componentSource).toContain('iconSvg: icon ? undefined : item.icon_svg')
    expect(componentSource).toContain('...customMenuItemsForUser.value.map(buildCustomMenuNavItem)')
    expect(componentSource).toContain('visible.push(buildCustomMenuNavItem(cm))')
    expect(componentSource).toContain('filtered.push(buildCustomMenuNavItem(cm))')
  })
})

describe('AppSidebar navigation labels', () => {
  it.each([
    ['zh', zh],
    ['en', en]
  ] as const)('has runtime translations for every %s sidebar nav key', (_locale, messages) => {
    const nav = (messages as { nav: Record<string, string> }).nav
    const missing = sidebarNavKeys().filter((key) => typeof nav[key] !== 'string' || nav[key].trim() === '')
    expect(missing).toEqual([])
  })

  it('translates the audit-log nav key and the lowercase legacy config key', () => {
    expect(zh.nav.auditLogs).toBe('操作日志')
    expect(zh.nav.auditlogs).toBe('操作日志')
    expect(en.nav.auditLogs).toBe('Audit Logs')
    expect(en.nav.auditlogs).toBe('Audit Logs')
    expect(componentSource).toContain('function resolveCustomMenuLabel')
    expect(componentSource).toContain('resolveCustomMenuLabel(item.label)')
  })
})

describe('AppSidebar scroll position persistence', () => {
  it('binds a template ref to the sidebar nav element', () => {
    expect(componentSource).toContain('ref="sidebarNavRef"')
    expect(componentSource).toContain('sidebar-nav')
  })

  it('declares sidebarNavRef in script setup', () => {
    expect(componentSource).toContain("const sidebarNavRef = ref<HTMLElement | null>(null)")
  })

  it('saves scroll position on beforeUnmount', () => {
    expect(componentSource).toContain('onBeforeUnmount')
    expect(componentSource).toContain('appStore.sidebarScrollTop')
    expect(componentSource).toContain('sidebarNavRef.value.scrollTop')
  })

  it('restores scroll position on mount', () => {
    expect(componentSource).toContain('onMounted')
    expect(componentSource).toContain('appStore.sidebarScrollTop')
    expect(componentSource).toContain('nextTick')
  })
})

describe('AppSidebar header styles', () => {
  it('only shows the version badge to admins', () => {
    expect(componentSource).toContain('VersionBadge v-if="isAdmin"')
    expect(componentSource).not.toMatch(/<VersionBadge(?![^>]*v-if="isAdmin")[^>]*:version="siteVersion"/)
  })

  it('does not clip the version badge dropdown', () => {
    const sidebarHeaderBlockMatch = styleSource.match(/\.sidebar-header\s*\{[\s\S]*?\n {2}\}/)
    const sidebarBrandBlockMatch = componentSource.match(/\.sidebar-brand\s*\{[\s\S]*?\n\}/)

    expect(sidebarHeaderBlockMatch).not.toBeNull()
    expect(sidebarBrandBlockMatch).not.toBeNull()
    expect(sidebarHeaderBlockMatch?.[0]).not.toContain('@apply overflow-hidden;')
    expect(sidebarBrandBlockMatch?.[0]).not.toContain('overflow: hidden;')
  })
})

describe('AppSidebar Fork navigation invariants', () => {
  const selfNavBlock = componentSource.match(/function buildSelfNavItems[\s\S]*?\n}\n\n\/\/ finalizeNav/)?.[0] ?? ''

  it('keeps the models, image tools, and Growth group in user navigation', () => {
    expect(selfNavBlock).toContain("path: '/models'")
    expect(selfNavBlock).toContain("path: '/image-studio'")
    expect(selfNavBlock).toContain("path: '/batch-image'")
    expect(selfNavBlock).toContain("path: '/growth-group'")
    expect(componentSource).toContain('children: buildGrowthNavChildren()')
  })

  it('does not expose channel operations in user navigation', () => {
    expect(selfNavBlock).not.toContain("path: '/available-channels'")
    expect(selfNavBlock).not.toContain("path: '/monitor'")
  })
})
