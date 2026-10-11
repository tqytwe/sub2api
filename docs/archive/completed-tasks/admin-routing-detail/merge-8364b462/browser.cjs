// Prepared real HTTP/PG consumer. No API response stubs or synthetic JSON states.
// Real JWT/admin middleware and PG users; test-only token issuance, no production login proof.
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'/opt/codex/runtimes/cua/lib/node_modules/playwright-core')
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict')
;(async()=>{
 const [metaFile,out]=process.argv.slice(2)
 assert(metaFile&&out,'usage: node browser.cjs metadata.json output-directory')
 const m=JSON.parse(fs.readFileSync(metaFile,'utf8')),url=new URL(m.url)
 assert.equal(url.hostname,'127.0.0.1');assert.equal(url.protocol,'http:')
 fs.mkdirSync(out) // Exclusive evidence directory; never reuse an earlier capture.
 const browser=await chromium.launch({executablePath:process.env.CHROMIUM_BIN||'/usr/bin/chromium',headless:true,args:['--no-sandbox']})
 const evidence={scope:m.scope,executed_at:new Date().toISOString(),real_routing_HTTP_PG:true,real_JWT_admin_authorization:true,production_authentication:false,full_scheduler_or_provider:false,states:[],page_errors:[],role_denials:[],screenshots:[]}
 let control
 try{
  const admin=await browser.newContext({viewport:{width:1280,height:900}});control=admin.request
  const issued={}
  for(const uid of [101,102,999,1000]){
   const res=await admin.request.post(m.url+'/api/v1/auth/login',{data:{email:`fixture-${uid}@example.invalid`}})
   assert.equal(res.status(),200);issued[uid]=(await res.json()).data
  }
  const headers=(uid=999)=>({Authorization:uid?`Bearer ${issued[uid].access_token}`:''})
  const apiGet=(path,uid=999)=>admin.request.get(m.url+path,{headers:headers(uid)})
  const init=async(ctx,uid)=>ctx.addInitScript(({data,uid})=>{
   localStorage.setItem('auth_token',data.access_token)
   localStorage.setItem('auth_user',JSON.stringify(data.user))
   localStorage.setItem(`${uid>=999?'admin':'user'}_guide_${uid}_${uid>=999?'admin':'user'}_v4_interactive`,'true')
  },{data:issued[uid],uid})
  await init(admin,999)
  const p=await admin.newPage();p.on('pageerror',e=>evidence.page_errors.push(e.message))
  const calls=[];p.on('request',r=>{if(new URL(r.url()).pathname.endsWith('/routing'))calls.push(r.url())})
  const region=p.getByTestId('ledger-section-routing'),billing=p.getByTestId('ledger-section-billing')
  const idle=()=>p.waitForFunction(()=>document.querySelector('[data-testid="ledger-section-routing"]')?.getAttribute('aria-busy')==='false')
  const get=(id,uid=999)=>apiGet(`/api/v1/admin/requests/${id}/routing`,uid)
  const open=async id=>{await p.goto(m.url+`/admin/requests?private_id=${id}&lang=en`);await p.getByRole('dialog').waitFor();await idle()}
  for(const [name,id]of Object.entries(m.requests)){
   const res=await get(id);assert.equal(res.status(),200);assert.equal(res.headers()['cache-control'],'no-store')
   const {data}=await res.json(),expected=name==='zero_candidates'?'complete':name
   assert.equal(data.request_id,id);assert.equal(data.status,expected);assert(data.events.length<=128)
   await open(id);assert.equal(await region.locator(`[data-routing-status="${expected}"]`).count(),1)
   if(name==='complete'){
    assert.equal(data.selection_count,2)
    assert((await region.getByTestId('routing-first-veto').innerText()).includes('235'))
    assert((await region.innerText()).includes('does not prove forwarding'))
   }
   if(name==='truncated'){
    assert(data.omitted_count>0)
    const n=calls.length;await region.locator('[data-action=next]').click()
    assert((await region.getByTestId('routing-page').innerText()).includes('21–40'));assert.equal(calls.length,n)
   }
   if(name==='incomplete')assert.equal(data.incomplete,true)
   if(name==='zero_candidates'){
    assert.equal(data.source_population_complete,false)
    assert.equal(data.events.find(e=>e.reason==='prefiltered_schedulable_pool').value,0)
    assert((await region.innerText()).includes('does not prove that the group has no accounts'))
   }
   await region.locator('h3').scrollIntoViewIfNeeded()
   const image=`state-${name}.png`;await p.screenshot({path:path.join(out,image),animations:'disabled'})
   evidence.screenshots.push(image);evidence.states.push({name,request_id:id,status:data.status,event_count:data.event_count,returned:data.events.length,omitted_count:data.omitted_count,source_population_complete:data.source_population_complete})
  }
  const id=m.requests.complete
  await open(id)
  await p.getByTestId('ledger-section-attempts').locator('[data-action=next]').click()
  await p.waitForFunction(()=>document.querySelector('[data-testid="ledger-section-attempts"]')?.textContent.includes('Page 2'))
  const usageButton=billing.getByRole('button',{name:`Usage record #${m.usage_id}`,exact:true})
  await usageButton.click();await p.getByText('synthetic-usage-fixture',{exact:true}).waitFor()
  const before=(await(await apiGet(`/api/v1/admin/requests/${id}/usage/${m.usage_id}`)).json()).data
  assert.equal(before.billed_cost,1.25)
  try{
   assert.equal((await control.post(m.url+'/__fixture/routing-storage/unavailable')).status(),204)
   const failure=await get(id);assert.equal(failure.status(),503);assert.equal(failure.headers()['cache-control'],'no-store')
   const body=await failure.text();assert(!body.includes('routing363_fixture'));assert(!body.toLowerCase().includes('sql'))
   await region.locator('[data-action=refresh]').click();await idle();await region.getByRole('alert').waitFor()
   assert((await billing.innerText()).includes('synthetic-usage-fixture'))
   assert((await p.getByTestId('ledger-section-attempts').innerText()).includes('Page 2'))
   for(const section of ['attempts','billing','wallet','diagnostics'])assert.equal((await apiGet(`/api/v1/admin/requests/${id}?section=${section}`)).status(),200)
   assert.equal((await apiGet(`/api/v1/admin/requests/${id}/usage/${m.usage_id}`)).status(),200)
   await p.screenshot({path:path.join(out,'real-storage-503.png'),animations:'disabled'})
  }finally{assert.equal((await control.post(m.url+'/__fixture/routing-storage/available')).status(),204)}
  await region.locator('[data-action=retry]').click();await idle();assert.equal(await region.getByRole('alert').count(),0)
  assert((await billing.innerText()).includes('synthetic-usage-fixture'))
  const after=(await(await apiGet(`/api/v1/admin/requests/${id}/usage/${m.usage_id}`)).json()).data
  assert.deepEqual(after,before)
  evidence.real_storage_failure_503=true;evidence.billing_usage_and_attempt_cursor_preserved=true
  // Real JWT validation and admin role checks, with users loaded from PostgreSQL.
  for(const [uid,expected]of [['',401],['101',403],['102',403]]){
   const res=await get(id,uid);assert.equal(res.status(),expected);const body=await res.text();assert(!body.includes(id));assert(!body.includes('model_rate_limited'))
   evidence.role_denials.push({fixture_uid:uid||'guest',http_status:res.status(),source:'real JWT/admin middleware and PG user lookup'})
  }
  assert.equal((await get('not-a-uuid')).status(),400)
  assert.equal((await get('00000000-0000-4000-8000-000000000099')).status(),404)
  const user=await browser.newContext({viewport:{width:1280,height:900}})
  await init(user,101)
  const up=await user.newPage(),userRouting=[];up.on('pageerror',e=>evidence.page_errors.push(e.message));up.on('request',r=>{if(new URL(r.url()).pathname.endsWith('/routing'))userRouting.push(r.url())})
  await up.goto(m.url+`/requests?private_id=${id}&lang=en`);await up.getByRole('dialog').waitFor();await up.waitForLoadState('networkidle')
  assert.equal(await up.getByTestId('ledger-section-routing').count(),0);assert.equal(userRouting.length,0)
  assert.equal((await apiGet(`/api/v1/requests/${id}/routing`,101)).status(),404)
  assert.equal((await apiGet(`/api/v1/requests/${id}?view=summary`,102)).status(),404)
  // Hold only bytes fetched from the real fixture, then deliver them out of order.
  // No fabricated routing or authentication response bodies are used.
  const AUTH="document.querySelector('#app').__vue_app__.config.globalProperties.$pinia._s.get('auth')"
  const login=uid=>p.evaluate(async({auth,uid})=>{await eval(auth).login({email:`fixture-${uid}@example.invalid`,password:''})},{auth:AUTH,uid})
  const authOpen=async()=>{await p.evaluate(async id=>document.querySelector('#app').__vue_app__.config.globalProperties.$router.push(`/admin/requests?private_id=${id}&lang=en`),id);await p.getByRole('dialog').waitFor();await idle()}
  let holdRouting=false,holdAuth=false,heldStatus=200;const held=[],authHeld=[]
  await p.route('**/api/v1/admin/requests/*/routing**',async route=>{
   if(!holdRouting)return route.continue()
   const response=await route.fetch();assert.equal(response.status(),heldStatus)
   if(heldStatus===200){const data=await response.json();assert.equal(data.data.request_id,id)}
   held.push({route,response})
  })
  await p.route('**/api/v1/auth/login**',async route=>{
   if(!holdAuth)return route.continue()
   const response=await route.fetch();assert.equal(response.status(),200)
   authHeld.push({route,response})
  })
  const until=async predicate=>{for(let n=0;!predicate()&&n<1000;n++)await new Promise(r=>setTimeout(r,10));assert(predicate(),'held real response did not arrive')}
  const release=async entry=>{try{await entry.route.fulfill({response:entry.response});return 'released'}catch(e){if(!/closed|aborted|Invalid InterceptionId|already handled/i.test(String(e)))throw e;return 'browser_aborted'}}
  const hold=async(status=200)=>{heldStatus=status;holdRouting=true;await region.locator('[data-action=refresh]').click();await until(()=>held.length===1)}
  const drain=async()=>{holdRouting=false;const outcomes=[];for(const entry of held.splice(0))outcomes.push(await release(entry));return outcomes}
  await open(id)
  await p.evaluate(()=>window.routingFixtureDocumentMarker='same-document-close-reopen')
  try{
   assert.equal((await control.post(m.url+'/__fixture/routing-storage/unavailable')).status(),204)
   await hold(503)
  }finally{assert.equal((await control.post(m.url+'/__fixture/routing-storage/available')).status(),204)}
  await p.getByRole('button',{name:'Close',exact:true}).click();await p.getByRole('dialog').waitFor({state:'hidden'})
  holdRouting=false;await p.getByRole('button',{name:'Details',exact:true}).click();await idle()
  assert.equal(await region.locator('[data-routing-status=complete]').count(),1)
  evidence.late_503_delivery=await drain();await p.waitForLoadState('networkidle')
  assert.equal(await p.evaluate(()=>window.routingFixtureDocumentMarker),'same-document-close-reopen')
  assert.equal(await region.getByRole('alert').count(),0);assert.equal(await region.locator('[data-routing-status=complete]').count(),1)
  evidence.real_late_storage_503_ignored_after_reopen=true
  await hold();await login(999);await p.getByRole('dialog').waitFor({state:'hidden'});await drain()
  assert.equal(await region.count(),0);await open(id)
  holdAuth=true
  await p.evaluate(auth=>{window.routingOldLogin=eval(auth).login({email:'fixture-1000@example.invalid',password:''}).then(()=>({ok:true}),e=>({code:e.code}))},AUTH)
  await until(()=>authHeld.length===1)
  await p.evaluate(auth=>{window.routingNewLogin=eval(auth).login({email:'fixture-999@example.invalid',password:''}).then(()=>({ok:true}),e=>({code:e.code}))},AUTH)
  await until(()=>authHeld.length===2)
  await release(authHeld[1]);assert.deepEqual(await p.evaluate(()=>window.routingNewLogin),{ok:true})
  await p.getByRole('dialog').waitFor({state:'hidden'});await authOpen();await hold()
  await release(authHeld[0]);assert.deepEqual(await p.evaluate(()=>window.routingOldLogin),{code:'AUTH_SESSION_CHANGED'})
  assert.equal(await p.evaluate(auth=>eval(auth).user.id,AUTH),999)
  assert.equal(await region.getAttribute('aria-busy'),'true');holdAuth=false;authHeld.length=0;await drain();await idle()
  await hold()
  const failed=await p.evaluate(async auth=>{const original=Storage.prototype.setItem;let fired=false;Storage.prototype.setItem=function(key,value){if(this===localStorage&&key==='auth_user'&&!fired){fired=true;throw new DOMException('fixture publication failure','QuotaExceededError')}return original.call(this,key,value)};try{await eval(auth).login({email:'fixture-1000@example.invalid',password:''});return {unexpected:true}}catch(e){return {fired,name:e.name,authenticated:eval(auth).isAuthenticated}}finally{Storage.prototype.setItem=original}},AUTH)
  assert.deepEqual(failed,{fired:true,name:'QuotaExceededError',authenticated:false})
  await p.getByRole('dialog').waitFor({state:'hidden'});await drain();assert.equal(await region.count(),0)
  await login(999);await authOpen();await hold();await login(1000)
  await p.getByRole('dialog').waitFor({state:'hidden'});await drain();assert.equal(await region.count(),0)
  // Explicitly querying as the second administrator is allowed; the stale dialog stays closed.
  await login(101);assert.equal(await region.count(),0)
  await p.evaluate(async()=>document.querySelector('#app').__vue_app__.config.globalProperties.$router.push('/requests?lang=en'))
  await p.waitForFunction(()=>location.pathname==='/requests'&&!new URLSearchParams(location.search).has('private_id'))
  assert.equal(await p.evaluate(auth=>eval(auth).user.id,AUTH),101)
  await p.evaluate(async id=>document.querySelector('#app').__vue_app__.config.globalProperties.$router.push(`/requests?private_id=${id}&lang=en`),id);await p.getByRole('dialog').waitFor()
  assert.equal(await region.count(),0)
  await p.evaluate(auth=>eval(auth).logout(),AUTH);await p.getByRole('dialog').waitFor({state:'hidden'})
  evidence.real_auth_session_cases=['close cancels late routing','same account relogin clears','reverse login bytes preserve winning identity','publication failure clears and recovers','admin switch clears','admin to user hides routing','logout clears']
  assert.deepEqual(evidence.page_errors,[]);evidence.user_ui_routing_absent=true
  fs.writeFileSync(path.join(out,'browser-evidence.json'),JSON.stringify(evidence,null,2)+'\n',{flag:'wx'})
  console.log(JSON.stringify({states:evidence.states.length,page_errors:0,scope:evidence.scope,production_authentication:false}))
 }finally{
  if(control)await control.post(m.url+'/__fixture/routing-storage/available').catch(()=>{})
  await browser.close()
 }
})().catch(e=>{console.error(e);process.exitCode=1})
