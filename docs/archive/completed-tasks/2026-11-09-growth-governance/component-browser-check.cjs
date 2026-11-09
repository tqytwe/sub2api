// Isolated actual Vue component with deterministic API responses; no server or login.
const fs = require('node:fs');
const { createRequire } = require('node:module');
const root = process.cwd();
const req = createRequire(root + '/frontend/package.json');
const { parse, compileScript } = req('vue/compiler-sfc');
const esbuild = createRequire(req.resolve('vite'))('esbuild');
const postcss = req('postcss');
const tailwind = req('tailwindcss');
const { chromium } = require('/data/cache/npm/_npx/e41f203b7505f1fb/node_modules/playwright');
const mode = process.argv[2] || 'updated';
const componentPath = root + '/frontend/src/components/admin/play/GrowthGovernanceOperations.vue';
(async () => {
  let source = fs.readFileSync(componentPath, 'utf8');
  // Prototype only: apply the proposed null guard and move the independent status section.
  if (mode === 'prototype') {
    source = source.replace('item.blocking_reasons.length', 'item.blocking_reasons?.length');
    const start = source.indexOf('    <section class="card overflow-hidden" aria-labelledby="growth-reward-readiness-title">');
    const end = source.indexOf('    </section>', start) + '    </section>'.length;
    const section = source.slice(start, end);
    source = source.slice(0, start) + source.slice(end);
    source = source.replace('    <div class="grid gap-4 xl:grid-cols-2">', section + '\n\n    <div class="grid gap-4 xl:grid-cols-2">');
  }
  const compiled = compileScript(parse(source).descriptor, { id: 'governance-fixture', inlineTemplate: true });
  const localeCode = await esbuild.build({ entryPoints: [root + '/frontend/src/i18n/locales/zh/admin/playOps.ts'], bundle: true, write: false, format: 'cjs', platform: 'node' });
  const moduleBox = { exports: {} };
  new Function('module', 'exports', localeCode.outputFiles[0].text)(moduleBox, moduleBox.exports);
  const locale = (moduleBox.exports.default || moduleBox.exports).playOps;
  const cohort = { window_start:'2026-10-01T00:00:00Z', window_end:'2026-10-15T00:00:00Z', metrics_available:false, participation_users:20, real_call_7d_users:4, real_call_7d_ratio:0.2, real_call_30d_users:6, real_call_30d_ratio:0.3, first_recharge_users:3, first_recharge_ratio:0.15, coupons_issued:10, coupons_redeemed:3, coupon_redemption_ratio:0.3, actual_reward_cost:2, d7_retained_users:4, d7_retention_ratio:0.2, abnormal_redemption_users:0, abnormal_redemption_ratio:null, appeal_count:0, false_positive_appeals:0, appeal_false_positive_ratio:null, unavailable_metrics:['abnormal_redemption_ratio','appeal_false_positive_ratio'] };
  const readiness = [{activity:'checkin',enabled:true,ready:true,blocking_reasons:null,coupon_pool_ready:true,governance_available:true,governance_decision:'not_required'}, {activity:'blindbox',enabled:true,ready:false,blocking_reasons:['coupon_pool_unavailable'],coupon_pool_ready:false,governance_available:true,governance_decision:'not_required',blindbox_cost:0.1,blindbox_daily_limit:3}];
  const stubs = {
    '@/api/admin/play': `const cohort=${JSON.stringify(cohort)};const readiness=${JSON.stringify(readiness)};export default {getGrowthCohort:async()=>cohort,getGrowthGovernance:async()=>({decision:'none',approved:false,cohort}),getRewardReadiness:async()=>readiness}`,
    '@/stores': 'export const useAppStore=()=>({showError(){},showSuccess(){}})',
    '@/composables/useStepUp': 'export const useStepUp=()=>({});export const isStepUpBlocked=()=>false;export const isStepUpCancelled=()=>false;export const stepUpBlockReason=()=>"";',
    '@/utils/apiError': 'export const extractI18nErrorMessage=(e,t,p,f)=>f;',
    'vue-i18n': `const messages=${JSON.stringify(locale)};export const useI18n=()=>({locale:{value:'zh-CN'},t:key=>{let k=key.replace(/^admin\\.playOps\\./,'');return k.split('.').reduce((o,p)=>o?.[p],messages)||key}});`,
  };
  const plugin = {name:'component-fixture',setup(build){
    build.onResolve({filter:/^(@\/|vue-i18n$)/}, args=>({path:args.path,namespace:'fixture'}));
    build.onLoad({filter:/.*/,namespace:'fixture'}, args=>({contents:stubs[args.path] || 'export default {render:()=>null}',loader:'js',resolveDir:root+'/frontend'}));
  }};
  const bundle = await esbuild.build({stdin:{contents:compiled.content.replace('export default', 'const Component =')+'\nimport {createApp} from "vue";createApp(Component).mount("#app");',loader:'ts',resolveDir:root+'/frontend'},bundle:true,write:false,format:'iife',plugins:[plugin],define:{'process.env.NODE_ENV':'"production"'}});
  const cssSource=fs.readFileSync(root+'/frontend/src/style.css','utf8').replace("@import './styles/tokens.css';",fs.readFileSync(root+'/frontend/src/styles/tokens.css','utf8'));
  const config=(await import('file://'+root+'/frontend/tailwind.config.js')).default;
  config.content=[{raw:source,extension:'vue'}];
  const css=(await postcss([tailwind(config)]).process(cssSource,{from:root+'/frontend/src/style.css'})).css;
  const browser=await chromium.launch({headless:true,executablePath:'/root/.cache/ms-playwright/chromium_headless_shell-1194/chrome-linux/headless_shell',args:['--no-sandbox']});
  try {
    const page=await browser.newPage();
    await page.route('**/*',r=>r.abort());
    const errors=[];page.on('pageerror',e=>errors.push(e.message));
    const scenarios=mode==='updated'?[[360,'light'],[768,'light'],[1280,'light'],[1920,'light'],[360,'dark'],[1280,'dark']]:[[1280,'light']];
    for(const [width,theme] of scenarios){
      errors.length=0;
      await page.setViewportSize({width,height:900});
      await page.setContent(`<!doctype html><html class="${theme==='dark'?'dark':''}"><meta charset="utf-8"><style>${css}</style><body><main style="padding:24px" id="app"></main><script>${bundle.outputFiles[0].text}</script></body></html>`);
      await page.waitForTimeout(150);
      const count=await page.locator('[aria-labelledby="growth-reward-readiness-title"] article').count();
      const overflow=await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth);
      await page.screenshot({path:`${root}/docs/visual-reviews/assets/2026-11-09-governance-${mode}-${width}-${theme}.png`,fullPage:true});
      console.log(JSON.stringify({mode,width,theme,rewardStatuses:count,overflow,errors:[...errors]}));
      if(mode!=='baseline'&&(count!==2||overflow||errors.length))throw new Error('Component render regression');
    }
  } finally {await browser.close();}
})().catch(e=>{console.error(e.stack);process.exitCode=1;});
