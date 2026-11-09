// Real isolated components, repository CSS, synthetic records, no server/login.
const fs=require('node:fs');const {execFileSync}=require('node:child_process');const {createRequire}=require('node:module');
const root=process.cwd(),req=createRequire(root+'/frontend/package.json');
const {parse,compileScript}=req('vue/compiler-sfc');const esbuild=createRequire(req.resolve('vite'))('esbuild');const postcss=req('postcss'),tailwind=req('tailwindcss');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'/data/cache/npm/_npx/e41f203b7505f1fb/node_modules/playwright');
const proposal=require('./proposal.cjs');const mode=process.argv[2]||'updated';
const surfaces={checkin:'frontend/src/views/user/CheckInView.vue',hub:'frontend/src/views/user/PlayHubView.vue',admin:'frontend/src/components/admin/play/AdminCheckinRecords.vue'};
(async()=>{
 const localeCode=await esbuild.build({entryPoints:[root+'/frontend/src/i18n/locales/jisudeng-pages.zh.ts'],bundle:true,write:false,format:'cjs',platform:'node'});const box={exports:{}};new Function('module','exports',localeCode.outputFiles[0].text)(box,box.exports);const messages=box.exports.default||box.exports.jisudengPagesZh||box.exports;if(mode==='prototype')Object.assign(messages.checkin,proposal.messages.zh);messages.common={refresh:'刷新',retry:'重试'};
 const eligibility={tier:'explorer',reward_mode:'energy',primary_reason:'no_recent_activity',email_verified:true,account_age_days:8,has_recent_usage:false,net_balance_recharge_30d:0,has_active_subscription:false,progress:{account_age_days:8,minimum_account_age_days:3,net_balance_recharge_30d:0,minimum_recharge_cny:10}};
 const status={enabled:true,eligible:true,checked_in_today:false,reward_amount:mode==='baseline'?0.5:0,server_date:'2026-11-09',streak_count:6,next_milestone_days:mode==='baseline'?7:0,next_milestone_bonus:mode==='baseline'?1:0,growth_energy_enabled:mode==='baseline',redeemable_reward_eligible:false,growth_eligibility:eligibility};
 const hub={any_enabled:true,pending_actions:1,growth:{balance:0,total_recharged:0,payment_enabled:false},campaigns:[],checkin:status};
 const row={id:1,user_id:42,email:'fixture@example.test',date:'2026-11-09',stored_streak:7,observed_streak:7,reward_amount:'0.00000000',ledger_amount:'0.00000000',growth_energy:0,reward_mode:'energy',eligibility_reason:'no_recent_activity',snapshot_id:901,evidence_status:'recorded',created_at:'2026-11-09T00:00:00Z'};
 const report={items:[row,{...row,id:2,user_id:43,growth_energy:1}],total:2,page:1,page_size:20,start_date:'2026-10-11',end_date:'2026-11-09',as_of:'2026-11-09T12:00:00Z'};
 const stubs={
  '@/api/play':`export default {getCheckinStatus:async()=>(${JSON.stringify(status)}),getPlayHub:async()=>(${JSON.stringify(hub)})}`,
  '@/api/admin/play':`export default {listCheckinRecords:async()=>(${JSON.stringify(report)})}`,
  '@/api/auth':'export default {getCurrentUser:async()=>({data:{balance:0}})}',
  '@/api/referralCampaign':'export const listReferralCampaigns=async()=>[];',
  '@/stores/auth':'const user={id:42,balance:0};export const useAuthStore=()=>({user,token:"synthetic",refreshUser:async()=>{}})',
  '@/stores/app':'export const useAppStore=()=>({showError(){},showSuccess(){}})',
  '@/composables/useStepUp':'export const useStepUp=()=>({onCancel(){}});export const isStepUpBlocked=()=>false;export const isStepUpCancelled=()=>false;export const stepUpBlockReason=()=>"";',
  '@/utils/apiError':'export const extractApiErrorCode=()=>"";',
  '@/utils/growthAnalytics':'export const trackQuestCompleteOnce=()=>{};export const trackGrowthEvent=()=>{};',
  '@/utils/featureFlags':'export const FeatureFlags={playCheckin:"checkin"};export const isFeatureFlagEnabled=key=>key==="checkin";',
  '@/utils/vipColors':'export const vipTierBadgeClass=()=>"";',
  '@/utils/playCampaign':'export const resolveCampaignDisplayName=c=>c.name;',
  'vue-router':'export const useRouter=()=>({push(){}});',
  'vue-i18n':`const messages=${JSON.stringify(messages)};export const useI18n=()=>({locale:{value:'zh-CN'},t:(key,params)=>{const v=key.split('.').reduce((o,p)=>o?.[p],messages)||key;return typeof v==='string'?v.replace(/[{]([a-zA-Z]+)[}]/g,(m,k)=>params?.[k]??m):key}});`,
 };
 const allSources=Object.values(surfaces).map(p=>fs.readFileSync(root+'/'+p,'utf8')).join('\n');
 const cssSource=fs.readFileSync(root+'/frontend/src/style.css','utf8').replace("@import './styles/tokens.css';",fs.readFileSync(root+'/frontend/src/styles/tokens.css','utf8'));
 const config=(await import('file://'+root+'/frontend/tailwind.config.js')).default;config.content=[{raw:allSources,extension:'vue'}];
 const css=(await postcss([tailwind(config)]).process(cssSource,{from:root+'/frontend/src/style.css'})).css+'\n'+fs.readFileSync(root+'/frontend/src/styles/growth-world.css','utf8');
 const browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_PATH||'/root/.cache/ms-playwright/chromium_headless_shell-1194/chrome-linux/headless_shell',args:['--no-sandbox']});
 try {for(const [surface,file] of Object.entries(surfaces)){
  let source=mode==='updated'?fs.readFileSync(root+'/'+file,'utf8'):execFileSync('git',['show','4cce2a54d33c8063100cf8aea2f77e46964714b7:'+file],{encoding:'utf8'});if(mode==='prototype')source=proposal.changes[file](source);
  const compiled=compileScript(parse(source).descriptor,{id:'checkin-'+surface,inlineTemplate:true});
  const plugin={name:'fixture',setup(build){build.onResolve({filter:/^(@\/|vue-i18n$|vue-router$)/},args=>({path:args.path,namespace:'fixture'}));build.onLoad({filter:/.*/,namespace:'fixture'},args=>({contents:stubs[args.path]||(args.path.includes('AppLayout')?'import {h} from "vue";export default {setup:(p,{slots})=>()=>h("div",null,slots.default?.())}':'export default {render:()=>null}'),loader:'js',resolveDir:root+'/frontend'}));}};
  const bundle=await esbuild.build({stdin:{contents:compiled.content.replace('export default','const Component =')+'\nimport {createApp} from "vue";createApp(Component).mount("#app");',loader:'ts',resolveDir:root+'/frontend'},bundle:true,write:false,format:'iife',plugins:[plugin],define:{'process.env.NODE_ENV':'"production"'}});
  // AppLayout needs a runtime render function because the production Vue build has no template compiler.
  const page=await browser.newPage();await page.route('**/*',r=>r.abort());const errors=[];page.on('pageerror',e=>errors.push(e.message));
  for(const [width,theme] of (mode==='updated'?[[360,'light'],[768,'light'],[1280,'light'],[1920,'light'],[360,'dark'],[1280,'dark']]:[[1280,'light']])){
   errors.length=0;await page.setViewportSize({width,height:900});await page.setContent(`<!doctype html><html class="${theme==='dark'?'dark':''}"><meta charset="utf-8"><style>${css}</style><body><main id="app" style="padding:24px"></main><script>${bundle.outputFiles[0].text}</script></body></html>`);await page.waitForTimeout(150);
   const text=await page.locator('body').innerText();const overflow=await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth);
   await page.screenshot({path:`${root}/docs/visual-reviews/assets/2026-11-09-participation-${surface}-${mode}-${width}-${theme}.png`,fullPage:true});console.log(JSON.stringify({mode,surface,width,theme,overflow,errors,textPresent:text.length>20}));
   if(errors.length||text.length<20||overflow)throw new Error('Fixture render failure');
  }await page.close();
 }}finally{await browser.close();}
})().catch(e=>{console.error(e.stack);process.exitCode=1});
