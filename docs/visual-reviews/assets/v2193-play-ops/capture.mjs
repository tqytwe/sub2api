// Run with PLAYWRIGHT_CORE_PATH pointing to an installed playwright-core module.
import { dirname, resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { writeFileSync } from 'node:fs'
const { chromium } = await import(pathToFileURL(process.env.PLAYWRIGHT_CORE_PATH).href)
const dir = dirname(fileURLToPath(import.meta.url))
const browser = await chromium.launch({ executablePath: '/root/.cache/ms-playwright/chromium-1194/chrome-linux/chrome', headless: true, args: ['--no-sandbox', '--disable-dev-shm-usage'] })
const checks = []
try {
  for (const width of [360, 768, 1280, 1920]) {
    for (const surface of ['checkin', 'farm']) {
      const page = await browser.newPage({ viewport: { width, height: 1000 }, reducedMotion: 'reduce' })
      const errors = []
      page.on('pageerror', e => errors.push(e.message))
      await page.goto(pathToFileURL(resolve(dir, 'updated.html')).href + '?' + surface)
      if (surface === 'farm') {
        await page.locator('input[type=number]').fill('42')
        await page.getByRole('button', { name: '查询用量' }).click()
        await page.getByRole('button', { name: '预览新旧口径' }).click()
        await page.getByText('20,000', { exact: true }).first().waitFor()
      } else await page.getByText('fixture@example.test').waitFor()
      const overflow = await page.evaluate(() => document.documentElement.scrollWidth > innerWidth)
      if (overflow) throw Error(`${surface} ${width}: document overflow`)
      await page.keyboard.press('Tab')
      const focused = await page.evaluate(() => ({ tag: document.activeElement.tagName, outline: getComputedStyle(document.activeElement).outlineStyle, tabIndex: document.activeElement.tabIndex }))
      if (focused.tabIndex < 0) throw Error('keyboard focus is unreachable')
      await page.screenshot({ path: resolve(dir, `${surface}-${width}-zh-light.png`), fullPage: true })
      if (errors.length) throw Error(errors.join('\n'))
      checks.push({ surface, width, documentOverflow: overflow, focused, pageErrors: errors })
      await page.close()
    }
  }
  for (const state of ['empty', 'error', 'loading']) {
    const page = await browser.newPage({ viewport: { width: 360, height: 800 } })
    await page.goto(pathToFileURL(resolve(dir, 'updated.html')).href + '?checkin&' + state)
    await page.screenshot({ path: resolve(dir, `checkin-${state}-360.png`), fullPage: true })
    if (state === 'error') { await page.getByRole('button', { name: '重试' }).click(); await page.getByRole('alert').waitFor() }
    if (state === 'loading' && !(await page.getByRole('button', { name: '查询中…' }).isDisabled())) throw Error('loading allowed duplicate query')
    checks.push({ surface: 'checkin', state, passed: true })
    await page.close()
  }
  const page = await browser.newPage({ viewport: { width: 1280, height: 1000 }, reducedMotion: 'reduce' })
  await page.goto(pathToFileURL(resolve(dir, 'updated.html')).href + '?farm&en&dark')
  await page.locator('input[type=number]').fill('42')
  await page.getByRole('button', { name: 'Query usage' }).click()
  await page.getByRole('button', { name: 'Preview accounting change' }).click()
  await page.locator('textarea').fill('Reviewed fixture token accounting and payout delta')
  await page.getByRole('checkbox').check()
  await page.getByRole('button', { name: 'Activate for this period' }).click()
  await page.getByRole('status').waitFor()
  await page.screenshot({ path: resolve(dir, 'farm-success-1280-en-dark.png'), fullPage: true })
  checks.push({ surface: 'farm', state: 'success', language: 'en', theme: 'dark', passed: true, note: 'Synthetic write only; authentication dialog not exercised' })
  await page.close()
  for (const width of [360, 768, 1280, 1920]) {
    const page = await browser.newPage({ viewport: { width, height: 1000 }, reducedMotion: 'reduce' })
    const errors = []; page.on('pageerror', error => errors.push(error.message))
    await page.goto(pathToFileURL(resolve(dir, 'updated.html')).href + '?sidebar&collapsed')
    const group = page.getByRole('button', { name: '智能运维', exact: true })
    const duration = await page.locator('aside').evaluate(el => getComputedStyle(el).transitionDuration)
    if (duration.split(',').some(value => parseFloat(value) > 0.1)) throw Error('sidebar ignores reduced motion')
    await group.focus(); await page.keyboard.press('Enter')
    await page.waitForFunction(() => document.querySelector('aside').getBoundingClientRect().width >= 250)
    await page.locator('a[href="/admin/account-quality"]').waitFor()
    if (await group.getAttribute('aria-expanded') !== 'true') throw Error('group did not expand')
    await page.screenshot({ path: resolve(dir, 'sidebar-' + width + '-zh-light.png'), fullPage: true })
    if (errors.length) throw Error(errors.join('\n'))
    checks.push({ surface: 'sidebar', width, keyboardExpand: true, pageErrors: errors })
    await page.close()
  }
  for (const lang of ['zh', 'en']) {
    const page = await browser.newPage({ viewport: { width: 1280, height: 1000 }, reducedMotion: 'reduce' })
    const errors = []; page.on('pageerror', error => errors.push(error.message))
    await page.goto(pathToFileURL(resolve(dir, 'updated.html')).href + '?ops&' + lang + (lang === 'en' ? '&dark' : ''))
    await page.locator('[data-testid="arena-period-history"]').selectOption('7')
    await page.locator('p[class*="px-5"]').waitFor()
    await page.getByText('Historical winner').waitFor()
    await page.getByText(lang === 'zh' ? '实际到账' : 'Actually credited', {exact:true}).waitFor()
    await page.screenshot({ path: resolve(dir, 'settled-ops-1280-' + lang + '.png'), fullPage: true })
    if (errors.length) throw Error(errors.join('\n'))
    checks.push({ surface: 'settled-ops', width: 1280, language: lang, pageErrors: errors, note:'Other tab components stubbed; no production services' })
    await page.close()
  }
  const opsMobile = await browser.newPage({ viewport: { width: 360, height: 900 }, reducedMotion: 'reduce' })
  await opsMobile.goto(pathToFileURL(resolve(dir, 'updated.html')).href + '?ops&zh')
  await opsMobile.locator('[data-testid="arena-period-history"]').selectOption('7')
  await opsMobile.locator('p[class*="px-5"]').waitFor()
  if (await opsMobile.evaluate(() => document.documentElement.scrollWidth > innerWidth)) throw Error('settled ops mobile document overflow')
  await opsMobile.screenshot({ path: resolve(dir, 'settled-ops-history-360-zh.png'), fullPage: true })
  checks.push({ surface: 'settled-ops-history', width: 360, language: 'zh', documentOverflow: false })
  await opsMobile.close()
  for (const width of [360, 768, 1280, 1920]) {
    const page = await browser.newPage({ viewport: { width, height: 1000 }, reducedMotion: 'reduce' })
    const errors = []; page.on('pageerror', error => errors.push(error.message))
    await page.goto(pathToFileURL(resolve(dir, 'updated.html')).href + '?public-farm')
    await page.locator('[data-testid="arena-energy-breakdown"]').waitFor()
    const breakdown = await page.locator('[data-testid="arena-energy-breakdown"]').textContent()
    if (!breakdown.includes('12') || !breakdown.includes('10') || !breakdown.includes('2')) throw Error('missing energy decomposition')
    await page.screenshot({ path: resolve(dir, 'public-farm-' + width + '.png'), fullPage: true })
    checks.push({ surface: 'public-farm', width, breakdown, pageErrors: errors })
    if (errors.length) throw Error(errors.join('\n'))
    await page.close()
  }
  for (const width of [360, 1280]) {
    const page = await browser.newPage({ viewport: { width, height: 1000 }, reducedMotion: 'reduce' })
    await page.goto(pathToFileURL(resolve(dir, 'updated.html')).href + '?checkin')
    await page.locator('[data-testid="checkin-select-1"]').check()
    await page.locator('[data-testid="checkin-preview-repair"]').click()
    await page.getByText('证据完整，可修复').waitFor()
    await page.locator('textarea').fill('已核对原签到资格快照与当日冻结的里程碑规则')
    await page.locator('[data-testid="checkin-repair-confirmed"]').check()
    await page.screenshot({ path: resolve(dir, 'checkin-repair-' + width + '.png'), fullPage: true })
    await page.locator('[data-testid="checkin-confirm-repair"]').click()
    await page.getByText('修复已完成，独立调整与操作者证据已保存。').waitFor()
    checks.push({ surface: 'checkin-repair', width, passed: true, note: 'Synthetic API and step-up; actual payment tested separately in PostgreSQL' })
    await page.close()
  }
  writeFileSync(resolve(dir, 'browser-checks.json'), JSON.stringify(checks, null, 2) + '\n')
} finally { await browser.close() }
