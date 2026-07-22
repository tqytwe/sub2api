import { describe, expect, it } from 'vitest'
import { isHomeContentUrl, sanitizeHomeContent } from '@/utils/homeContent'

describe('home content helpers', () => {
  it('detects URL-based custom home pages without sanitizer work', () => {
    expect(isHomeContentUrl('https://example.com/home')).toBe(true)
    expect(isHomeContentUrl(' http://example.com/home ')).toBe(true)
    expect(isHomeContentUrl('<strong>hello</strong>')).toBe(false)
  })

  it('returns an empty sanitized body for blank and URL custom content', async () => {
    await expect(sanitizeHomeContent('')).resolves.toBe('')
    await expect(sanitizeHomeContent('https://example.com/home')).resolves.toBe('')
  })

  it('sanitizes inline custom home HTML on demand', async () => {
    const html = await sanitizeHomeContent('<img src=x onerror="alert(1)"><strong>Hi</strong>')

    expect(html).toContain('<strong>Hi</strong>')
    expect(html).not.toContain('onerror')
  })
})
