import assert from 'node:assert/strict'
import { createRequire } from 'node:module'
import { mkdir } from 'node:fs/promises'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { build } from 'vite'
import vue from '@vitejs/plugin-vue'

const here = dirname(fileURLToPath(import.meta.url))
const frontend = resolve(here, '../..')
process.chdir(frontend)
const require = createRequire(process.env.PLAN_PLAYWRIGHT_PACKAGE || import.meta.url)
const { chromium } = require('@playwright/test')
const origin = process.env.PLAN_CONTRACT_URL
assert.match(origin, /^http:\/\/127\.0\.0\.1:\d+$/)
const baseline = process.env.PLAN_CONTRACT_BASELINE === '1'
const evidence = process.env.PLAN_CONTRACT_EVIDENCE || resolve(frontend, '../docs/visual-reviews/assets/plan-edit-preserve')
await mkdir(evidence, { recursive: true })
await build({
  configFile: false, root: frontend, logLevel: 'silent', plugins: [vue()],
  resolve: { alias: [
    { find: '@/components/layout/AppLayout.vue', replacement: resolve(here, 'Shell.vue') },
    { find: '@', replacement: resolve(frontend, 'src') },
    { find: 'vue-i18n', replacement: 'vue-i18n/dist/vue-i18n.runtime.esm-bundler.js' },
  ] },
  define: { __INTLIFY_JIT_COMPILATION__: true },
  build: { target: 'esnext', outDir: process.env.PLAN_CONTRACT_DIST, emptyOutDir: true, rollupOptions: { input: resolve(here, 'index.html') } },
})
const browser = await chromium.launch({ headless: true })
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 900 } })
  page.setDefaultTimeout(10000)
  page.on('console', message => { if (message.type() === 'error') console.log('Console:', message.text()) })
  page.on('pageerror', error => console.log('Browser error:', error.message))
  page.on('requestfailed', request => console.log('Request failed:', new URL(request.url()).pathname))
  await page.addInitScript(token => localStorage.setItem('auth_token', token), process.env.PLAN_CONTRACT_TOKEN)
  const puts = []
  page.on('request', request => { if (request.method() === 'PUT') puts.push(request.postDataJSON()) })
  const listed = page.waitForResponse(r => r.url().includes('/admin/payment/plans') && r.request().method() === 'GET')
  await page.goto(`${origin}/e2e/plan-edit/index.html`)
  const original = (await (await listed).json()).data[0]
  const edit = page.getByRole('button', { name: 'Edit', exact: true })
  await edit.click()
  const form = page.locator('#plan-form')
  await form.waitFor()
  await page.waitForFunction(() => !document.querySelector('.modal-enter-active, .modal-leave-active'))
  await page.screenshot({ path: resolve(evidence, baseline ? 'before-1280.png' : 'after-1280.png') })
  if (baseline) {
    // The unchanged baseline dialog is the implementation prototype: no layout,
    // controls, theme or new visual pattern is proposed by this data repair.
    await page.screenshot({ path: resolve(evidence, 'prototype-1280.png') })
  } else {
    assert.equal(await form.locator('[data-test="plan-cover-image-url"]').inputValue(), original.cover_image_url)
    assert.equal(await form.locator('[data-test="plan-detail-description"]').inputValue(), original.detail_description)
    assert.equal(await form.locator('[data-test="plan-storefront-badge"]').inputValue(), original.storefront_badge)
  }
  // Cancellation must leave the actual HTTP write count at zero.
  await form.locator('textarea').first().fill('Cancelled description')
  await page.getByRole('button', { name: 'Cancel', exact: true }).click()
  assert.equal(puts.length, 0)
  await edit.click()
  await page.waitForFunction(() => !document.querySelector('.modal-enter-active, .modal-leave-active'))
  assert.equal(await form.locator('textarea').first().inputValue(), original.description)
  for (const width of [360, 768, 1920]) {
    await page.setViewportSize({ width, height: 900 })
    await page.screenshot({ path: resolve(evidence, `${baseline ? 'before' : 'after'}-${width}.png`) })
  }
  await page.emulateMedia({ reducedMotion: 'reduce', colorScheme: 'dark' })
  await page.evaluate(() => document.documentElement.classList.add('dark'))
  await page.keyboard.press('Tab')
  await page.screenshot({ path: resolve(evidence, `${baseline ? 'before' : 'after'}-dark.png`) })
  await form.locator('textarea').first().fill('Browser edited description')
  const saved = page.waitForResponse(r => r.request().method() === 'PUT')
  await page.getByRole('button', { name: 'Save', exact: true }).click()
  assert.equal((await saved).status(), 200)
  assert.equal(puts.length, 1)
  if (!baseline) assert.deepEqual(puts[0], { description: 'Browser edited description' })
  const response = await page.request.get(`${origin}/api/v1/admin/payment/plans`, { headers: { Authorization: `Bearer ${process.env.PLAN_CONTRACT_TOKEN}` } })
  assert.equal(response.status(), 200)
  const after = (await response.json()).data[0]
  for (const [key, value] of Object.entries(baseline ? {} : original)) {
    if (key === 'description' || key === 'updated_at') continue
    assert.deepEqual(after[key], value, `GET after browser PUT: ${key}`)
  }
  console.log(baseline ? 'Baseline browser submitted actual edit; Go will check for DB loss.' : 'Browser: GET → actual view/dialog → cancel/reopen → description-only PUT → GET passed; DB verified by Go parent.')
} finally { await browser.close() }
