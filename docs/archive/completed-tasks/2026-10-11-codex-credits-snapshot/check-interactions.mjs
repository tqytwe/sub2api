// Supplemental checks reuse capture.mjs's built after/dist; no rebuild or server.
import { pathToFileURL } from 'node:url'
const { chromium } = await import(pathToFileURL(process.env.PLAYWRIGHT_MODULE))
import { readFile, writeFile } from 'node:fs/promises'
import path from 'node:path'
const [dist, assets] = process.argv.slice(2).map(value => path.resolve(value))
const browser = await chromium.launch({ executablePath: '/usr/bin/chromium', headless: true, args: ['--no-sandbox'] })
const observations = []
try {
  for (const [language, theme] of [['zh', 'light'], ['en', 'dark']]) {
    const page = await browser.newPage({ viewport: { width: 1280, height: 1000 }, reducedMotion: 'reduce', timezoneId: 'UTC', locale: language === 'zh' ? 'zh-CN' : 'en-US' })
    await page.route('**/*', async route => {
      const url = new URL(route.request().url())
      if (url.hostname !== 'quota.fixture') throw new Error('External request forbidden')
      const file = path.join(dist, url.pathname === '/' ? 'index.html' : url.pathname.slice(1))
      await route.fulfill({ body: await readFile(file), contentType: file.endsWith('.js') ? 'text/javascript' : file.endsWith('.css') ? 'text/css' : 'text/html' })
    })
    await page.goto('http://quota.fixture/?language=' + language + '&theme=' + theme)
    await page.evaluate(() => window.prepare())
    const button = page.locator('[data-row="2"] [data-testid="codex-credits"]')
    await button.focus()
    await page.keyboard.down('Space')
    const active = await button.evaluate(element => element.matches(':active'))
    await page.screenshot({ path: path.join(assets, `after-${language}-${theme}-1280-keydown.png`), fullPage: true })
    // Blur before key-up so this state capture issues no additional query.
    await button.evaluate(element => element.blur()); await page.keyboard.up('Space')
    const motion = await page.locator('[data-row="6"] svg.animate-spin').evaluate(element => ({ duration: getComputedStyle(element).animationDuration, name: getComputedStyle(element).animationName }))
    await page.evaluate(() => { document.documentElement.style.zoom = '2' })
    const zoom = await page.evaluate(() => ({ overflow: document.documentElement.scrollWidth > innerWidth, resetCalls: window.resetCalls, queryCalls: window.queryCalls, points: [...document.querySelectorAll('[data-testid="codex-credits"]')].map(element => { const rect = element.getBoundingClientRect(); return { left: rect.left, right: rect.right, width: rect.width } }) }))
    if (zoom.overflow || zoom.resetCalls || zoom.queryCalls.length !== 3) throw new Error(JSON.stringify({ active, zoom, motion }))
    observations.push({ language, theme, active, reducedMotionRequested: true, motion, zoom })
    await page.close()
  }
} finally { await browser.close() }
await writeFile(path.join(assets, 'interaction-checks.json'), JSON.stringify(observations, null, 2) + '\n')
