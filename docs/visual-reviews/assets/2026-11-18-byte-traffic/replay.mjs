import assert from 'node:assert/strict'
import { mkdir, readFile } from 'node:fs/promises'
import { resolve } from 'node:path'
import { pathToFileURL } from 'node:url'

const [mode, dist, out] = process.argv.slice(2)
assert(['baseline', 'updated'].includes(mode) && dist && out, 'Usage: replay.mjs baseline|updated DIST OUT')
const { chromium } = await import(process.env.PLAYWRIGHT_MODULE
  ? pathToFileURL(process.env.PLAYWRIGHT_MODULE).href
  : 'playwright')
const browser = await chromium.launch({ executablePath: process.env.CHROMIUM_PATH, args: ['--no-sandbox'] })
const host = 'traffic-review.invalid'
const routes = process.env.SURFACE ? [process.env.SURFACE] : ['/usage', '/admin/usage', '/admin/ops']
const paged = { items: [], total: 0, pages: 1, page: 1, page_size: 20 }
const totals = {
  total_requests: 126, total_input_tokens: 32149, total_output_tokens: 65601, total_cache_tokens: 0,
  total_cache_creation_tokens: 0, total_cache_read_tokens: 0, total_tokens: 97750,
  total_cost: 0.83, total_actual_cost: 0.72, total_account_cost: 0.68, average_duration_ms: 917,
}
const person = { id: 22, email: 'review@example.invalid', username: 'Review user', status: 'active' }
const keys = [{ id: 7, name: 'Example gateway key', deleted: false }, { id: 9, name: 'Archived key', deleted: true }]
const items = ['/v1/responses', '/v1/chat/completions', '/v1/messages'].map((endpoint, i) => ({
  event_id: 'fixture-event-' + i, user_id: 22, api_key_id: 7, group_id: 4,
  request_id: 'fixture-request-' + i, endpoint, method: 'POST', started_at: '2026-11-18T12:00:00+08:00',
  finished_at: '2026-11-18T12:00:02+08:00', upload_bytes: 10240 + 5120 * i, download_bytes: 102400 - 20480 * i,
  status_code: i === 1 ? 413 : 200, outcome: i === 1 ? 'http_error' : 'completed',
  request_complete: i !== 1, measurement_version: 1,
}))
const trend = [12, 30, 16, 50, 32, 25, 44].map((n, i) => ({
  bucket: '11-' + (12 + i), requests: n, upload_bytes: n * 8192, download_bytes: n * 45056, total_bytes: n * 53248,
}))
const report = {
  upload_bytes: 1712128, download_bytes: 9416704, total_bytes: 11128832, requests: 209,
  start: '2026-11-01T00:00:00+08:00', end: '2026-12-01T00:00:00+08:00',
  recorded_since: '2026-11-12T12:00:00+08:00', timezone: 'Asia/Shanghai',
  measurement_version: 1, page: 1, page_size: 20, has_more: false,
}
const health = {
  status: 'delayed', pending_events: 12, quarantined_events: 0, oldest_pending_at: '2026-11-18T03:58:31Z',
  oldest_pending_age_seconds: 89, delivered_events: 209, delivery_failures: 1, staging_failures: 0,
  persistence_failures: 1, last_delivery_at: '2026-11-18T03:58:31Z', checked_at: '2026-11-18T04:00:00Z',
  queued_events: 10, queue_capacity: 4096, processing_events: 2, dropped_events: 0, queue_dropped_events: 0,
  shutdown_dropped_events: 0, spool_limit_dropped_events: 0, invalid_dropped_events: 0,
  unavailable_dropped_events: 0, worker_panics: 0, worker_running: true, storage_available: true,
}
const at = (i) => '2026-11-18T04:' + (20 + i) + ':00Z'
const points = ['edge-east-01', 'edge-west-02'].flatMap((instance, j) =>
  [34, 58, 38, 95, 77, 68].map((value, i) => ({
    at: at(i), instance, egress_mbps: value + j * 10, tcp_retrans_percent: [0.2, 0.1, null, 0.8, 0.2, 0.4][i],
    qdisc_drops: j ? [0, 1, 3, 1, 0, 0][i] : null, egress_status: 'ok',
    tcp_status: i === 2 ? 'unavailable' : 'ok', qdisc_status: j ? 'ok' : 'unsupported', window_seconds: 60,
  })))
const network = {
  points, body_points: [12, 47, 25, 60, 51, 41].map((n, i) => ({ at: at(i), bytes: n * 32768, samples: n })),
  scope: 'synthetic',
}
const mime = { html: 'text/html', js: 'text/javascript', css: 'text/css', svg: 'image/svg+xml', png: 'image/png', woff2: 'font/woff2', webp: 'image/webp' }

