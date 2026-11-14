import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const viewSource = readFileSync(
  resolve(dirname(fileURLToPath(import.meta.url)), '../GroupsView.vue'),
  'utf8',
)
const layoutSource = readFileSync(
  resolve(dirname(fileURLToPath(import.meta.url)), '../../../components/layout/TablePageLayout.vue'),
  'utf8',
)

describe('mobile groups search upstream regression', () => {
  it('keeps the search input below the sticky header when it regains focus', () => {
    expect(viewSource).toContain('class="input scroll-mt-20 pl-10"')
    expect(viewSource).toContain('@focus="handleSearchFocus"')
    expect(viewSource).toContain('const handleSearchFocus = (event: FocusEvent) =>')
    expect(viewSource).toContain('input.scrollIntoView({ block: "start", behavior: "auto" })')
  })

  it('lets mobile table content define the page height', () => {
    expect(layoutSource).toMatch(
      /\.table-page-layout\.mobile-mode\s*\{[^}]*height:\s*auto;/s,
    )
  })
})
