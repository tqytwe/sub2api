import { chromium } from '/tmp/v2193-browser-tools/node_modules/playwright-core/index.mjs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'
const dir = dirname(fileURLToPath(import.meta.url))
const browser = await chromium.launch({ executablePath: '/root/.cache/ms-playwright/chromium-1194/chrome-linux/chrome', args: ['--no-sandbox'] })
try {
  for (const surface of ['sidebar', 'ops', 'public-farm']) {
    const page = await browser.newPage({ viewport: { width: 1280, height: 1000 } })
    await page.goto(pathToFileURL(resolve(dir, 'baseline.html')).href + '?' + surface)
    await page.locator(surface === 'sidebar' ? 'nav' : surface === 'ops' ? 'table' : '.arena-level-card').first().waitFor()
    await page.screenshot({ path: resolve(dir, 'baseline-' + surface + '-1280.png'), fullPage: true })
    await page.close()
  }
} finally { await browser.close() }
