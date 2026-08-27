import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const routerSource = readFileSync(resolve(process.cwd(), 'src/router/index.ts'), 'utf8')
const launchViewSource = readFileSync(resolve(process.cwd(), 'src/views/user/CanvasLaunchView.vue'), 'utf8')

describe('AI creation space web routing', () => {
  it('uses the unified AI creation space as the protected web entry', () => {
    expect(routerSource).toMatch(
      /path: '\/ai-creation-space'[\s\S]*?name: 'AICreationSpace'[\s\S]*?component: \(\) => import\('@\/views\/user\/CanvasLaunchView\.vue'\)[\s\S]*?requiresAuth: true/,
    )
  })

  it('keeps legacy web URLs as compatibility redirects', () => {
    const legacyRedirect = "redirect: to => ({ path: '/ai-creation-space', query: to.query })"
    expect(routerSource).toMatch(/path: '\/ai',\s*redirect: to => \(\{ path: '\/ai-creation-space', query: to\.query \}\)/)
    expect(routerSource).toMatch(/path: '\/image-studio',\s*redirect: to => \(\{ path: '\/ai-creation-space', query: to\.query \}\)/)
    expect(routerSource).toContain(legacyRedirect)
  })

  it('does not route the web entry through the retired NextChat page', () => {
    expect(routerSource).not.toContain("component: () => import('@/views/user/NextChatLaunchView.vue')")
    expect(routerSource).toContain("component: () => import('@/views/user/CanvasLaunchView.vue')")
  })

  it('does not remove the mobile or API implementation by deleting session routes', () => {
    expect(readFileSync(resolve(process.cwd(), '../backend/internal/server/routes/nextchat.go'), 'utf8'))
      .toContain('mobile/sessions/:purpose/group')
  })

  it('opens Canvas directly without minting or forwarding an authentication token', () => {
    expect(launchViewSource).toContain("const canvasURL = new URL('https://canvas.jisudeng.com')")
    expect(launchViewSource).not.toContain('launchAICreationSpace')
    expect(launchViewSource).not.toContain('launch_token')
    expect(launchViewSource).toContain("canvasURL.searchParams.set('creation_prompt'")
  })

  it('does not depend on the retired public prompt-square routes', () => {
    expect(routerSource).not.toContain("PromptSquareView.vue")
    expect(routerSource).not.toContain("PromptDetailView.vue")
  })
})