function fixture(path, search, actor) {
  if (path === '/auth/me') return actor
  if (path === '/groups/available') return [{ id: 4, name: 'Example group', platform: 'openai' }]
  if (path === '/settings/public') return { site_name: '极速蹬', allow_user_view_error_requests: true }
  if (path === '/admin/settings') return { ops_monitoring_enabled: true }
  if (path.endsWith('/traffic/health')) return health
  if (path.endsWith('/traffic/keys')) return keys.filter(key => !search.get('q') || key.name.toLowerCase().includes(search.get('q').toLowerCase()))
  if (path.endsWith('/traffic')) {
    const view = search.get('view')
    return { ...report, period: search.get('period'), trend: view === 'details' ? [] : trend, items: view === 'summary' ? [] : items }
  }
  if (path.endsWith('/network-series')) return network
  if (path.endsWith('/search-users')) return [{ ...person, deleted: false }]
  if (path === '/usage/stats' || path === '/admin/usage/stats') return totals
  if (path === '/auth/me') return actor
  if (path === '/groups/available') return [{ id: 4, name: 'Example group', platform: 'openai' }]
  if (path.endsWith('/snapshot-v2')) return { overview: null, throughput_trend: { points: [] }, error_trend: { points: [] } }
  if (path.endsWith('/throughput-trend') || path.endsWith('/error-trend')) return { points: [] }
  if (path.endsWith('/latency-histogram')) return { buckets: [] }
  if (path.endsWith('/error-distribution')) return { items: [] }
  if (path.endsWith('/advanced-settings')) return { auto_refresh_enabled: false }
  if (path.endsWith('/metric-thresholds')) return {}
  if (path.endsWith('/all') || path.endsWith('/alert-events') || path.endsWith('/announcements')) return []
  return paged
}

