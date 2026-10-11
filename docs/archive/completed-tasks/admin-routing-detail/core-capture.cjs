// Existing ledger core regression: actual build + HTTP/PG. Routing API is absent on fb main; routing local error must not break billing. No routing PG claim.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || '/opt/codex/runtimes/cua/lib/node_modules/playwright-core')
const fs = require('fs'), path = require('path'), assert = require('node:assert/strict')
const root = path.resolve(__dirname, '../../../visual-reviews/assets/admin-routing-detail/core-regression')
;(async () => {
 const m = fs.readFileSync(process.argv[2], 'utf8').match(/DETAIL_BROWSER_READY url=(\S+) request=(\S+) batch=(\S+) usage=(\d+) other=(\S+)/)
 assert(m, 'fixture is not ready')
 const [,url,id,batchID,usageID,otherID] = m
 const browser = await chromium.launch({ executablePath: process.env.CHROMIUM_BIN || '/usr/bin/chromium', headless: true, args: ['--no-sandbox'] })
 const evidence = []
 try {
  for (const [uid,role] of [[101,'user'],[999,'admin']]) {
   const context = await browser.newContext({viewport:{width:1280,height:900},reducedMotion:'reduce',extraHTTPHeaders:{'X-Ledger-Test-User':String(uid)}})
   await context.addInitScript(({uid,role})=>{
    localStorage.setItem('auth_token','fixture-only-not-a-credential')
    localStorage.setItem('auth_user',JSON.stringify({id:uid,role,username:'Synthetic reviewer',email:'fixture@example.invalid',balance:94}))
    localStorage.setItem(`${role}_guide_${uid}_${role}_v4_interactive`,'true')
   },{uid,role})
   const prefix = role==='admin'?'/admin':''
   const basePath = `/api/v1${prefix}/requests/${id}`
   const legacy = await context.request.get(url+basePath)
   assert.equal(legacy.status(),409)
   assert.equal((await legacy.json()).reason,'PAGINATION_REQUIRED')
   const foreign = await context.request.get(url+`/api/v1/requests/${id}?section=billing`,{headers:{'X-Ledger-Test-User':'102'}})
   assert.equal(foreign.status(),404)
   const denied = await context.request.get(url+`/api/v1/admin/requests/${id}?section=attempts`,{headers:{'X-Ledger-Test-User':'101'}})
   assert.equal(denied.status(),403)
   const p = await context.newPage(), errors=[], calls=[], pending=[]
   p.on('pageerror',e=>errors.push(e.message))
   p.on('request',request=>{
    const u=new URL(request.url())
    if(u.pathname===basePath) {
     assert(u.searchParams.get('view')==='summary'||u.searchParams.has('section'),'UI used legacy detail')
     calls.push({section:u.searchParams.get('section')||'summary',cursor:!!u.searchParams.get('cursor')})
    }
   })
   p.on('response',response=>{
    const u=new URL(response.url())
    if(u.pathname===basePath&&u.searchParams.has('section')&&response.status()===200) pending.push((async()=>{
     const {data}=await response.json();assert(data.items.length<=20);assert.equal(data.consistency,'live')
     if(role==='user'&&u.searchParams.get('section')==='attempts')for(const a of data.items)for(const key of ['account_id','credential_account_id','upstream_request_id','upstream_response_id'])assert.equal(a[key],undefined)
    })())
   })
   const region = name=>p.getByTestId(`ledger-section-${name}`)
   async function step(name,action) {
    const [res]=await Promise.all([
     p.waitForResponse(r=>{const u=new URL(r.url());return u.pathname===basePath&&u.searchParams.get('section')===name}),
     region(name).locator(`[data-action=${action}]`).click(),
    ])
    await p.waitForFunction(el=>el.getAttribute('aria-busy')==='false',await region(name).elementHandle())
    return res
   }
   async function lastPage(name) {
    let n=0
    while(await region(name).locator('[data-action=next]').isEnabled()) { assert(++n<20);await step(name,'next') }
    return n
   }
   async function screenshot(name,section='billing') {
    await region(section).locator('nav').scrollIntoViewIfNeeded()
    await p.screenshot({path:`${root}/${name}-${role}-1280.png`,animations:'disabled'})
   }
   await p.goto(url+`${prefix}/requests?private_id=${id}`,{waitUntil:'networkidle'})
   await p.getByRole('dialog').waitFor()
   assert.equal(await region('diagnostics').count(),role==='admin'?1:0)
   const initialOthers = calls.filter(c=>['billing','wallet'].includes(c.section)).length
   await step('attempts','next')
   assert((await region('attempts').innerText()).includes('#21'))
   assert(!(await region('attempts').innerText()).includes('#1\n'))
   assert.equal(calls.filter(c=>['billing','wallet'].includes(c.section)).length,initialOthers)
   await screenshot('attempts','attempts')
   await lastPage('billing'); await lastPage('wallet')
   const usageButton=p.getByRole('button',{name:`用量记录 #${usageID}`,exact:true})
   await usageButton.focus();await p.keyboard.press('Enter')
   await p.getByText(batchID,{exact:true}).waitFor()
   assert((await region('billing').innerText()).includes('该批次已结算'))
   assert((await region('billing').innerText()).includes('图片张数'))
   assert.equal(await p.getByText('输入 Token',{exact:true}).count(),0)
   const proof=(await(await context.request.get(url+`${basePath}/usage/${usageID}`)).json()).data
   assert.equal(proof.image_count,3);assert.equal(proof.billed_cost,6);assert.equal(proof.batch_id,batchID)
   for(const width of [360,768,1280,1920]) {
    await p.setViewportSize({width,height:900});await region('billing').locator('nav').scrollIntoViewIfNeeded()
    await p.screenshot({path:`${root}/after-${role}-${width}.png`,animations:'disabled'})
    assert.equal(await p.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false,`overflow at ${width}`)
   }
   await p.setViewportSize({width:1280,height:900})
   await p.evaluate(()=>document.documentElement.classList.add('dark'));await screenshot('dark');await p.evaluate(()=>document.documentElement.classList.remove('dark'))
   let releaseBilling
   const billingDelay=new Promise(resolve=>{releaseBilling=resolve})
   await context.route(u=>u.pathname===basePath&&u.searchParams.get('section')==='billing',async route=>{await billingDelay;await route.continue()})
   await region('billing').locator('[data-action=previous]').click()
   await p.waitForFunction(el=>el.getAttribute('aria-busy')==='true',await region('billing').elementHandle())
   assert(await usageButton.isDisabled(),'previous-page usage remained clickable during page change')
   assert.equal(await p.getByText(batchID,{exact:true}).count(),0,'page change retained old usage drill')
   releaseBilling();await p.waitForFunction(el=>el.getAttribute('aria-busy')==='false',await region('billing').elementHandle());await context.unrouteAll()
   await step('wallet','refresh')
   assert((await region('wallet').innerText()).includes('仍有后续页'))
   assert(await region('wallet').locator('[data-action=next]').isEnabled())
   assert(await region('wallet').locator('[data-action=previous]').isDisabled())
   await screenshot('empty','wallet')
   const walletRefresh=region('wallet').locator('[data-action=refresh]')
   await walletRefresh.hover();await screenshot('hover','wallet')
   await walletRefresh.focus();await screenshot('focus','wallet')
   const refreshBox=await walletRefresh.boundingBox()
   await p.mouse.move(refreshBox.x+refreshBox.width/2,refreshBox.y+refreshBox.height/2)
   await p.mouse.down();await screenshot('active','wallet')
   const activeResponse=p.waitForResponse(r=>{const u=new URL(r.url());return u.pathname===basePath&&u.searchParams.get('section')==='wallet'})
   await p.mouse.up();await activeResponse
   await p.waitForFunction(el=>el.getAttribute('aria-busy')==='false',await region('wallet').elementHandle())
   let release
   const delay=new Promise(resolve=>{release=resolve})
   await context.route(u=>u.pathname===basePath&&u.searchParams.get('section')==='wallet'&&!!u.searchParams.get('cursor'),async route=>{await delay;await route.continue()})
   await region('wallet').locator('[data-action=next]').click()
   await p.waitForFunction(el=>el.getAttribute('aria-busy')==='true',await region('wallet').elementHandle())
   assert(await region('wallet').locator('[data-action=next]').isDisabled())
   assert(await region('wallet').locator('[data-action=refresh]').isDisabled())
   await screenshot('loading','wallet')
   release();await p.waitForFunction(el=>el.getAttribute('aria-busy')==='false',await region('wallet').elementHandle());await context.unrouteAll()
   await step('billing','refresh')
   await context.route(u=>u.pathname===basePath&&u.searchParams.get('section')==='billing'&&!!u.searchParams.get('cursor'),route=>route.fulfill({status:503,contentType:'application/json',body:'{"code":503,"message":"synthetic section failure"}'}))
   await step('billing','next')
   await region('billing').getByRole('alert').waitFor()
   await region('billing').getByRole('alert').scrollIntoViewIfNeeded()
   await p.screenshot({path:`${root}/error-${role}-1280.png`,animations:'disabled'})
   assert((await region('attempts').innerText()).includes('#21'),'one failed section hid another')
   await context.unrouteAll();await step('billing','retry')
   assert.equal(await region('billing').getByRole('alert').count(),0)
   await region('billing').locator('[data-action=refresh]').focus()
   const refreshed=p.waitForResponse(r=>{const u=new URL(r.url());return u.pathname===basePath&&u.searchParams.get('section')==='billing'&&!u.searchParams.get('cursor')})
   await p.keyboard.press('Enter');await refreshed
   await p.waitForFunction(el=>el.getAttribute('aria-busy')==='false',await region('billing').elementHandle())
   assert(await region('billing').locator('[data-action=previous]').isDisabled())
   await p.keyboard.press('Escape');await p.getByRole('dialog').waitFor({state:'hidden'})
   await p.goto(url+`${prefix}/requests?private_id=${id}&lang=en`,{waitUntil:'networkidle'})
   await lastPage('billing')
   await p.getByRole('button',{name:`Usage record #${usageID}`,exact:true}).click();await p.getByText(batchID,{exact:true}).waitFor()
   await screenshot('en')
   await p.evaluate(()=>document.documentElement.style.zoom='200%');await screenshot('zoom');await p.evaluate(()=>document.documentElement.style.zoom='')
   await p.goto(url+`${prefix}/requests?private_id=${otherID}&lang=en`,{waitUntil:'networkidle'})
   assert.equal(await p.getByText(batchID,{exact:true}).count(),0)
   assert(await region('attempts').locator('[data-action=previous]').isDisabled())
   assert(await region('billing').locator('[data-action=next]').isDisabled())
   await Promise.all(pending)
   assert.deepEqual(errors,[])
   evidence.push({role,request_id:id,usage_id:Number(usageID),batch_id:batchID,billed_cost:6,image_count:3,legacy_status:409,foreign_status:404,admin_denied_status:403,request_count:calls.length,summary_reads:calls.filter(c=>c.section==='summary').length,all_ui_detail_reads_explicit:true,bounded_items:true,independent_paging:true,empty_intermediate:true,loading:true,usage_disabled_during_page_change:true,error_retry:true,refresh_resets_cursor:true,hover_active_focus:true,keyboard:true,reduced_motion:true,zoom:'CSS 200% simulation',viewports:[360,768,1280,1920],page_errors:errors})
   await context.close()
  }
  fs.writeFileSync(path.join(__dirname,'core-browser-evidence.json'),JSON.stringify(evidence,null,2)+'\n')
  console.log(JSON.stringify(evidence))
 } finally { await browser.close();fs.writeFileSync('/tmp/ledger-detail-browser-stop','done') }
})().catch(e=>{console.error(e);process.exit(1)})
