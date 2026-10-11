// Actual compiled frontend, synthetic HTTP routing/auth/ledger contract. NOT real routing PostgreSQL.
const {chromium} = require(process.env.PLAYWRIGHT_MODULE || '/opt/codex/runtimes/cua/lib/node_modules/playwright-core')
const fs=require('node:fs'), path=require('node:path'), http=require('node:http'), assert=require('node:assert/strict')
const root=path.resolve(__dirname,'../../../../'), dist=process.env.ROUTING_CAPTURE_DIST || path.join(root,'backend/internal/web/dist')
const images=path.resolve(__dirname,'../../../visual-reviews/assets/admin-routing-detail')
const AUTH="document.querySelector('#app').__vue_app__.config.globalProperties.$pinia._s.get('auth')"
const ROUTER="document.querySelector('#app').__vue_app__.config.globalProperties.$router"
const id='00000000-0000-4000-8000-000000000001', calls=[], held=[]
let state='complete', count=3, routingHTTP=200, hold=false
const profile=uid=>({id:uid,role:uid>=999?'admin':'user',username:'Synthetic reviewer',email:'fixture@example.invalid',balance:94,status:'active'})
const record={id,parent_id:null,kind:'http',turn_no:null,route:'/v1/responses',method:'POST',user_id:101,api_key_id:201,started_at:'2026-10-11T00:00:00Z',ended_at:'2026-10-11T00:00:01Z',execution_state:'succeeded',usage_state:'known',settlement_state:'settled',error_code:'',http_status:200,attempt_count:1,output_observed:true}
const routing=()=>({request_id:id,coverage_version:state==='not_recorded'?'':'openai_compatible_http_text_v1',status:state,selection_count:1,event_count:count,veto_count:1,omitted_count:state==='truncated'?7:0,incomplete:state==='incomplete',source_population_complete:false,events:['not_recorded','unsupported_coverage'].includes(state)?[]:Array.from({length:count},(_,i)=>({selection_no:1,stage:i===0?'pool_source':i===1?'initial_filter':'selection',reason:i===0?'prefiltered_schedulable_pool':i===1?'model_rate_limited':'selection_selected',...(i?{account_id:234+i}:{}),value:0,at:'2026-10-11T00:00:00Z'}))})
const send=(res,data,status=200)=>{if(res.destroyed)return;res.writeHead(status,{'Content-Type':'application/json','Cache-Control':'no-store'});res.end(JSON.stringify({code:status===200?0:status,data}))}
;(async()=>{
const server=http.createServer(async(req,res)=>{
 const u=new URL(req.url,'http://fixture.test'),uid=Number(String(req.headers.authorization||'').match(/ledger-fixture-(\d+)/)?.[1]||0)
 if(u.pathname.endsWith('/auth/login')) {let body='';for await(const chunk of req)body+=chunk;const email=JSON.parse(body).email,target=Number(email.match(/^fixture-(101|999|1000)(?:-short)?@example.invalid$/)?.[1]);return send(res,{access_token:`ledger-fixture-${target}`,refresh_token:`ledger-refresh-${target}`,expires_in:email.includes('-short@')?123:3600,user:profile(target)})}
 if(u.pathname.endsWith('/auth/refresh'))return send(res,{access_token:'ledger-fixture-999:renewed',refresh_token:'ledger-refresh-999',expires_in:3600,token_type:'Bearer'})
 if(u.pathname.endsWith('/auth/logout'))return send(res,{})
 if(u.pathname.endsWith('/auth/me'))return send(res,profile(uid))
 if(u.pathname.endsWith('/settings/public'))return send(res,{site_name:'极速蹬',custom_menu_items:[]})
 if(u.pathname.includes('/requests')){
  const kind=u.pathname.endsWith('/routing')?'routing':u.pathname.includes('/usage/')?'usage':u.searchParams.get('section')||u.searchParams.get('view')||'list'
  calls.push({uid,kind,path:u.pathname,cursor:u.searchParams.get('cursor')||''})
  if(!uid)return send(res,{},401)
  if(u.pathname.includes('/admin/')&&uid<999)return send(res,{},403)
  if(uid===1000)return send(res,{},503)
  if(kind==='routing'){if(hold){held.push(res);return}return send(res,routing(),routingHTTP)}
  if(kind==='list')return send(res,{items:[record],total:1,page:1,page_size:20})
  if(kind==='summary')return send(res,{request:record})
  if(kind==='usage')return send(res,{id:42,model:'synthetic-text-model',input_tokens:12,output_tokens:8,billed_cost:6,billing_mode:'token'})
  const items=kind==='billing'?[{usage_log_id:42,billed_cost:6,settled:true,applied:true}]:kind==='attempts'?[{attempt_no:1,phase:'request',account_id:uid>=999?234:undefined,upstream_request_id:uid>=999?'req_synthetic_only':undefined,usage_state:'known',execution_state:'succeeded',started_at:record.started_at,ended_at:record.ended_at,output_observed:true,http_status:200,error_code:''}]:[]
  return send(res,{items,page_size:20,has_more:false,next_cursor:'',consistency:'live',refresh_hint:'new_records_late_evidence_and_state_changes'})
 }
 if(u.pathname.startsWith('/api/'))return send(res,{items:[],total:0})
 const file=path.join(dist,path.normalize('/'+u.pathname)),target=fs.existsSync(file)&&fs.statSync(file).isFile()?file:path.join(dist,'index.html')
 res.setHeader('Content-Type',target.endsWith('.js')?'text/javascript':target.endsWith('.css')?'text/css':target.endsWith('.png')?'image/png':target.endsWith('.svg')?'image/svg+xml':'text/html');fs.createReadStream(target).pipe(res)
})
await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));const url=`http://127.0.0.1:${server.address().port}`
const browser=await chromium.launch({executablePath:process.env.CHROMIUM_BIN||'/usr/bin/chromium',headless:true,args:['--no-sandbox']})
const evidence={scope:'actual build + synthetic HTTP contract; routing PG integration pending #359',roles:[],states:[],page_errors:[],viewport_measurements:[]}
try{
 for(const [uid,role]of [[999,'admin'],[101,'user']].filter(([,role])=>!process.env.ROUTING_CAPTURE_ROLE||role===process.env.ROUTING_CAPTURE_ROLE)){
  const ctx=await browser.newContext({viewport:{width:1280,height:900},reducedMotion:'reduce'})
  await ctx.addInitScript(()=>{for(const[id,role]of [[101,'user'],[999,'admin'],[1000,'admin']])localStorage.setItem(`${role}_guide_${id}_${role}_v4_interactive`,'true')})
  const p=await ctx.newPage();p.on('pageerror',e=>evidence.page_errors.push(e.message));await p.goto(url+'/login?lang=en',{waitUntil:'networkidle'})
  const login=async(account,short=false)=>p.evaluate(async({auth,account,short})=>eval(auth).login({email:`fixture-${account}${short?'-short':''}@example.invalid`,password:'synthetic-fixture-only'}),{auth:AUTH,account,short})
  const open=async(lang='en')=>{await p.evaluate(async({router,target})=>eval(router).push(target),{router:ROUTER,target:`${role==='admin'?'/admin':''}/requests?private_id=${id}&lang=${lang}`});await p.getByRole('dialog').waitFor();await p.waitForLoadState('networkidle')}
  const r=p.getByTestId('ledger-section-routing'),bill=p.getByTestId('ledger-section-billing')
  const idle=()=>p.waitForFunction(()=>document.querySelector('[data-testid="ledger-section-routing"]')?.getAttribute('aria-busy')==='false')
  const refresh=async()=>{await r.locator('[data-action=refresh]').click();await idle()}
  const shot=async name=>{if(role==='admin')await r.locator('h3').evaluate(el=>el.scrollIntoView({block:'start'}));await p.screenshot({path:path.join(images,name+'.png'),animations:'disabled'})}
  await login(uid);await open()
  assert.equal(await r.count(),role==='admin'?1:0)
  if(role==='admin'){
   await idle();assert((await r.getByTestId('routing-first-veto').innerText()).includes('235'));assert((await r.innerText()).includes('does not prove forwarding'))
   assert((await r.innerText()).includes('does not prove that the group has no accounts'))
   await bill.getByRole('button',{name:'Usage record #42',exact:true}).click();await p.getByText('synthetic-text-model',{exact:true}).waitFor()
   for(const status of ['complete','truncated','pending','incomplete','not_recorded','unsupported_coverage']){state=status;await refresh();assert.equal(await r.locator(`[data-routing-status=${status}]`).count(),1);await shot(`state-${status}-admin-1280`);evidence.states.push(status)}
   state='complete';count=45;await refresh();const before=calls.length;await r.locator('[data-action=next]').click();assert((await r.getByTestId('routing-page').innerText()).includes('21–40'));assert.equal(calls.length,before)
   routingHTTP=503;await refresh();await r.getByRole('alert').waitFor();assert((await bill.innerText()).includes('synthetic-text-model'));await shot('error-admin-1280')
   routingHTTP=200;count=3;await r.locator('[data-action=retry]').click();await idle();assert.equal(await r.getByRole('alert').count(),0)
   await r.locator('[data-action=refresh]').focus();await shot('focus-admin-1280');await p.keyboard.press('Enter');await idle()
   await r.locator('[data-action=refresh]').hover();await shot('hover-admin-1280');await p.mouse.down();await shot('active-admin-1280');await p.mouse.up();await idle()
   for(const width of [360,768,1280,1920]){await p.setViewportSize({width,height:900});await shot(`after-admin-${width}`);const measure=await p.evaluate(()=>({width:innerWidth,document_width:document.documentElement.scrollWidth,dialog_right:document.querySelector('[role=dialog]').getBoundingClientRect().right,dialog_width:document.querySelector('[role=dialog]').scrollWidth,dialog_client:document.querySelector('[role=dialog]').clientWidth}));evidence.viewport_measurements.push({role,...measure});assert(measure.dialog_right<=width);assert(measure.dialog_width<=measure.dialog_client);if(role==='admin'||width!==360)assert.equal(measure.document_width,width);else assert.equal(measure.document_width,489,'baseline English user header width changed')}
   await p.setViewportSize({width:1280,height:900});await p.evaluate(()=>document.documentElement.classList.add('dark'));await shot('dark-admin-1280');await p.evaluate(()=>document.documentElement.classList.remove('dark'))
   await p.evaluate(()=>document.documentElement.style.zoom='200%');await shot('zoom-admin-1280');await p.evaluate(()=>document.documentElement.style.zoom='')
   // Real auth actions and built session guards: pending routing survives normal renewal.
   await login(uid,true);await open();hold=true;await r.locator('[data-action=refresh]').click();await p.waitForFunction(()=>document.querySelector('[data-testid="ledger-section-routing"]')?.getAttribute('aria-busy')==='true');await shot('loading-admin-1280')
   assert(await r.locator('[data-action=refresh]').isDisabled());await p.waitForFunction(auth=>eval(auth).token.endsWith(':renewed'),AUTH,{timeout:10000});assert.equal(await r.getAttribute('aria-busy'),'true');hold=false;held.splice(0).forEach(res=>send(res,routing()));await idle()
   hold=true;await r.locator('[data-action=refresh]').click();await p.getByRole('button',{name:'Close',exact:true}).click();await p.getByRole('dialog').waitFor({state:'hidden'});const stale=held.splice(0);hold=false;await p.getByRole('button',{name:'Details',exact:true}).click();await idle();stale.forEach(res=>send(res,{},503));assert.equal(await r.getByRole('alert').count(),0)
   await login(1000);await p.getByRole('dialog').waitFor({state:'hidden'});await p.getByTestId('ledger-retry').waitFor();assert(!(await p.locator('body').innerText()).includes(id));await p.screenshot({path:path.join(images,'session-account-admin-1280.png')})
   await login(uid);await open('zh');await idle();await shot('zh-admin-1280');await p.keyboard.press('Escape');await p.getByRole('dialog').waitFor({state:'hidden'})
   await p.evaluate(auth=>eval(auth).logout(),AUTH);assert.equal(await r.count(),0)
  }else{
   assert.equal(calls.filter(c=>c.uid===uid&&c.kind==='routing').length,0);assert(!(await p.locator('body').innerText()).includes('req_synthetic_only'))
   for(const width of [360,768,1280,1920]){await p.setViewportSize({width,height:900});await p.screenshot({path:path.join(images,`after-user-${width}.png`)});const measure=await p.evaluate(()=>({width:innerWidth,document_width:document.documentElement.scrollWidth,dialog_right:document.querySelector('[role=dialog]').getBoundingClientRect().right,dialog_width:document.querySelector('[role=dialog]').scrollWidth,dialog_client:document.querySelector('[role=dialog]').clientWidth}));evidence.viewport_measurements.push({role,...measure});assert(measure.dialog_right<=width);assert(measure.dialog_width<=measure.dialog_client);if(role==='admin'||width!==360)assert.equal(measure.document_width,width);else assert.equal(measure.document_width,489,'baseline English user header width changed')}
   assert.equal((await ctx.request.get(url+`/api/v1/admin/requests/${id}/routing`,{headers:{Authorization:'Bearer ledger-fixture-101'}})).status(),403)
   assert.equal((await ctx.request.get(url+`/api/v1/admin/requests/${id}/routing`)).status(),401)
  }
  evidence.roles.push({role,viewports:[360,768,1280,1920],routing_visible:role==='admin'});await ctx.close()
 }
 assert.deepEqual(evidence.page_errors,[]);const adminExecuted=evidence.roles.some(r=>r.role==='admin');Object.assign(evidence,{routing_contract_http:adminExecuted,ledger_postgres:false,independent_error_retry:adminExecuted,local_paging:adminExecuted,closed_late_error:adminExecuted,session_renewal:adminExecuted,same_account_relogin:adminExecuted,switch_503_clear:adminExecuted,keyboard:adminExecuted,reduced_motion:true,zoom:adminExecuted?'CSS200% simulation':'not run',fixture_permission_denials:evidence.roles.some(r=>r.role==='user')?[401,403]:[]})
 fs.writeFileSync(path.join(__dirname,process.env.ROUTING_CAPTURE_OUTPUT || 'browser-evidence.json'),JSON.stringify(evidence,null,2)+'\n');console.log(JSON.stringify(evidence))
}finally{await browser.close();server.closeAllConnections();await new Promise(resolve=>server.close(resolve))}
})().catch(e=>{console.error(e);process.exit(1)})
