import { mkdirSync, mkdtempSync, rmSync, unlinkSync, writeFileSync } from 'node:fs'
import { execFileSync } from 'node:child_process'
import { createRequire } from 'node:module'
import { tmpdir } from 'node:os'
import { dirname, join, resolve } from 'node:path'
import { pathToFileURL } from 'node:url'

const repo = resolve(process.cwd())
const root = join(repo, 'frontend')
const output = resolve(process.env.ENCRYPTED_HISTORY_400_OUTPUT || join(repo, 'tmp-encrypted-history-400'))
const port = Number(process.env.ENCRYPTED_HISTORY_400_PORT || 4297)
const origin = `http://127.0.0.1:${port}`
const require = createRequire(join(root, 'package.json'))
const { createServer } = await import(pathToFileURL(join(dirname(require.resolve('vite/package.json')), 'dist/node/index.js')).href)
const vue = (await import(pathToFileURL(require.resolve('@vitejs/plugin-vue')).href)).default
const playwrightPath = process.env.ENCRYPTED_HISTORY_400_PLAYWRIGHT_MODULE
if (!playwrightPath) throw Error('ENCRYPTED_HISTORY_400_PLAYWRIGHT_MODULE is required')
const { chromium } = await import(pathToFileURL(playwrightPath).href)
const cache = mkdtempSync(join(tmpdir(), 'encrypted-history-400-'))
const entryName = `.encrypted-history-400-${process.pid}.js`
const entry = join(root, entryName)
const errors = []
let server
let browser

mkdirSync(output, { recursive: true })
process.chdir(root)
writeFileSync(entry, `
import { createApp, ref } from 'vue'
import { createPinia } from 'pinia'
import { i18n, loadLocaleMessages } from '/src/i18n/index.ts'
import Setting from '/src/components/settings/EncryptedHistory400Settings.vue'
import '/src/style.css'
const locale = new URL(location.href).searchParams.get('locale') === 'en' ? 'en' : 'zh'
await loadLocaleMessages(locale, 'core')
await loadLocaleMessages(locale, 'admin-settings')
i18n.global.locale.value = locale
createApp({
  components: { Setting },
  setup() { return { enabled: ref(false) } },
  template: '<main class="min-h-screen p-6"><section class="grid grid-cols-1 gap-6 md:grid-cols-2"><Setting v-model="enabled" :user-id="12345" /></section></main>'
}).use(createPinia()).use(i18n).mount('#app')
`, { flag: 'wx' })

try {
  server = await createServer({
    root,
    configFile: false,
    cacheDir: cache,
    resolve: { alias: { '@': join(root, 'src'), vue: 'vue/dist/vue.esm-bundler.js' } },
    plugins: [{
      name: 'encrypted-history-400-preview',
      configureServer(vite) {
        vite.middlewares.use((request, response, next) => {
          if (request.url === '/' || request.url?.startsWith('/?')) {
            response.setHeader('Content-Type', 'text/html')
            response.end(`<!doctype html><html lang="zh"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"></head><body><div id="app"></div><script type="module" src="/${entryName}"></script></body></html>`)
          } else if (request.url === '/favicon.ico') {
            response.statusCode = 204
            response.end()
          } else next()
        })
      },
    }, vue()],
    server: { host: '127.0.0.1', port, strictPort: true },
  })
  await server.listen()
  browser = await chromium.launch({
    headless: true,
    ...(process.env.CHROMIUM_EXECUTABLE ? { executablePath: process.env.CHROMIUM_EXECUTABLE, args: ['--no-sandbox'] } : {}),
  })
  const page = await browser.newPage({ viewport: { width: 1280, height: 900 }, reducedMotion: 'reduce' })
  page.on('pageerror', error => errors.push(error.message))
  page.on('console', message => {
    if (message.type() === 'error') errors.push(message.text())
  })
  await page.route('**/*', route => {
    const url = new URL(route.request().url())
    if (url.origin === origin || url.protocol === 'data:') return route.continue()
    errors.push(`Blocked external request: ${url.origin}`)
    return route.abort()
  })

  const capture = async (name, width, height, locale = 'zh', dark = false, enabled = false) => {
    await page.setViewportSize({ width, height })
    await page.goto(`${origin}/?locale=${locale}`, { waitUntil: 'networkidle' })
    if (dark) await page.evaluate(() => document.documentElement.classList.add('dark'))
    const toggle = page.getByTestId('openai-400-omit-encrypted-history-toggle')
    await toggle.waitFor()
    if (enabled) await toggle.click()
    if (await toggle.getAttribute('aria-checked') !== String(enabled)) throw Error(`${name}: unexpected toggle state`)
    const hasOverflow = await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth)
    if (hasOverflow) throw Error(`${name}: horizontal overflow at ${width}px`)
    const body = await page.locator('body').innerText()
    if (/(?:admin\.|common\.)?[A-Za-z0-9_.]+\.[A-Za-z0-9_.]+/.test(body)) throw Error(`${name}: unresolved locale key`)
    await page.screenshot({ path: join(output, `${name}.png`), fullPage: true, animations: 'disabled' })
  }

  for (const [width, height] of [[360, 800], [768, 900], [1280, 900], [1920, 900]]) {
    await capture(`default-off-zh-light-${width}`, width, height)
  }
  await page.setViewportSize({ width: 1280, height: 900 })
  await page.goto(`${origin}/?locale=zh`, { waitUntil: 'networkidle' })
  const toggle = page.getByTestId('openai-400-omit-encrypted-history-toggle')
  await toggle.waitFor()
  await toggle.focus()
  await page.keyboard.press('Space')
  if (await toggle.getAttribute('aria-checked') !== 'true') throw Error('Space did not enable the switch')
  await page.screenshot({ path: join(output, 'keyboard-enabled-zh-light-1280.png'), fullPage: true, animations: 'disabled' })
  await capture('enabled-en-dark-360', 360, 800, 'en', true, true)
  await capture('default-off-en-light-1280', 1280, 900, 'en')
  const badge = page.getByTestId('new-feature-badge')
  await badge.waitFor({ state: 'visible' })
  await badge.click()
  await badge.waitFor({ state: 'detached' })
  await page.reload({ waitUntil: 'networkidle' })
  await toggle.waitFor()
  if (await badge.count()) throw Error('Feature notice reappeared after dismissal and reload')
  await page.screenshot({ path: join(output, 'notice-dismissed-en-light-1280.png'), fullPage: true, animations: 'disabled' })

  const sourceCommit = execFileSync('git', ['rev-parse', 'HEAD'], { cwd: repo, encoding: 'utf8' }).trim()
  writeFileSync(join(output, 'manifest.json'), JSON.stringify({
    source_commit: sourceCommit,
    working_tree: 'The preview ran against the checked-out files; uncommitted changes may be present.',
    surface: 'Production EncryptedHistory400Settings.vue component with synthetic settings state; no API or production data.',
    browser_errors: errors,
  }, null, 2))
  if (errors.length) throw Error(errors.join('\n'))
} finally {
  await browser?.close()
  await server?.close()
  unlinkSync(entry)
  rmSync(cache, { recursive: true, force: true })
}
