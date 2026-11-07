// Component rendering evidence only; never connects to production or starts a server.
// MONITOR_PLAYWRIGHT_MODULE=/absolute/path/to/playwright-core/index.mjs node <this file> [--baseline]
import assert from 'node:assert/strict'
import { dirname, resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { writeFile } from 'node:fs/promises'
import { execFileSync } from 'node:child_process'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../../../..')
const frontend = resolve(root, 'frontend')
process.chdir(frontend)
const { build } = await import(pathToFileURL(resolve(frontend, 'node_modules/vite/dist/node/index.js')))
const { default: vue } = await import(pathToFileURL(resolve(frontend, 'node_modules/@vitejs/plugin-vue/dist/index.mjs')))
assert(process.env.MONITOR_PLAYWRIGHT_MODULE, 'Set MONITOR_PLAYWRIGHT_MODULE to an installed Playwright module')
const { chromium } = await import(pathToFileURL(process.env.MONITOR_PLAYWRIGHT_MODULE))
const baseline = process.argv.includes('--baseline')
const artifactDir = resolve(root, 'docs/visual-reviews/assets/monitor-status-bars')
const entry = `
import { createApp, h, reactive } from 'vue'
import { createI18n } from 'vue-i18n'
import MonitorStatusCards from '/src/features/channel-monitor-v2/MonitorStatusCards.vue'
import zh from '/src/i18n/locales/zh/channelMonitorV2.ts'
import en from '/src/i18n/locales/en/channelMonitorV2.ts'
import '/src/style.css'
const states = ['healthy', 'warning', 'critical', 'unknown']
const metric = {has_samples:true, request_count:0, error_rate:0.08, cache_rate:0.4, ttft:{p50_ms:120}}
const items = states.map((state, i) => ({
  platform:'openai', group_id:i+1, group_name:(i+1)+' / '+state+' / 渠道状态示例', metrics:metric,
  health:{overall:state,minimum_sample:50},
  buckets:Array.from({length:18},(_,n)=>({bucket_start:new Date(Date.UTC(2026,10,7,0,n*5)).toISOString(),metrics:metric,health:{overall:state,minimum_sample:50}})),
}))
items.push({...items[2],group_id:5,group_name:'5 / 样本不足 / low volume',buckets:[{...items[2].buckets[0],metrics:{...metric,request_count:1}}]})
items.push({...items[3],group_id:6,group_name:'6 / 无样本 / no samples',metrics:{...metric,has_samples:false},buckets:[]})
const props = reactive({items,loading:false,now:Date.UTC(2026,10,7,2),timelineMode:'samples',coverage:{requested_start:'2026-11-07T00:00:00Z',requested_end:'2026-11-07T01:30:00Z',bucket_seconds:300}})
const i18n=createI18n({legacy:false,locale:'zh-CN',fallbackLocale:'en',missingWarn:false,fallbackWarn:false,messages:{'zh-CN':zh,en}})
createApp({render:()=>h(MonitorStatusCards,props)}).use(i18n).mount('#app')
window.setReview=(locale,mode)=>{i18n.global.locale.value=locale;props.timelineMode=mode}
`
const output = await build({
  configFile:false, root:frontend, logLevel:'error',
  plugins:[vue(),{name:'monitor-render-fixture',enforce:'pre',resolveId(id){
    if(id==='virtual:monitor-review')return '\0monitor-review'
    if(id==='@/components/user/pelican/PelicanGroupHistoryDialog.vue')return '\0closed-dialog'
  },load(id){
    if(id==='\0monitor-review')return entry
    if(id==='\0closed-dialog')return 'export default {render:()=>null}'
    if(baseline&&['MonitorStatusCards.vue','monitorCards.ts'].some(name=>id===resolve(frontend,'src/features/channel-monitor-v2',name)))return execFileSync('git',['show','01fc395e0306b7af031643e3412c41979afec4be:'+id.slice(root.length+1)],{cwd:root,encoding:'utf8'})
  }}],
  resolve:{alias:{'@':resolve(frontend,'src'),'vue-i18n':'vue-i18n/dist/vue-i18n.runtime.esm-bundler.js'}},
  define:{__VUE_OPTIONS_API__:true,__VUE_PROD_DEVTOOLS__:false,__VUE_PROD_HYDRATION_MISMATCH_DETAILS__:false,__INTLIFY_JIT_COMPILATION__:true},
  build:{write:false,minify:false,rollupOptions:{input:'virtual:monitor-review'}},
})
const files = new Map(output.output.map(file=>[file.fileName,file.type==='chunk'?file.code:file.source]))
const script = output.output.find(file=>file.type==='chunk'&&file.isEntry).fileName
const css = output.output.filter(file=>file.fileName.endsWith('.css')).map(file=>`<link rel="stylesheet" href="/${file.fileName}">`).join('')
const html = `<!doctype html><html><head><meta charset="utf-8">${css}</head><body style="padding:16px"><p style="margin-bottom:16px">Component fixture / synthetic data / ${baseline?'BASELINE':'UPDATED'} / not production</p><div id="app"></div><script type="module" src="/${script}"></script></body></html>`
const browser = await chromium.launch({headless:true,args:['--no-sandbox']})
const report=[]
try {
  const page=await browser.newPage({locale:'zh-CN',timezoneId:'UTC'})
  const errors=[]
  page.on('pageerror',error=>errors.push(error.message))
  page.on('console',message=>{if(message.type()==='error')errors.push(message.text())})
  await page.route('**/*', async route=>{
    const url=new URL(route.request().url())
    assert.equal(url.origin,'https://monitor-review.invalid','Unexpected network request')
    const name=url.pathname.slice(1)
    if(!name)return route.fulfill({contentType:'text/html',body:html})
    assert(files.has(name), 'Unknown fixture asset: '+name)
    return route.fulfill({contentType:name.endsWith('.css')?'text/css':'text/javascript',body:files.get(name)})
  })
  await page.goto('https://monitor-review.invalid/')
  await page.waitForSelector('[data-testid="monitor-group-card"]').catch(async error=>{throw new Error(JSON.stringify({error:error.message,pageErrors:errors,html:await page.content()}))})
  for(const width of baseline?[1280]:[360,768,1280,1920])for(const theme of baseline?['light']:['light','dark'])for(const locale of baseline?['zh-CN']:['zh-CN','en'])for(const mode of baseline?['samples']:['samples','windows']){
    await page.setViewportSize({width,height:1000})
    await page.emulateMedia({reducedMotion:'reduce'})
    await page.evaluate(({theme,locale,mode})=>{document.documentElement.classList.toggle('dark',theme==='dark');window.setReview(locale,mode)}, {theme,locale,mode})
    await page.waitForTimeout(30)
    const cards=await page.locator('[data-testid="monitor-group-card"]').evaluateAll(nodes=>nodes.map(node=>({
      dot:getComputedStyle(node.querySelector('.status-dot')).backgroundColor,
      bars:[...node.querySelectorAll('[role="img"] > span')].map(bar=>({height:bar.getBoundingClientRect().height,width:bar.getBoundingClientRect().width,color:getComputedStyle(bar).backgroundColor,title:bar.title})),
      hasNoSamples:node.querySelector('[role="img"]')===null,
    })))
    const overflow=await page.evaluate(()=>document.documentElement.scrollWidth>window.innerWidth)
    if(baseline){assert.equal(cards[0].bars[0].height,0)}else{
      assert.equal(overflow,false, 'Unexpected horizontal overflow')
      for(const card of cards)for(const bar of card.bars){assert(bar.height>=19&&bar.width>0,'Collapsed sample bar');assert.notEqual(bar.color,'rgba(0, 0, 0, 0)')}
      assert.equal(new Set(cards.slice(0,4).map(card=>card.bars[0].color)).size,4,'Health states must remain distinct')
      assert.equal(new Set(cards.slice(0,4).map(card=>card.dot)).size,4,'Health dots must remain distinct')
      assert(cards.every(card=>card.dot!=='rgba(0, 0, 0, 0)'), 'Invisible health dot')
      if(mode==='samples'){assert.equal(cards[4].bars[0].color,cards[3].bars[0].color,'Low-volume sample must stay unknown');assert(cards[5].hasNoSamples)}
    }
    report.push({width,theme,locale,mode,overflow,cards:cards.map(card=>({
      dot:card.dot,barCount:card.bars.length,heights:[...new Set(card.bars.map(bar=>bar.height))],
      minBarWidth:card.bars.length?Math.min(...card.bars.map(bar=>bar.width)):null,
      colors:[...new Set(card.bars.map(bar=>bar.color))],firstTitle:card.bars[0]?.title,hasNoSamples:card.hasNoSamples,
    }))})
    if(baseline||(locale==='zh-CN'&&mode==='samples'&&(width===360||width===1280)))await page.screenshot({path:resolve(artifactDir,`${baseline?'baseline':'updated'}-${width}-${theme}.png`),fullPage:true})
  }
  assert.deepEqual(errors,[])
  await writeFile(resolve(artifactDir,`${baseline?'baseline':'updated'}-geometry.json`),JSON.stringify({source:'compiled Vue component + production Tailwind CSS; synthetic data; closed dialog stub only',report},null,2)+'\n')
  console.log(JSON.stringify({phase:baseline?'RED collapsed bars reproduced':'GREEN',scenarios:report.length,pageErrors:errors}))
} finally {await browser.close()}
