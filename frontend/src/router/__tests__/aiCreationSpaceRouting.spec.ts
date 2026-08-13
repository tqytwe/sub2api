import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const routerSource = readFileSync(resolve(process.cwd(), 'src/router/index.ts'), 'utf8')

describe('AI creation space web routing', () => {
  it('uses the unified AI creation space as the protected web entry', () => {
    expect(routerSource).toMatch(
      /path: '\/ai-creation-space'[\s\S]*?name: 'AICreationSpace'[\s\S]*?requiresAuth: true[\s\S]*?requiresNextChat: true/,
    )
  })

  it('keeps legacy web URLs as compatibility redirects', () => {
    const legacyRedirect = "redirect: to => ({ path: '/ai-creation-space', query: to.query })"
    expect(routerSource).toMatch(/path: '\/ai',\s*redirect: to => \(\{ path: '\/ai-creation-space', query: to\.query \}\)/)
    expect(routerSource).toMatch(/path: '\/image-studio',\s*redirect: to => \(\{ path: '\/ai-creation-space', query: to\.query \}\)/)
    expect(routerSource).toContain(legacyRedirect)
  })

  it('does not remove the mobile or API implementation by deleting web routes', () => {
    expect(routerSource).toContain("component: () => import('@/views/user/NextChatLaunchView.vue')")
    expect(readFileSync(resolve(process.cwd(), '../backend/internal/server/routes/nextchat.go'), 'utf8'))
      .toContain('mobile/sessions/:purpose/group')
  })
})
