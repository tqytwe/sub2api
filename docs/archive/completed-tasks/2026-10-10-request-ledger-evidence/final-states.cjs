const {chromium}=require('/opt/codex/runtimes/cua/lib/node_modules/playwright-core');
const fs=require('fs');
(async()=>{
 const browser=await chromium.launch({executablePath:'/usr/bin/chromium',headless:true,args:['--no-sandbox']});
 const evidence=[];
 try {
  for(const [id,role] of [[101,'user'],[999,'admin']]) {
   const c=await browser.newContext({viewport:{width:1280,height:900},extraHTTPHeaders:{'X-Ledger-Test-User':String(id)}});
   await c.addInitScript(({id,role})=>{localStorage.setItem('auth_token','fixture-only-not-a-credential');localStorage.setItem('auth_user',JSON.stringify({id,role}));localStorage.setItem(`${role==='admin'?'admin':'user'}_guide_${id}_${role}_v4_interactive`,'true')},{id,role});
   const p=await c.newPage(),base=role==='admin'?'/admin':'';const errors=[];p.on('pageerror',e=>errors.push(e.message));
   await p.goto(`http://127.0.0.1:18762${base}/requests?lang=en`,{waitUntil:'networkidle'});
   const balance=p.locator('header span.tabular-nums');await balance.waitFor();if((await balance.innerText()).trim()!=='$98.75')throw Error('header differs from the real PG wallet balance');
   await p.getByRole('button',{name:'Details',exact:true}).first().click();
   const dialog=p.getByRole('dialog');await dialog.waitFor();
   await dialog.getByRole('button',{name:/Usage record #/}).click();
   await dialog.getByText('synthetic-usage-fixture',{exact:true}).waitFor();
   const detailText=await dialog.innerText();if(!detailText.includes('Auxiliary download'))throw Error('auxiliary phase label missing');if(!detailText.includes('1.2500')||!detailText.includes('Input tokens'))throw Error('missing verified usage drilldown');
   await p.screenshot({path:`/workspace/request-ledger/docs/visual-reviews/assets/request-ledger/usage-${role}-en-1280.png`,fullPage:false,animations:'disabled'});
   await p.keyboard.press('Escape');
   await p.getByRole('button',{name:'Apply filters',exact:true}).click();await p.waitForLoadState('networkidle');
   if(!p.url().includes('lang=en'))throw Error('language dropped during filter');
   await p.getByRole('button',{name:'Details',exact:true}).first().waitFor();
   if(role==='admin') { const pending=p.waitForResponse(r=>r.url().includes('/admin/requests?')&&new URL(r.url()).searchParams.get('page')==='2');await p.getByRole('button',{name:'Next',exact:true}).last().click();await pending;await p.locator('[aria-current=page]').filter({hasText:'2'}).waitFor(); }
   await p.screenshot({path:`/workspace/request-ledger/docs/visual-reviews/assets/request-ledger/updated-${role}-en-1280.png`,fullPage:true,animations:'disabled'});
   const match=u=>new URL(u).pathname===`/api/v1${base}/requests`;
   let release;const gate=new Promise(resolve=>release=resolve);
   await p.route(match,async route=>{await gate;await route.continue()});
   await p.getByTestId('ledger-refresh').click();await p.locator('section[aria-busy=true]').waitFor();
   await p.screenshot({path:`/workspace/request-ledger/docs/visual-reviews/assets/request-ledger/loading-${role}-1280.png`,fullPage:true,animations:'disabled'});
   release();await p.locator('section[aria-busy=false]').waitFor();await p.unroute(match);
   await p.route(match,route=>route.fulfill({status:503,contentType:'application/json',body:JSON.stringify({code:503,message:'Synthetic storage failure'})}));
   await p.getByTestId('ledger-refresh').click();await p.getByTestId('ledger-retry').waitFor();
   await p.screenshot({path:`/workspace/request-ledger/docs/visual-reviews/assets/request-ledger/error-${role}-1280.png`,fullPage:true,animations:'disabled'});
   await p.unroute(match);await p.getByTestId('ledger-retry').click();await p.getByRole('button',{name:'Details',exact:true}).first().waitFor();
   evidence.push({role,realUsageDrilldown:true,realWalletBalance:98.75,languagePreserved:true,pagination:role==='admin',loading:true,syntheticReadError:true,retryRealAPI:true,pageErrors:errors});await c.close();
  }
  fs.writeFileSync('/tmp/ledger-visual/final-state-evidence.json',JSON.stringify(evidence,null,2));console.log(JSON.stringify(evidence));
 } finally {await browser.close()}
})().catch(e=>{console.error(e);process.exit(1)});
