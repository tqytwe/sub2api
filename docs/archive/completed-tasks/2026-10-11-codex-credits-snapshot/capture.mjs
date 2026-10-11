// Reproducible isolated component screenshots. No HTTP server or real account API.
// Usage: PLAYWRIGHT_MODULE=/path/to/playwright/index.mjs node capture.mjs ROOT BASELINE
import { createRequire } from 'node:module'
import { pathToFileURL } from 'node:url'
import { mkdtemp, mkdir, writeFile, readFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import path from 'node:path'
const root = path.resolve(process.argv[2])
const baseline = path.resolve(process.argv[3])
const require = createRequire(path.join(root, 'frontend/package.json'))
const { build } = await import(pathToFileURL(path.join(path.dirname(require.resolve('vite/package.json')), 'dist/node/index.js')))
const { default: vue } = await import(pathToFileURL(require.resolve('@vitejs/plugin-vue')))
const { chromium } = await import(pathToFileURL(process.env.PLAYWRIGHT_MODULE))
const scratch = await mkdtemp(path.join(tmpdir(), 'quota-capture-'))
const assets = path.join(root, 'docs/visual-reviews/assets/codex-credits-snapshot')
await mkdir(assets, { recursive: true })
const reports = []
for (const [mode, source] of [['prototype', baseline], ['before', baseline], ['after', root]]) {
  const folder = path.join(scratch, mode)
  await mkdir(folder)
  await writeFile(path.join(folder, 'api.ts'), `export const refreshOpenAIQuota = (id) => window.fixtureQuery(id);
export const resetOpenAIQuota = () => { window.resetCalls++; throw new Error('Reset forbidden in fixture'); };
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
const pending = {};
window.fixtureQuery = (id) => { window.queryCalls.push(id); if (id === 7) return Promise.reject(new Error(language === 'zh' ? '本地模拟：读取失败' : 'Local fixture: read failed')); return new Promise(resolve => { pending[id] = resolve; }); };
const credits = (balance) => balance === null ? null : { has_credits: balance !== '0', unlimited: false, balance };
const account = (id, balance, time = 1791720000) => ({ id, name: 'synthetic-' + id, platform: 'openai', type: 'oauth', parent_account_id: null, extra: { codex_credits_snapshot: { credits: credits(balance), fetched_at: time } } });
const desired = ['1234.50', '42.50', '0', null, '42.50', '1234.50', '1234.50', '12345678901234567890.0123'];
const names = language === 'zh' ? ['初始已知余额', '同账号快照更新', '明确零余额', '未知余额（不是零）', '新快照保留，旧响应迟到', '本地查询等待中', '本地查询失败，保留余额', '高精度十进制余额'] : ['Initial known balance', 'Same-account snapshot update', 'Explicit zero balance', 'Unknown balance (not zero)', 'New snapshot survives old response', 'Local read pending', 'Local read failed; balance retained', 'High precision decimal balance'];
const values = ref(desired.map((value, index) => account(index + 1, '${mode}' === 'prototype' ? value : (index >= 1 && index <= 4 ? '1234.50' : value))));
const i18n = createI18n({ legacy: false, locale: language, messages: { zh: { ...zhCommon, admin: zhAdmin }, en: { ...enCommon, admin: enAdmin } } });
createApp({ setup() { return () => h('main', { class: 'p-4 md:p-8', style: { maxWidth: '960px', margin: '0 auto' } }, [
 h('h1', { class: 'text-xl font-semibold mb-2' }, language === 'zh' ? '账号额度 · 隔离组件验收' : 'Account credits · isolated component review'),
 h('p', { class: 'text-sm text-gray-500 mb-5' }, language === 'zh' ? '合成账号与本地响应；真实组件、词条和样式。非生产截图。' : 'Synthetic accounts and local responses; actual component, translations and styles. Not production.'),
 ...values.value.map((value, index) => h('section', { 'data-row': index + 1, class: 'border-b py-4' }, [h('h2', { class: 'text-sm font-medium mb-2' }, names[index]), h(Cell, { account: value })]))
]); }}).use(i18n).mount('#app');
window.prepare = async () => {
 await nextTick();
 if ('${mode}' !== 'prototype') {
   document.querySelector('[data-row="5"] [data-testid="codex-credits"]').click(); await nextTick();
   for (let i = 1; i <= 4; i++) values.value[i] = account(i + 1, desired[i], 1791720600);
   await nextTick();
   pending[5]({ credits: credits('1234.50'), fetched_at: 1791720000, cache_persisted: true, credits_cache_persisted: true });
   await nextTick(); await nextTick();
 }
 document.querySelector('[data-row="6"] [data-testid="codex-credits"]').click();
 document.querySelector('[data-row="7"] [data-testid="codex-credits"]').click();
 await nextTick(); await nextTick();
};
`)
  await build({ configFile: false, root: folder, plugins: [vue()], resolve: { alias: [
    { find: '@/api/admin/accounts', replacement: path.join(folder, 'api.ts') },
    { find: '@', replacement: path.join(source, 'frontend/src') },
    { find: 'vue', replacement: require.resolve('vue/dist/vue.runtime.esm-bundler.js') },
    { find: 'vue-i18n', replacement: require.resolve('vue-i18n/dist/vue-i18n.esm-bundler.js') }
  ] }, css: { postcss: { plugins: [require('tailwindcss')({ ...require(path.join(root, 'frontend/tailwind.config.js')).default, content: [path.join(source, 'frontend/src/**/*.{vue,js,ts}'), path.join(folder, 'main.ts')] }), require('autoprefixer')()] } }, build: { outDir: path.join(folder, 'dist'), emptyOutDir: true }, logLevel: 'warn' })
  const browser = await chromium.launch({ executablePath: process.env.CHROMIUM_PATH || '/usr/bin/chromium', headless: true, args: ['--no-sandbox'] })
  try {
    const variants = mode === 'prototype' ? [{ width: 1280, language: 'zh', theme: 'light' }] : [360, 768, 1280, 1920].flatMap(width => [{ width, language: 'zh', theme: 'light' }, { width, language: 'en', theme: 'dark' }])
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
      await page.evaluate(() => window.prepare())
      await page.locator('[data-row="7"]').getByText(variant.language === 'zh' ? '本地模拟：读取失败' : 'Local fixture: read failed').waitFor()
      const name = mode + '-' + variant.language + '-' + variant.theme + '-' + variant.width
      await page.screenshot({ path: path.join(assets, name + '.png'), fullPage: true })
      const balances = await page.locator('[data-testid="codex-credits"]').allTextContents()
      if (mode === 'after' && (!balances[1].includes('42.50') || !balances[2].trim().endsWith('0') || !balances[3].includes('—') || !balances[4].includes('42.50'))) throw new Error('Freshness rendering failed: ' + balances)
      if (mode === 'before' && !balances[1].includes('1234.50')) throw new Error('Baseline no longer reproduces stale balance')
      const checks = await page.evaluate(() => ({ queryCalls: window.queryCalls, resetCalls: window.resetCalls, overflow: document.documentElement.scrollWidth > innerWidth, titles: [...document.querySelectorAll('[data-testid="codex-credits"]')].map(el => el.title), reducedMotion: matchMedia('(prefers-reduced-motion: reduce)').matches }))
      if (checks.resetCalls || checks.overflow || external.length || errors.length) throw new Error(JSON.stringify({ checks, external, errors }))
      if (mode === 'after' && variant.width === 1280) {
        await page.keyboard.press('Tab'); await page.keyboard.press('Tab');
        const focus = await page.evaluate(() => ({ focused: document.activeElement?.getAttribute('data-testid'), visible: document.activeElement?.matches(':focus-visible') }))
        await page.screenshot({ path: path.join(assets, name + '-focus.png'), fullPage: true })
        await page.locator('[data-row="2"] [data-testid="codex-credits"]').hover()
        await page.screenshot({ path: path.join(assets, name + '-hover.png'), fullPage: true })
        await page.evaluate(() => { document.documentElement.style.zoom = '2' })
        await page.screenshot({ path: path.join(assets, name + '-zoom200.png'), fullPage: true })
        reports.push({ name, balances, checks, focus, errors, external })
      } else reports.push({ name, balances, checks, errors, external })
      await page.close()
    }
  } finally { await browser.close() }
}
await writeFile(path.join(assets, 'browser-checks.json'), JSON.stringify(reports, null, 2) + '\n')
console.log('Browser capture complete: ' + assets)
