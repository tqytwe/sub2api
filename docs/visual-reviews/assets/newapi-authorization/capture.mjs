import assert from 'node:assert/strict'
import { resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { mkdir, writeFile } from 'node:fs/promises'

const root = resolve(fileURLToPath(new URL('../../../../', import.meta.url)))
const frontend = resolve(root, 'frontend')
process.chdir(frontend)
const { build } = await import(pathToFileURL(resolve(frontend, 'node_modules/vite/dist/node/index.js')))
const { default: vue } = await import(pathToFileURL(resolve(frontend, 'node_modules/@vitejs/plugin-vue/dist/index.mjs')))
const playwrightModule = process.env.NEWAPI_REVIEW_PLAYWRIGHT_MODULE || '/data/cache/npm/_npx/e41f203b7505f1fb/node_modules/playwright-core/index.mjs'
const { chromium } = await import(playwrightModule)
const entry = `
import {createApp,h,nextTick,ref} from 'vue';import {createI18n} from 'vue-i18n';
import zh from '/src/i18n/locales/zh/admin/accounts.ts';import en from '/src/i18n/locales/en/admin/accounts.ts';
import Dialog from '/src/components/account/NewAPIUpstreamConfigDialog.vue';import '/src/style.css';
const common={loading:'Loading',cancel:'Cancel',confirm:'Confirm',close:'Close'};
const i18n=createI18n({legacy:false,locale:'en',fallbackLocale:'en',missingWarn:false,fallbackWarn:false,messages:{zh:{admin:zh,common:{loading:'加载中',cancel:'取消',confirm:'确认',close:'关闭'}},en:{admin:en,common}}});
const key=ref(0);const show=ref(true);window.reviewScenario='default';
window.setReview=async(locale,theme,scenario='default')=>{i18n.global.locale.value=locale;document.documentElement.classList.toggle('dark',theme==='dark');window.reviewScenario=scenario;key.value++;show.value=true;await nextTick()};
createApp({render:()=>h(Dialog,{key:key.value,show:show.value,account:{id:7,name:'Synthetic account',type:'apikey',platform:'openai'},onClose:()=>{show.value=false}})}).use(i18n).mount('#app');
`
const api = `
const config={account_id:7,site_url:'https://upstream.example/tenant',configured:true,user_id:42,encryption_key_configured:true,accounts:[{account_id:7,name:'Synthetic account',configured_user_id:42},{account_id:8,name:'Second synthetic account',configured_user_id:42}]};
export const getNewAPIUpstreamConfig=async()=>({...config,encryption_key_configured:window.reviewScenario!=='disabled'});
export const previewNewAPIUpstreamConfig=async()=>{if(window.reviewScenario==='error')throw {code:'NEW_API_VERIFICATION_FAILED'};return {site_url:config.site_url,user_id:42,wallet:{amount:12.34,unit:'USD'},accounts:[7,8].map(id=>({account_id:id,name:id===7?'Synthetic account':'Second synthetic account',matched:true,token_id:id+100,group:'default',rate:0.5,token_options:[{token_id:id+100,name:'Synthetic token',group:'default',masked_key:'abcd**********wxyz'}]}))}};
export const saveNewAPIUpstreamConfig=previewNewAPIUpstreamConfig;
export const deleteNewAPIUpstreamConfig=async()=>({account_id:7,configured:false});
`
const output = await build({configFile:false,root:frontend,logLevel:'error',plugins:[{name:'newapi-review',enforce:'pre',resolveId(id){if(id==='virtual:review')return '\0review';if(id==='@/api/admin/accounts'||id===resolve(frontend,'src/api/admin/accounts'))return '\0api'},load(id){if(id==='\0review')return entry;if(id==='\0api')return api}},vue()],resolve:{alias:{'@':resolve(frontend,'src'),'vue':resolve(frontend,'node_modules/vue/dist/vue.runtime.esm-bundler.js'),'vue-i18n':resolve(frontend,'node_modules/vue-i18n/dist/vue-i18n.runtime.esm-bundler.js')}},define:{__VUE_OPTIONS_API__:true,__VUE_PROD_DEVTOOLS__:false,__VUE_PROD_HYDRATION_MISMATCH_DETAILS__:false,__INTLIFY_JIT_COMPILATION__:true},build:{write:false,minify:false,rollupOptions:{input:'virtual:review'}}})
const files=new Map(output.output.map(file=>[file.fileName,file.type==='chunk'?file.code:file.source]))
const script=output.output.find(file=>file.type==='chunk'&&file.isEntry).fileName
const css=output.output.filter(file=>file.fileName.endsWith('.css')).map(file=>'<link rel="stylesheet" href="/'+file.fileName+'">').join('')
const html='<!doctype html><html><head><meta charset="utf-8">'+css+'</head><body><p class="p-2 text-xs text-gray-500">Actual compiled Vue dialog · synthetic APIs · development evidence</p><div id="app"></div><script type="module" src="/'+script+'"></script></body></html>'
const artifacts=resolve(root,'docs/visual-reviews/assets/newapi-authorization')
await mkdir(artifacts,{recursive:true})
const browser=await chromium.launch({headless:true,executablePath:process.env.NEWAPI_REVIEW_CHROMIUM||'/root/.cache/ms-playwright/chromium-1194/chrome-linux/chrome',args:['--no-sandbox']})
try {
  const page=await browser.newPage();const errors=[];const reports=[]
  page.on('pageerror',error=>errors.push(error.message))
  await page.route('**/*',route=>{const url=new URL(route.request().url());assert.equal(url.origin,'https://newapi-review.invalid');const name=url.pathname.slice(1);if(!name)return route.fulfill({contentType:'text/html',body:html});const value=files.get(name);if(value===undefined)return route.abort();return route.fulfill({contentType:name.endsWith('.css')?'text/css':'text/javascript',body:value})})
  await page.goto('https://newapi-review.invalid/')
  await page.waitForFunction(()=>typeof window.setReview==='function')
  for(const width of [360,768,1280,1920])for(const theme of ['light','dark'])for(const locale of ['en','zh']) {
    await page.setViewportSize({width,height:900});await page.emulateMedia({reducedMotion:'reduce'});await page.evaluate(args=>window.setReview(...args),[locale,theme]);await page.locator('[data-testid="new-api-preview"]').click();await page.waitForTimeout(60)
    reports.push({width,theme,locale,overflow:await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth)})
    await page.screenshot({path:resolve(artifacts,`component-${locale}-${width}-${theme}.png`),fullPage:true})
  }
  await page.setViewportSize({width:360,height:900});await page.evaluate(()=>window.setReview('en','light','error'));await page.locator('[data-testid="new-api-preview"]').click();await page.locator('[role="alert"]').waitFor();await page.screenshot({path:resolve(artifacts,'component-error-360.png'),fullPage:true})
  await page.evaluate(()=>window.setReview('en','light','disabled'));assert.equal(await page.locator('[data-testid="new-api-save"]').isDisabled(),true);await page.screenshot({path:resolve(artifacts,'component-disabled-360.png'),fullPage:true})
  await page.evaluate(()=>window.setReview('en','light'));await page.locator('[data-testid="new-api-remove"]').click();await page.locator('[data-testid="new-api-confirm-remove"]').waitFor();await page.screenshot({path:resolve(artifacts,'component-unlink-360.png'),fullPage:true})
  await page.locator('[data-testid="new-api-save"]').focus();await page.keyboard.press('Tab');assert.equal(await page.locator('button[aria-label="Close"]').evaluate(el=>el===document.activeElement),true)
  await page.keyboard.press('Shift+Tab');assert.equal(await page.locator('[data-testid="new-api-save"]').evaluate(el=>el===document.activeElement),true)
  await page.locator('[data-testid="new-api-access-token"]').fill('synthetic-secret-must-clear');await page.keyboard.press('Escape');await page.locator('[role="dialog"]').waitFor({state:'detached'})
  await page.evaluate(()=>window.setReview('en','light'));assert.equal(await page.locator('[data-testid="new-api-access-token"]').inputValue(),'')
  await page.setViewportSize({width:1280,height:900});await page.evaluate(()=>{document.documentElement.style.zoom='2'});await page.locator('[data-testid="new-api-preview"]').click()
  assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false)
  await page.screenshot({path:resolve(artifacts,'component-200-percent.png'),fullPage:true})
  assert.deepEqual(errors,[]);assert.equal(reports.some(row=>row.overflow),false)
  await writeFile(resolve(artifacts,'capture-results.json'),JSON.stringify({reports,errors,keyboard:'Tab and Shift+Tab wrap within dialog; Escape closes; reopen clears token',zoom:'200% has no horizontal overflow; preview action reachable',source:'actual compiled component with synthetic APIs; no server or upstream calls'},null,2))
  console.log(JSON.stringify({scenarios:reports.length,errors,overflow:false}))
} finally {await browser.close()}
