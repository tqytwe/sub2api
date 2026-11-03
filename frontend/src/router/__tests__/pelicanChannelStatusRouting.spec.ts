import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const routerSource = readFileSync(resolve(dirname(fileURLToPath(import.meta.url)), '../index.ts'), 'utf8')
const sidebarSource = readFileSync(resolve(dirname(fileURLToPath(import.meta.url)), '../../components/layout/AppSidebar.vue'), 'utf8')
const pelicanAdminSource = readFileSync(resolve(dirname(fileURLToPath(import.meta.url)), '../../views/admin/PelicanTestsView.vue'), 'utf8')

describe('Pelican user entry points', () => {
  it('keeps the user-facing results inside Channel Status and the admin config page', () => {
    expect(routerSource).toContain("path: '/monitor'")
    expect(routerSource).toContain("path: '/admin/pelican-tests'")
    expect(routerSource).not.toContain("path: '/pelican-showcase'")
    expect(sidebarSource).not.toContain("path: '/pelican-showcase'")
    expect(sidebarSource).toContain("path: '/admin/pelican-tests'")
    expect(pelicanAdminSource).toContain('RouterLink to="/monitor"')
  })
})
