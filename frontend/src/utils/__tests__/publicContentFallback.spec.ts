import { describe, expect, it } from 'vitest'

import { removePublicContentFallback } from '../publicContentFallback'

describe('public content fallback', () => {
  it('removes the server-rendered fallback before Vue mounts', () => {
    document.body.innerHTML = '<div id="app"><main data-jisudeng-public-fallback="true"><h1>Catalog</h1></main></div>'

    removePublicContentFallback()

    expect(document.querySelector('[data-jisudeng-public-fallback="true"]')).toBeNull()
    expect(document.querySelector('#app')).toBeTruthy()
  })

  it('does not remove unrelated application content', () => {
    document.body.innerHTML = '<div id="app"><main><h1>Interactive app</h1></main></div>'

    removePublicContentFallback()

    expect(document.querySelector('#app h1')?.textContent).toBe('Interactive app')
  })
})
