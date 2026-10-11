// Reproducible isolated component screenshots. No HTTP server or real account API.
// Fake reset only; all network outside fixture origin is blocked.
// Usage: PLAYWRIGHT_MODULE=/path/to/playwright/index.mjs node capture.mjs ROOT BASELINE
import { createRequire } from 'node:module'
import { pathToFileURL } from 'node:url'
import { mkdtemp, mkdir, writeFile, readFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import path from 'node:path'
const root = path.resolve(process.argv[2])
const require = createRequire(path.join(root, 'frontend/package.json'))
const { build } = await import(pathToFileURL(path.join(path.dirname(require.resolve('vite/package.json')), 'dist/node/index.js')))
const { default: vue } = await import(pathToFileURL(require.resolve('@vitejs/plugin-vue')))
const { chromium } = await import(pathToFileURL(process.env.PLAYWRIGHT_MODULE))
const scratch = await mkdtemp(path.join(tmpdir(), 'quota-capture-'))
const assets = path.join(root, 'docs/visual-reviews/assets/codex-credits-snapshot')
await mkdir(assets, { recursive: true })
const reports = []
for (const [mode, source] of [['reset-feedback', root]]) {
  const folder = path.join(scratch, mode)
  await mkdir(folder)
  await writeFile(path.join(folder, 'api.ts'), `export const refreshOpenAIQuota = (id) => window.fixtureQuery(id);
export const resetOpenAIQuota = () => window.fixtureReset();
export const refreshOpenAIReferrals = () => { throw new Error('Referral forbidden in fixture'); };
export const sendOpenAIReferralInvite = () => { throw new Error('Invite forbidden in fixture'); };`)
  await writeFile(path.join(folder, 'index.html'), '<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"></head><body><div id="app"></div><script type="module" src="/main.ts"></script></body></html>')
  await writeFile(path.join(folder, 'main.ts'), `
import { createApp, h, ref, nextTick } from 'vue';
import { createI18n } from 'vue-i18n';
import Cell from '${source}/frontend/src/components/account/OpenAIQuotaResetCell.vue';
import zhAdmin from '${source}/frontend/src/i18n/locales/zh/admin/accounts.ts';
import enAdmin from '${source}/frontend/src/i18n/locales/en/admin/accounts.ts';
import zhCommon from '${source}/frontend/src/i18n/locales/zh/common.ts';
import enCommon from '${source}/frontend/src/i18n/locales/en/common.ts';
import '${source}/frontend/src/style.css';
const params = new URLSearchParams(location.search);
const language = params.get('language') || 'zh';
const dark = params.get('theme') === 'dark';
document.documentElement.classList.toggle('dark', dark);
document.documentElement.lang = language;
window.resetCalls = 0; window.queryCalls = [];
const credits = { has_credits: true, unlimited: false, balance: '1234.50' };
const row = ref({ id: 1, name: 'synthetic', platform: 'openai', type: 'oauth', parent_account_id: null,
 extra: { codex_credits_snapshot: { credits, fetched_at: 1770000000 }, codex_reset_credit_snapshot: { available_count: 1, credits: [{ expires_at: '2099-07-03T04:05:06Z' }] } } });
window.fixtureQuery = () => { window.queryCalls.push(1); throw new Error('Query forbidden'); };
window.fixtureReset = async () => { window.resetCalls++; return { code: 'success', windows_reset: 1, cache_refreshed: false, account_state_recovered: true, warning_code: 'reset_credit_cache_refresh_failed', account: JSON.parse(JSON.stringify(row.value)) }; };
window.feedbackCount = 0;
const i18n = createI18n({ legacy: false, locale: language, messages: { zh: { ...zhCommon, admin: zhAdmin }, en: { ...enCommon, admin: enAdmin } } });
createApp({ setup() { return () => h('main', { class: 'p-4 md:p-8', style: { maxWidth: '960px', margin: '0 auto' } }, [
 h('h1', { class: 'text-xl font-semibold mb-2' }, language === 'zh' ? '账号额度 · 隔离组件验收' : 'Account credits · isolated component review'),
 h('p', { class: 'text-sm text-gray-500 mb-5' }, language === 'zh' ? '合成账号与本地响应；真实组件、词条和样式。非生产截图。' : 'Synthetic accounts and local responses; actual component, translations and styles. Not production.'),
 h('h2', { class: 'font-medium mb-3' }, language === 'zh' ? '本地重置成功、回读失败：父行回灌旧快照后仍未知' : 'Local reset succeeded, readback failed: parent feedback remains unknown'),
 h(Cell, { account: row.value, 'onAccount-updated': (updated) => { window.feedbackCount++; row.value = { ...row.value, ...updated }; } })
]); }}).use(i18n).mount('#app');
`)
  await build({ configFile: false, root: folder, plugins: [vue()], resolve: { alias: [
    { find: '@/api/admin/accounts', replacement: path.join(folder, 'api.ts') },
    { find: '@', replacement: path.join(source, 'frontend/src') },
    { find: 'vue', replacement: require.resolve('vue/dist/vue.runtime.esm-bundler.js') },
    { find: 'vue-i18n', replacement: require.resolve('vue-i18n/dist/vue-i18n.esm-bundler.js') }
  ] }, css: { postcss: { plugins: [require('tailwindcss')({ ...require(path.join(root, 'frontend/tailwind.config.js')).default, content: [path.join(source, 'frontend/src/**/*.{vue,js,ts}'), path.join(folder, 'main.ts')] }), require('autoprefixer')()] } }, build: { outDir: path.join(folder, 'dist'), emptyOutDir: true }, logLevel: 'warn' })
  const browser = await chromium.launch({ executablePath: process.env.CHROMIUM_PATH || '/usr/bin/chromium', headless: true, args: ['--no-sandbox'] })
  try {
    const variants = [{ width: 1280, language: 'zh', theme: 'light' }, { width: 360, language: 'en', theme: 'dark' }]
    for (const variant of variants) {
      const page = await browser.newPage({ viewport: { width: variant.width, height: 1000 }, locale: variant.language === 'zh' ? 'zh-CN' : 'en-US', timezoneId: 'UTC', reducedMotion: 'reduce' })
      const errors = []; const external = [];
      page.on('pageerror', error => errors.push(error.message))
      await page.route('**/*', async route => {
        const url = new URL(route.request().url())
        if (url.hostname !== 'quota.fixture') { external.push(url.origin); await route.abort(); return }
        const relative = url.pathname === '/' ? 'index.html' : url.pathname.slice(1)
        const file = path.join(folder, 'dist', relative)
        await route.fulfill({ body: await readFile(file), contentType: file.endsWith('.js') ? 'text/javascript' : file.endsWith('.css') ? 'text/css' : 'text/html' })
      })
      await page.goto('http://quota.fixture/?language=' + variant.language + '&theme=' + variant.theme)
      await page.locator('main button').nth(1).click()
      await page.locator('[role="dialog"] button').last().click()
      await page.waitForFunction(() => window.feedbackCount === 1 && document.querySelector('[data-testid="codex-credits"]').textContent.includes('—'))
      // Vue updates precede the leave transition; capture only its settled state.
      await page.locator('[role="dialog"]').waitFor({ state: 'detached' })
      const name = mode + '-' + variant.language + '-' + variant.theme + '-' + variant.width
      await page.screenshot({ path: path.join(assets, name + '.png'), fullPage: true })
      const checks = await page.evaluate(() => ({ dialogDetached: document.querySelector('[role="dialog"]') === null, queryCalls: window.queryCalls, resetCalls: window.resetCalls, feedbackCount: window.feedbackCount, balance: document.querySelector('[data-testid="codex-credits"]').textContent, overflow: document.documentElement.scrollWidth > innerWidth }))
      if (!checks.dialogDetached || checks.resetCalls !== 1 || checks.feedbackCount !== 1 || checks.queryCalls.length || checks.overflow || external.length || errors.length) throw new Error(JSON.stringify({ checks, external, errors }))
      reports.push({ name, checks, external, errors, resetMode: 'local build-time stub only; no upstream reset' })
      await page.close()
    }
  } finally { await browser.close() }
}
await writeFile(path.join(assets, 'reset-feedback-checks.json'), JSON.stringify(reports, null, 2) + '\n')
console.log('Browser capture complete: ' + assets)
