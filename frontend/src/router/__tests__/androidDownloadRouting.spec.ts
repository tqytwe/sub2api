import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

import { describe, expect, it } from 'vitest'

describe('Android download route integration', () => {
  it('exposes the official APK page as a public route and home entry', () => {
    const routerSource = readFileSync(resolve(process.cwd(), 'src/router/index.ts'), 'utf8')
    const navigationSource = readFileSync(resolve(process.cwd(), 'src/router/publicNavigation.ts'), 'utf8')
    const homeSource = readFileSync(resolve(process.cwd(), 'src/views/HomeView.vue'), 'utf8')
    const downloadSource = readFileSync(resolve(process.cwd(), 'src/views/public/AndroidDownloadView.vue'), 'utf8')

    expect(routerSource).toContain("path: '/download/android'")
    expect(routerSource).toContain("requiresAuth: false")
    expect(routerSource).toContain("component: () => import('@/views/public/AndroidDownloadView.vue')")
    expect(routerSource).toContain("'/download/android'")
    expect(navigationSource).toContain("androidDownload: 'AndroidDownload'")
    expect(navigationSource).toContain("labelKey: 'home.jisudeng.nav.androidApp'")
    expect(homeSource).toContain('class="nav-download"')
    expect(downloadSource).toContain("const APK_PATH = '/downloads/jisudengchat-android.apk?v=2.0.1-20001'")
    expect(downloadSource).toContain("const MANIFEST_PATH = '/downloads/android-version.json'")
  })
})
