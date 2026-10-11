// Built assets + real HTTP/PG ledger. Auth/failure injection is loopback-only synthetic test transport.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || '/opt/codex/runtimes/cua/lib/node_modules/playwright-core')
const fs = require('node:fs'), path = require('node:path'), http = require('node:http'), assert = require('node:assert/strict')
const images = path.resolve(__dirname, '../../../visual-reviews/assets/ledger-detail-pagination')
const AUTH = "document.querySelector('#app').__vue_app__.config.globalProperties.$pinia._s.get('auth')"
const ROUTER = "document.querySelector('#app').__vue_app__.config.globalProperties.$router"
;(async () => {
 const match = fs.readFileSync(process.argv[2], 'utf8').match(/DETAIL_BROWSER_READY url=(\S+) request=(\S+) batch=(\S+) usage=(\d+)/)
 assert(match, 'fixture is not ready')
 const [, backend, id] = match, calls = [], evidence = []
 assert.equal(new URL(backend).hostname, '127.0.0.1')
 const profile = uid => ({id:uid,role:uid>=999?'admin':'user',username:'Synthetic reviewer',email:'fixture@example.invalid',balance:94,status:'active'})
 const send = (res, data, status=200) => { res.writeHead(status, {'Content-Type':'application/json'});res.end(JSON.stringify({code:status===200?0:status,data})) }
 const proxy = http.createServer(async (req,res) => {
  const u = new URL(req.url,backend), token = String(req.headers.authorization || ''), uid = Number(token.match(/ledger-fixture-(\d+)/)?.[1] || 101)
  if (u.pathname.endsWith('/auth/login')) {
   let raw='';for await (const chunk of req) raw+=chunk
   const email=JSON.parse(raw).email, target=Number(email.match(/^fixture-(101|102|999|1000)(?:-short)?@example.invalid$/)?.[1])
   if (!target) return send(res,{},400)
   return send(res,{access_token:`ledger-fixture-${target}`,refresh_token:`ledger-refresh-${target}`,expires_in:email.includes('-short@')?125:3600,user:profile(target)})
  }
  if (u.pathname.endsWith('/auth/refresh')) {
   let raw='';for await(const chunk of req) raw+=chunk
   const target=Number(JSON.parse(raw).refresh_token.match(/^ledger-refresh-(101|999)$/)?.[1])
   if(!target)return send(res,{},400)
   calls.push({kind:'refresh',uid:target})
   return send(res,{access_token:`ledger-fixture-${target}:renewed`,refresh_token:`ledger-refresh-${target}`,expires_in:3600,token_type:'Bearer'})
  }
  if(u.pathname.endsWith('/auth/logout'))return send(res,{})
  if(u.pathname.endsWith('/auth/me'))return send(res,profile(uid))
  if(u.pathname.includes('/requests')) {
   calls.push({kind:'ledger',uid,section:u.searchParams.get('section')||u.searchParams.get('view')||'list',cursor:!!u.searchParams.get('cursor')})
   if(uid===102||uid===1000)return send(res,{},503)
  }
  const forwarded=http.request(u,{method:req.method,headers:{...req.headers,host:new URL(backend).host,'X-Ledger-Test-User':String(uid)}},upstream=>{res.writeHead(upstream.statusCode,upstream.headers);upstream.pipe(res)})
  forwarded.on('error',()=>{if(!res.headersSent)send(res,{},502);else res.destroy()})
  req.pipe(forwarded)
 })
 await new Promise(resolve=>proxy.listen(0,'127.0.0.1',resolve))
 const url=`http://127.0.0.1:${proxy.address().port}`
 const browser=await chromium.launch({executablePath:process.env.CHROMIUM_BIN||'/usr/bin/chromium',headless:true,args:['--no-sandbox']})
 try {
  for(const [uid,role] of [[101,'user'],[999,'admin']]) {
   const context=await browser.newContext({viewport:{width:1280,height:900},reducedMotion:'reduce'})
   await context.addInitScript(()=>{for(const [id,role] of [[101,'user'],[102,'user'],[999,'admin'],[1000,'admin']])localStorage.setItem(`${role}_guide_${id}_${role}_v4_interactive`,'true')})
   const p=await context.newPage(), errors=[]
   p.on('pageerror',e=>errors.push(e.message))
   await p.goto(url+'/login?lang=en',{waitUntil:'networkidle'})
   const login=async (account,short=false)=>p.evaluate(async({auth,account,short})=>{await eval(auth).login({email:`fixture-${account}${short?'-short':''}@example.invalid`,password:'synthetic-fixture-only'})},{auth:AUTH,account,short})
   const open=async()=>{
    await p.evaluate(async({router,target})=>eval(router).push(target),{router:ROUTER,target:`${role==='admin'?'/admin':''}/requests?private_id=${id}&lang=en`})
    await p.getByTestId('ledger-section-attempts').waitFor()
    await p.waitForFunction(()=>document.querySelector('[data-testid="ledger-section-attempts"]')?.getAttribute('aria-busy')==='false')
   }
   const region=p.getByTestId('ledger-section-attempts')
   await login(uid,true);await open()
   const oldMarker=await p.evaluate(()=>localStorage.getItem('auth_session_id'))
   let release, arrived
   const delayed=new Promise(resolve=>{release=resolve}), intercept=new Promise(resolve=>{arrived=resolve})
   await context.route(u=>u.pathname.includes('/requests/')&&u.searchParams.get('section')==='attempts'&&u.searchParams.has('cursor')&&!!u.searchParams.get('cursor'),async route=>{arrived();await delayed;await route.continue().catch(()=>{})})
   await region.locator('[data-action=next]').click();await intercept
   const readsBefore=calls.filter(c=>c.kind==='ledger').length
   await p.waitForFunction(auth=>eval(auth).token.endsWith(':renewed'),AUTH,{timeout:10000})
   assert.equal(await p.evaluate(()=>localStorage.getItem('auth_session_id')),oldMarker)
   assert.equal(calls.filter(c=>c.kind==='ledger').length,readsBefore)
   assert.equal(await region.getAttribute('aria-busy'),'true')
   release();await p.waitForFunction(()=>document.querySelector('[data-testid="ledger-section-attempts"]')?.getAttribute('aria-busy')==='false')
   assert(await region.locator('[data-action=previous]').isEnabled())
   assert((await region.innerText()).includes('#21'))
   await region.scrollIntoViewIfNeeded();await p.screenshot({path:`${images}/session-renewal-${role}-1280.png`})
   await context.unrouteAll()
   await login(uid)
   await p.getByRole('dialog').waitFor({state:'hidden'})
   assert.equal(new URL(p.url()).searchParams.has('private_id'),false)
   assert.notEqual(await p.evaluate(()=>localStorage.getItem('auth_session_id')),oldMarker)
   await open()
   await login(uid===101?102:1000)
   await p.getByRole('dialog').waitFor({state:'hidden'})
   await p.getByTestId('ledger-retry').waitFor()
   assert(!(await p.locator('body').innerText()).includes(id))
   assert.equal(await p.locator('[name=private_id]').inputValue(),'')
   await p.screenshot({path:`${images}/session-account-${role}-1280.png`})
   await login(uid);await open()
   await p.waitForLoadState('networkidle')
   const loggedOutCalls=calls.filter(c=>c.kind==='ledger').length
   await p.evaluate(async auth=>eval(auth).logout(),AUTH)
   await p.getByRole('dialog').waitFor({state:'hidden'})
   // The production router may redirect this guest to login when the old query is cleared.
   if (await p.getByTestId('ledger-refresh').count()) await p.getByTestId('ledger-refresh').click()
   else assert.equal(new URL(p.url()).pathname, '/login')
   await p.waitForTimeout(150)
   assert.equal(calls.filter(c=>c.kind==='ledger').length,loggedOutCalls)
   assert(!(await p.locator('body').innerText()).includes(id))
   await p.screenshot({path:`${images}/session-logout-${role}-1280.png`})
   assert.deepEqual(errors,[])
   evidence.push({role,actual_built_assets:true,actual_pinia_actions:true,ledger_http_postgres:true,auth_transport:'loopback synthetic only',account_b_failure:'injected HTTP 503',normal_rotation_keeps_pending_page_and_cursor:true,same_account_relogin_resets:true,same_role_switch_clears_on_failure:true,logout_clears_without_guest_read:true,page_errors:errors})
   await context.close()
  }
  fs.writeFileSync(path.join(__dirname,'session-browser-evidence.json'),JSON.stringify(evidence,null,2)+'\n')
  console.log(JSON.stringify(evidence))
 } finally {await browser.close();proxy.closeAllConnections();await new Promise(resolve=>proxy.close(resolve));fs.writeFileSync('/tmp/ledger-detail-browser-stop','done')}
})().catch(error=>{console.error(error);process.exit(1)})