async function routeCapture(surface, width, height, locale, theme, save = true) {
  const actor = { ...person, id: surface === '/usage' ? person.id : 1, role: surface === '/usage' ? 'user' : 'admin', balance: 16, concurrency: 2 }
  const page = await browser.newPage({ viewport: { width, height }, reducedMotion: 'reduce', colorScheme: theme, timezoneId: 'Asia/Shanghai' })
  const errors = []
  page.on('pageerror', error => errors.push(error.message))
  await page.addInitScript(({ actor, locale, theme }) => {
    localStorage.setItem('auth_token', 'synthetic-browser-fixture')
    localStorage.setItem('auth_user', JSON.stringify(actor))
    localStorage.setItem('sub2api_locale', locale)
    localStorage.setItem('theme', theme)
    localStorage.setItem((actor.role === 'admin' ? 'admin_guide' : 'user_guide') + '_' + actor.id + '_' + actor.role + '_v4_interactive', 'true')
  }, { actor, locale, theme })
  await page.routeWebSocket('**/*', socket => socket.close())
  await page.route('**/*', async route => {
    const { hostname, pathname, searchParams } = new URL(route.request().url())
    if (hostname !== host) return route.abort()
    if (pathname === '/setup/status') return route.fulfill({ json: { code: 0, data: { needs_setup: false } } })
    if (pathname.startsWith('/api/v1/')) {
      return route.fulfill({ json: { code: 0, message: 'success', data: fixture(pathname.slice(7), searchParams, actor) } })
    }
    const file = routes.includes(pathname) ? 'index.html' : pathname.slice(1)
    try {
      return await route.fulfill({ contentType: mime[file.split('.').at(-1)], body: await readFile(resolve(dist, file)) })
    } catch {
      return route.abort()
    }
  })
  try {
    await page.goto('https://' + host + surface + '?lang=' + locale, { waitUntil: 'domcontentloaded', timeout: 60000 })
    page.setDefaultTimeout(60000)
    await page.locator(surface === '/admin/ops' ? 'main' : 'main button').first().waitFor()
    const title = locale === 'zh' ? '流量记录' : 'Traffic records'
    let panel
    if (surface === '/admin/ops') {
      panel = page.getByTestId('ops-network-chart')
      if (mode === 'updated') {
        await panel.getByRole('combobox').waitFor()
        const curve = panel.getByTestId('network-egress').locator('path')
        const first = await curve.getAttribute('d')
        assert(first?.includes('L'), 'Ops egress chart rendered')
        await panel.getByRole('combobox').focus()
        await page.keyboard.press('ArrowDown')
        assert.equal(await panel.getByRole('combobox').inputValue(), 'edge-west-02')
        assert.notEqual(await curve.getAttribute('d'), first)
        assert((await panel.getByTestId('network-bodyBytes').locator('path').getAttribute('d')).includes('L'))
        await panel.scrollIntoViewIfNeeded()
      } else {
        await page.waitForResponse(response => response.url().includes('/dashboard/snapshot-v2')).catch(() => {})
        assert.equal(await panel.count(), 0)
      }
    } else if (mode === 'updated') {
      const tab = page.getByRole('button', { name: title, exact: true })
      await tab.focus()
      await page.keyboard.press('Enter')
      panel = page.getByRole('region', { name: title })
      await panel.getByText(locale === 'zh' ? '请求体未完整读取' : 'Request body partially read').waitFor()
      const canvas = panel.locator('canvas')
      await canvas.waitFor()
      await page.waitForFunction(() => {
        const canvas = document.querySelector('main section[aria-busy] canvas')
        if (!canvas?.width || !canvas.height) return false
        const pixels = canvas.getContext('2d').getImageData(0, 0, canvas.width, canvas.height).data
        for (let i = 0; i < pixels.length; i += 4) {
          if (pixels[i] === 14 && pixels[i + 1] === 165 && pixels[i + 2] === 233 && pixels[i + 3] === 255) return true
        }
        return false
      })
      if (surface === '/admin/usage') {
        await panel.getByTestId('traffic-health').getByText(locale === 'zh' ? /存在积压/ : /Delayed/).waitFor()
      } else {
        const key = panel.locator('#traffic-key')
        await key.focus()
        await page.keyboard.press('Enter')
        await page.getByText('Archived key', { exact: false }).waitFor()
        await page.keyboard.press('Escape')
        assert.equal(await key.getAttribute('aria-expanded'), 'false')
      }
      const details = panel.locator('details summary')
      await details.focus()
      await page.keyboard.press('Space')
      assert(await details.locator('..').evaluate(element => element.open))
      await page.keyboard.press('Space')
      await panel.evaluate(element => element.scrollIntoView({ block: 'start' }))
    } else {
      assert.equal(await page.getByRole('button', { name: title, exact: true }).count(), 0)
    }
    assert(await page.evaluate(() => matchMedia('(prefers-reduced-motion: reduce)').matches))
    assert.equal(await page.evaluate(() => document.documentElement.classList.contains('dark')), theme === 'dark')
    const overflow = await page.evaluate(() => ({
      document: document.documentElement.scrollWidth - innerWidth,
      main: document.querySelector('main').scrollWidth - document.querySelector('main').clientWidth,
    }))
    // Existing English ModelDistributionChart buttons overflow by 19px at 360px on #433.
    const allowance = surface === '/admin/usage' && width === 360 && locale === 'en' ? 19 : 1
    assert(overflow.document <= allowance && overflow.main <= allowance, surface + ' overflow at ' + width + ' ' + JSON.stringify(overflow))
    if (allowance > 1) console.log('Known baseline overflow', surface, width, overflow)
    if (panel && mode === 'updated') assert(await panel.evaluate(element => element.scrollWidth <= element.clientWidth + 1), 'traffic/network panel contains its overflow')
    if (mode === 'updated' && surface !== '/admin/ops' && width <= 390) {
      assert(await panel.getByRole('table', { name: locale === 'zh' ? '请求明细' : 'Request records' })
        .evaluate(table => table.parentElement.scrollWidth > table.parentElement.clientWidth), 'request table can scroll locally')
    }
    assert.deepEqual(errors, [], surface + ' browser exceptions')
    assert.equal(await page.locator('.driver-popover').count(), 0, 'fixture onboarding completed')
    if (save) {
      const file = [mode, surface.slice(1).replace('/', '-'), width, locale, theme].join('-') + '.png'
      await page.evaluate(() => scrollTo({ top: 0, left: 0, behavior: 'instant' })); await page.waitForFunction(() => scrollY === 0); await page.screenshot({ path: resolve(out, file), animations: 'disabled', fullPage: true })
      console.log(file)
    }
  } catch (error) {
    console.error(surface, width, locale, theme, errors, await page.locator('main').innerText()); throw error
  } finally {
    await page.close()
  }
}

try {
  await mkdir(out, { recursive: true })
  if (mode === 'baseline') {
    for (const surface of routes) await routeCapture(surface, 1280, 900, 'en', 'light'); await routeCapture('/admin/usage', 360, 844, 'en', 'dark')
  } else {
    for (const surface of routes) {
      await routeCapture(surface, 1280, 900, 'en', 'light')
      await routeCapture(surface, 390, 844, 'zh', 'dark')
      await routeCapture(surface, 360, 844, 'en', 'dark', false)
      await routeCapture(surface, 768, 900, 'zh', 'light', false)
      await routeCapture(surface, 1920, 1080, 'en', 'dark', false)
    }
  }
} finally {
  await browser.close()
}
