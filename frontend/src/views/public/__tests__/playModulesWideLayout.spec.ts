import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const root = `${process.cwd()}/src`

function read(path: string): string {
  return readFileSync(`${root}/${path}`, 'utf8')
}

function cssRule(source: string, selector: string): string {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const match = source.match(new RegExp(`${escaped}\\s*\\{([^}]*)\\}`))
  return match?.[1] ?? ''
}

describe('play module wide layout', () => {
  it('keeps public play pages on the wide workspace shell', () => {
    const css = read('styles/public-pages.css')

    expect(cssRule(css, '.play-main')).toContain('max-width: none')
    expect(cssRule(css, '.play-main')).not.toContain('max-width: 760px')
    expect(cssRule(css, '.public-page-header')).toContain('max-width: 1440px')
    expect(css).toContain('.play-detail-grid')
    expect(css).toContain('.play-hero-grid')
  })

  it('keeps user growth pages aligned with the play hub workspace', () => {
    const css = read('styles/growth-world.css')

    expect(cssRule(css, '.gw-page')).toContain('max-width: none')
    expect(cssRule(css, '.gw-page')).not.toContain('max-width: 48rem')
    expect(cssRule(css, '.gw-page--wide')).toContain('max-width: none')
    expect(css).toContain('.gw-detail-grid')
    expect(css).toContain('.gw-hero-grid')
  })

  it('does not reintroduce narrow module-specific shells', () => {
    expect(read('views/public/ArenaView.vue')).not.toContain('max-width: 1120px')
    expect(read('views/public/AgentTeamView.vue')).not.toContain('max-width: 1120px')
    expect(read('views/user/ImageStudioView.vue')).not.toContain('max-w-[1440px]')
  })
})
