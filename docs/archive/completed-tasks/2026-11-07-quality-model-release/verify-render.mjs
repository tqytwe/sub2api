// Offline real-component evidence. No app server, production access or provider calls.
// QUALITY_PLAYWRIGHT_MODULE=/path/to/playwright-core/index.mjs node <this file>
import assert from 'node:assert/strict'
import { dirname, resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { writeFile } from 'node:fs/promises'
const root=resolve(dirname(fileURLToPath(import.meta.url)),'../../../..')
const frontend=resolve(root,'frontend')
process.chdir(frontend)
const {build}=await import(pathToFileURL(resolve(frontend,'node_modules/vite/dist/node/index.js')))
const {default:vue}=await import(pathToFileURL(resolve(frontend,'node_modules/@vitejs/plugin-vue/dist/index.mjs')))
assert(process.env.QUALITY_PLAYWRIGHT_MODULE)
const {chromium}=await import(pathToFileURL(process.env.QUALITY_PLAYWRIGHT_MODULE))
const fixture=`
import {createApp,h} from 'vue'
import {createI18n} from 'vue-i18n'
import View from '/src/views/admin/AccountQualityView.vue'
import zh from '/src/i18n/locales/zh/admin/resources.ts'
import en from '/src/i18n/locales/en/admin/resources.ts'
import zhCommon from '/src/i18n/locales/zh/common.ts'
import enCommon from '/src/i18n/locales/en/common.ts'
import '/src/style.css'
const i18n=createI18n({legacy:false,locale:'zh',fallbackLocale:'en',missingWarn:false,fallbackWarn:false,messages:{zh:{admin:zh,...zhCommon},en:{admin:en,...enCommon}}})
createApp({render:()=>h(View)}).use(i18n).mount('#app')
window.reviewLocale=locale=>i18n.global.locale.value=locale
`
const fakeAPI=`
const cfg={model_ids:['sol','astra','missing'],prompt:'Synthetic probe',quality:{action:'isolate_models',remove_group_ids:[10],auto_restore:true,expected_answer:'ok',judge:{kind:'expected'}}}
const plan={id:9,account_id:7,model_id:'sol',cron_expression:'*/30 * * * *',enabled:true,max_results:100,updated_at:'2026-11-07T00:00:00Z',pelican_config:cfg}
const view={plan_id:9,account_id:7,rule_version:plan.updated_at,fence:'a'.repeat(64),action:'isolate_models',state_present:true,restore_ready:false,reconcile_ready:false,restore_blocked_reason:'orphan_marker',model_removals:[{model:'sol',isolation_model:'sol',ready:true,effect:'remove_orphan_marker_only'},{model:'astra',isolation_model:'astra',ready:false,effect:'remove_matching_marker',blocked_reason:'changed_same_owner'},{model:'missing',isolation_model:'missing',ready:true,effect:'remove_missing_target'}]}
window.fixturePlans=[plan];window.fixtureView=view;window.fixtureCalls=[];window.fixtureMode='normal';
const delay=()=>new Promise(resolve=>setTimeout(resolve,120))
export const adminAPI={accounts:{list:async()=>({items:[{id:7,name:'Synthetic quality account'}]})},groups:{getAllIncludingInactive:async()=>[{id:10,name:'Synthetic quarantine group'}]},scheduledTests:{ownershipPreview:async()=>{await delay();if(window.fixtureMode==='preview-error')throw new Error('Synthetic preview unavailable');return structuredClone(window.fixtureView)},removeOwnedModel:async(id,req)=>{window.fixtureCalls.push(req);await delay();if(window.fixtureMode==='conflict')throw new Error('Synthetic preview changed');window.fixtureView.model_removals=window.fixtureView.model_removals.filter(row=>row.model!==req.model);window.fixturePlans[0].pelican_config.model_ids=window.fixturePlans[0].pelican_config.model_ids.filter(model=>model!==req.model)},create:async()=>{},update:async()=>{}}}
export const listPlans=async()=>structuredClone(window.fixturePlans)
export const listHistory=async()=>({items:[],next_cursor:0})
export const listPresets=async()=>[]
export const listTemplates=async()=>[{id:6,model_id:'sol',cron_expression:'*/30 * * * *',enabled:true,max_results:100,plan_ids:[9],account_filter:{},pelican_config:cfg,created_at:'2026-11-07T00:00:00Z',updated_at:'2026-11-07T00:00:00Z'}]
export const createPreset=async()=>{}
export const createTemplate=async()=>{}
export const updateTemplate=async()=>{}
export const deleteTemplate=async()=>{}
export const listResults=async()=>[]
export const runPlan=async()=>{}
`
const result=await build({configFile:false,root:frontend,logLevel:'error',plugins:[vue(),{name:'quality-review-fixture',enforce:'pre',resolveId(id){
if(id==='virtual:quality-review')return '\0quality-review'
if(id==='@/components/layout/AppLayout.vue'||id===resolve(frontend,'src/components/layout/AppLayout.vue'))return '\0fixture-layout'
if(['@/api/admin','@/api/admin/accountQuality',resolve(frontend,'src/api/admin'),resolve(frontend,'src/api/admin/index.ts'),resolve(frontend,'src/api/admin/accountQuality'),resolve(frontend,'src/api/admin/accountQuality.ts')].includes(id))return '\0fixture-api'
},load(id){if(id==='\0quality-review')return fixture;if(id==='\0fixture-layout')return `import {h} from 'vue';export default {setup(p,{slots}){return()=>h('main',slots.default?.())}}`;if(id==='\0fixture-api')return fakeAPI}}],resolve:{alias:{'@':resolve(frontend,'src'),'vue-i18n':'vue-i18n/dist/vue-i18n.runtime.esm-bundler.js'}},define:{__VUE_OPTIONS_API__:true,__VUE_PROD_DEVTOOLS__:false,__VUE_PROD_HYDRATION_MISMATCH_DETAILS__:false,__INTLIFY_JIT_COMPILATION__:true},build:{write:false,minify:false,rollupOptions:{input:'virtual:quality-review'}}})
const files=new Map(result.output.map(f=>[f.fileName,f.type==='chunk'?f.code:f.source]))
const entry=result.output.find(f=>f.type==='chunk'&&f.isEntry).fileName
const css=result.output.filter(f=>f.fileName.endsWith('.css')).map(f=>`<link rel="stylesheet" href="/${f.fileName}">`).join('')
const html=`<!doctype html><html><head><meta charset="utf-8">${css}</head><body style="padding:16px"><p>Real Vue component / synthetic data / not production</p><div id="app"></div><script type="module" src="/${entry}"></script></body></html>`
const browser=await chromium.launch({headless:true,args:['--no-sandbox']})
const artifact=resolve(root,'docs/visual-reviews/assets/quality-model-release')
const report=[]
try{
const page=await browser.newPage()
const errors=[];page.on('pageerror',e=>errors.push(e.message))
await page.route('**/*',async route=>{const u=new URL(route.request().url());assert.equal(u.origin,'https://quality-review.invalid');const file=u.pathname.slice(1);if(!file)return route.fulfill({contentType:'text/html',body:html});if(!files.has(file)){errors.push('Unexpected fixture request: '+file);return route.abort()};return route.fulfill({contentType:file.endsWith('.css')?'text/css':'text/javascript',body:files.get(file)})})
await page.goto('https://quality-review.invalid/')
await page.waitForSelector('article')
const clickPreview=async()=>{await page.locator('article').last().getByRole('button').last().click();await page.waitForSelector('[data-testid="quality-model-removals"]')}
await clickPreview()
for(const width of [360,768,1280,1920])for(const theme of ['light','dark'])for(const locale of ['zh','en']){
await page.setViewportSize({width,height:1000});await page.emulateMedia({reducedMotion:'reduce'})
await page.evaluate(({theme,locale})=>{document.documentElement.classList.toggle('dark',theme==='dark');window.reviewLocale(locale)},{theme,locale})
await page.waitForTimeout(30)
const overflow=await page.evaluate(()=>document.documentElement.scrollWidth>window.innerWidth)
assert.equal(overflow,false,`${width}/${theme}/${locale}: overflow`)
assert(await page.locator('[data-testid="quality-remove-owned-model"]').nth(1).isDisabled())
await page.locator('[data-testid="quality-removal-reason"]').first().focus();await page.keyboard.press('Tab')
assert(await page.locator('[data-testid="quality-removal-reason"]').last().evaluate(el=>el===document.activeElement),'disabled buttons should be skipped in keyboard order')
report.push({width,theme,locale,overflow,keyboard:true,reducedMotion:await page.evaluate(()=>matchMedia('(prefers-reduced-motion: reduce)').matches)})
if(locale==='zh'&&(width===360||width===1280))await page.locator('article').last().screenshot({path:resolve(artifact,`ownership-${width}-${theme}.png`)})
}
await page.setViewportSize({width:360,height:1000})
await page.evaluate(()=>window.reviewLocale('zh'))
await page.locator('article').last().getByRole('button').first().click()
await page.waitForSelector('[data-testid="quality-isolation-groups"]')
await page.locator('[data-testid="quality-isolation-groups"]').screenshot({path:resolve(artifact,'groups-360-dark.png')})
await page.locator('[data-testid="quality-template-row"] button').first().click()
await page.locator('[data-testid="quality-template-form"]').screenshot({path:resolve(artifact,'template-360-dark.png')})
// Real mounted component mutation, loading feedback, confirmation and readback.
await page.evaluate(()=>window.fixtureMode='conflict')
await page.locator('[data-testid="quality-removal-reason"]').first().selectOption('retire_model')
page.on('dialog',dialog=>dialog.accept())
await page.locator('[data-testid="quality-remove-owned-model"]').first().click()
await page.waitForFunction(()=>document.body.textContent.includes('Synthetic preview changed'))
assert.equal(await page.evaluate(()=>window.fixtureCalls.length),1)
await page.screenshot({path:resolve(artifact,'conflict-360-dark.png'),fullPage:true})
await page.evaluate(()=>window.fixtureMode='normal')
await page.locator('[data-testid="quality-removal-reason"]').first().selectOption('orphan_marker_recovery')
await page.locator('[data-testid="quality-remove-owned-model"]').first().click()
await page.waitForFunction(()=>window.fixtureCalls.length===2)
await page.waitForFunction(()=>document.querySelectorAll('[data-testid="quality-remove-owned-model"]').length===2)
assert.equal(await page.locator('[data-testid="quality-removal-reason"]').count(),1)
await page.locator('article').last().screenshot({path:resolve(artifact,'released-360-dark.png')})
assert.deepEqual(errors,[])
await writeFile(resolve(artifact,'geometry.json'),JSON.stringify({source:'real compiled Vue + production Tailwind; mocked API and layout; offline fixture only',scenarios:report,interactive:['reason required','blocked action disabled','keyboard order','conflict preserves error and refreshes preview','confirmed release reloads scope and preview'],errors},null,2)+'\n')
console.log(JSON.stringify({scenarios:report.length,interactive:'passed',errors}))
}finally{await browser.close()}
