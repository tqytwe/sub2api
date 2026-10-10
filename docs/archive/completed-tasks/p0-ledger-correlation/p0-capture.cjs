const {chromium}=require(process.env.PLAYWRIGHT_MODULE || 'playwright-core');
const fs=require('fs');
const root=require('path').resolve(__dirname,'../../../visual-reviews/assets/p0-ledger-correlation');
(async()=>{
 const browser=await chromium.launch({executablePath:process.env.CHROMIUM_BIN || '/usr/bin/chromium',headless:true,args:['--no-sandbox']});const evidence=[];
 for(const [uid,role] of [[999,'admin'],[101,'user']]){
  const context=await browser.newContext({viewport:{width:1280,height:900},reducedMotion:'reduce',extraHTTPHeaders:{'X-Ledger-Test-User':String(uid)}});
  await context.addInitScript(({uid,role})=>{localStorage.setItem('auth_token','fixture-only-not-a-credential');localStorage.setItem('auth_user',JSON.stringify({id:uid,role,username:'Synthetic reviewer',email:'fixture@example.invalid',balance:0}));localStorage.setItem(`${role}_guide_${uid}_${role}_v4_interactive`,'true')},{uid,role});
  const prefix=role==='admin'?'/admin':'';const response=await context.request.get('http://127.0.0.1:18762/api/v1'+prefix+'/requests?page_size=100');const data=(await response.json()).data;
  const item=data.items.find(x=>x.attempt_count===2&&x.execution_state==='failed');if(!item)throw Error('missing exact fixture');
  const p=await context.newPage();const errors=[];p.on('pageerror',e=>errors.push(e.message));const url=`http://127.0.0.1:18762${prefix}/requests?private_id=${item.id}`;
  await p.goto(url,{waitUntil:'networkidle'});await p.getByText('精确关联证据',{exact:true}).waitFor();
  const detail=await (await context.request.get(`http://127.0.0.1:18762/api/v1${prefix}/requests/${item.id}`)).json();
  const text=await p.getByRole('dialog').innerText();if(!text.includes(detail.data.attempts[0].attempt_id))throw Error('UUID mismatch');
  if(role==='admin'&&!text.includes('req_fixture_1'))throw Error('missing upstream evidence');
  if(role==='user'&&(text.includes('req_fixture_1')||text.includes('Ops #')))throw Error('private fields visible');
  for(const width of [360,768,1280,1920]){
   await p.setViewportSize({width,height:900});await p.getByText('精确关联证据',{exact:true}).scrollIntoViewIfNeeded();
   await p.screenshot({path:`${root}/after-${role}-${width}.png`,animations:'disabled'});
   if(await p.evaluate(()=>document.documentElement.scrollWidth>innerWidth))throw Error('page overflow '+role+width);
  }
  await p.setViewportSize({width:1280,height:900});await p.evaluate(()=>document.documentElement.classList.add('dark'));await p.screenshot({path:`${root}/dark-${role}-1280.png`,animations:'disabled'});await p.evaluate(()=>document.documentElement.classList.remove('dark'));
  if(role==='admin'){
   const ops=p.locator('[data-testid^="ledger-ops-"]').first();await ops.focus();await p.keyboard.press('Enter');await p.locator(`a[href*="private_id=${item.id}"]`).first().waitFor();
   const modal=p.getByRole('dialog').last();if(!(await modal.innerText()).includes(item.id))throw Error('Ops request mismatch');
   await p.screenshot({path:`${root}/ops-admin-1280.png`,animations:'disabled'});
   await modal.getByRole('link').filter({hasText:item.id}).first().click();await p.waitForTimeout(200);
   if(!p.url().includes(item.id))throw Error('reverse link mismatch');
  }
  await p.keyboard.press('Escape');await p.goto(url+'&lang=en',{waitUntil:'networkidle'});await p.getByText('Exact correlation evidence',{exact:true}).waitFor();await p.getByText('Exact correlation evidence',{exact:true}).scrollIntoViewIfNeeded();
  await p.screenshot({path:`${root}/en-${role}-1280.png`,animations:'disabled'});
  await context.route(u=>u.pathname===`/api/v1${prefix}/requests/${item.id}`,route=>route.fulfill({status:503,contentType:'application/json',body:'{"message":"synthetic read fault"}'}));await p.reload({waitUntil:'networkidle'});await p.getByRole('dialog').getByRole('alert').waitFor();await p.screenshot({path:`${root}/error-${role}-1280.png`,animations:'disabled'});
  await context.unrouteAll();await p.getByRole('dialog').getByRole('button',{name:'Retry',exact:true}).click();await p.getByText('Exact correlation evidence',{exact:true}).waitFor();
  await context.route(u=>u.pathname===`/api/v1${prefix}/requests/${item.id}`,async route=>{
   const response=await route.fetch();const payload=await response.json();payload.data.attempt_evidence_unavailable=true;
   payload.data.attempts=payload.data.attempts.map(a=>({...a,attempt_id:null,upstream_request_id:undefined,upstream_response_id:undefined,identifier_conflict:undefined}));
   await route.fulfill({response,json:payload});
  });
  await p.reload({waitUntil:'networkidle'});await p.getByText('Correlation evidence is temporarily unavailable.',{exact:false}).waitFor();
  if(!(await p.getByRole('dialog').innerText()).includes(item.id))throw Error('core detail hidden by optional evidence fault');
  await p.screenshot({path:`${root}/evidence-unavailable-${role}-1280.png`,animations:'disabled'});await context.unrouteAll();
  await p.keyboard.press('Escape');await p.getByRole('dialog').waitFor({state:'hidden'});
  if(errors.length)throw Error(JSON.stringify(errors));evidence.push({role,request_id:item.id,attempt_uuids:detail.data.attempts.map(a=>a.attempt_id),viewports:[360,768,1280,1920],themes:['light','dark'],languages:['zh','en'],keyboard:true,reduced_motion:true,diagnostic_retention:role==='admin',http_failure_retry:true,optional_evidence_failure:true,pageErrors:errors});await context.close();
 }
 await browser.close();fs.writeFileSync(require('path').join(__dirname,'browser-evidence.json'),JSON.stringify(evidence,null,2));console.log(JSON.stringify(evidence));
})().catch(e=>{console.error(e);process.exit(1)});
