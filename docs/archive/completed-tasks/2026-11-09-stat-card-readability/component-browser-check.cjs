const fs=require('node:fs');
const path=require('node:path');
const {createRequire}=require('node:module');
// Development-only component fixture; no server or production session is used.
const root=process.env.STAT_CARD_REPO_ROOT || process.cwd();
const requireFrontend=createRequire(root+'/frontend/package.json');
const postcss=requireFrontend('postcss');
const tailwind=requireFrontend('tailwindcss');
const Vue=requireFrontend('vue');
const requireVue=createRequire(requireFrontend.resolve('vue'));
const {parse}=requireFrontend('vue/compiler-sfc');
const {compile}=requireVue('@vue/compiler-dom');
const {renderToString}=requireVue('@vue/server-renderer');
const {chromium}=require(process.env.STAT_CARD_PLAYWRIGHT_MODULE || 'playwright');
(async()=>{
 const mode=process.argv[2]||'baseline';
 const template=parse(fs.readFileSync(root+'/frontend/src/components/common/StatCard.vue','utf8')).descriptor.template.content;
 const render=new Function('Vue',compile(template,{mode:'function'}).code)(Vue);
 const Card={props:['title','value'],setup(p){return {formattedValue:Vue.computed(()=>p.value),change:undefined,icon:undefined,iconClass:'stat-icon-primary'}},render};
 const cards=[['本月已确认绑定邀请','0'],['本月有效首付人数','4'],['本月注册受邀转化率','66.7%'],['待核验（全部）','70']];
 const app=Vue.createSSRApp({render:()=>Vue.h('div',{class:'grid gap-3 sm:grid-cols-2 xl:grid-cols-4'},cards.map(([title,value])=>Vue.h(Card,{title,value})))});
 app.component('Icon',{render:()=>null});
 const markup=await renderToString(app);
 let source=fs.readFileSync(root+'/frontend/src/style.css','utf8').replace("@import './styles/tokens.css';",fs.readFileSync(root+'/frontend/src/styles/tokens.css','utf8'));
 const config=(await import('file://'+root+'/frontend/tailwind.config.js')).default;
 config.content=[{raw:markup,extension:'html'}];
 const base=(await postcss([tailwind(config)]).process(source,{from:root+'/frontend/src/style.css'})).css;
 let home=fs.readFileSync(root+'/frontend/src/styles/home-view.css','utf8');
 if(mode==='prototype'){
  const ast=postcss.parse(home);
  ast.walkRules(r=>{if(r.selector.includes('.stat-value')||r.selector.includes('.stat-label'))r.selector=r.selector.split(',').map(s=>'.home-page '+s.trim()).join(',')});
  home=ast.toString();
 }
 const browser=await chromium.launch({headless:true,executablePath:process.env.STAT_CARD_CHROMIUM_PATH || undefined,args:['--no-sandbox']});
 const page=await browser.newPage();
 await page.route('**/*',route=>route.abort());
 const scenarios=mode==='updated'?[[360,'light'],[768,'light'],[1280,'light'],[1920,'light'],[1280,'dark'],[360,'dark']]:[[1920,'light']];
 for(const [width,theme] of scenarios){
  await page.setViewportSize({width,height:width===360?640:360});
  await page.setContent(`<!doctype html><html class="${theme==='dark'?'dark':''}"><meta charset="utf-8"><style>${base}\n${home}</style><body><main style="padding:24px"><h2 style="margin-bottom:16px">邀请返利运营概览 · ${mode} component fixture</h2>${markup}<section class="home-page" style="margin-top:24px"><div class="stat"><span class="stat-value">123<span class="stat-unit">+</span></span><span class="stat-label">Homepage typography reference</span></div></section></main></body></html>`);
  const metrics=await page.locator('.stat-card p[title]').evaluateAll(els=>els.map(e=>{const s=getComputedStyle(e);return {text:e.textContent,fontSize:parseFloat(s.fontSize),lineHeight:parseFloat(s.lineHeight),fontFamily:s.fontFamily,height:e.getBoundingClientRect().height,scrollWidth:e.scrollWidth,clientWidth:e.clientWidth}}));
  const file=`${root}/docs/visual-reviews/assets/2026-11-09-stat-card-${mode}-${width}-${theme}.png`;
  await page.screenshot({path:file,fullPage:true});
  console.log(JSON.stringify({mode,width,theme,metrics}));
  if(mode==='updated'&&metrics.some(m=>m.fontSize>m.lineHeight||m.fontSize!==24||m.scrollWidth>m.clientWidth))throw new Error('Number clipped or wrong typography');
 }
 await browser.close();
})().catch(e=>{console.error(e.stack);process.exitCode=1});
