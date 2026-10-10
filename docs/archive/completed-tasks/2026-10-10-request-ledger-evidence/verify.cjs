const {chromium}=require('/opt/codex/runtimes/cua/lib/node_modules/playwright-core');
const fs=require('fs');
(async()=>{const browser=await chromium.launch({executablePath:'/usr/bin/chromium',headless:true,args:['--no-sandbox']});const evidence=[];
for(const [id,role] of [[101,'user'],[999,'admin']]) {
 const context=await browser.newContext({viewport:{width:1280,height:900},extraHTTPHeaders:{'X-Ledger-Test-User':String(id)}});
 await context.addInitScript(({id,role})=>{localStorage.setItem('auth_token','fixture-only-not-a-credential');localStorage.setItem('auth_user',JSON.stringify({id,role,username:'Synthetic reviewer',email:'fixture@example.invalid',balance:0}));localStorage.setItem(`${role==='admin'?'admin':'user'}_guide_${id}_${role}_v4_interactive`,'true')},{id,role});
 const page=await context.newPage();const errors=[];page.on('pageerror',e=>errors.push(e.message));
 await page.goto(`http://127.0.0.1:18762${role==='admin'?'/admin':''}/requests`,{waitUntil:'networkidle'});
 await page.getByRole('button',{name:'详情',exact:true}).first().waitFor();
 let text=await page.locator('main').innerText();if(!text.includes('用量未知')||!text.includes('待结算'))throw Error('missing uncertainty states');
 if(role==='user'&&await page.locator('input[name=user_id]').count())throw Error('user exposes admin filter');
 const before=await page.locator('main').innerText();await page.reload({waitUntil:'networkidle'});const after=await page.locator('main').innerText();if(!after.includes('/v1/responses'))throw Error('refresh lost records');
 for(const width of [360,768,1280,1920]){await page.setViewportSize({width,height:900});await page.waitForTimeout(400);await page.screenshot({path:`/workspace/request-ledger/docs/visual-reviews/assets/request-ledger/updated-${role}-${width}.png`,fullPage:true,animations:'disabled'});const overflow=await page.evaluate(()=>document.documentElement.scrollWidth>window.innerWidth);if(overflow)throw Error('page overflow '+role+' '+width);}
 await page.setViewportSize({width:1280,height:900});await page.getByRole('button',{name:'详情',exact:true}).first().click();await page.getByRole('dialog').waitFor();await page.waitForTimeout(400);await page.screenshot({path:`/workspace/request-ledger/docs/visual-reviews/assets/request-ledger/detail-${role}-1280.png`,fullPage:false,animations:'disabled'});await page.keyboard.press('Escape');
 await page.evaluate(()=>document.documentElement.classList.add('dark'));await page.screenshot({path:`/workspace/request-ledger/docs/visual-reviews/assets/request-ledger/dark-${role}-1280.png`,fullPage:true,animations:'disabled'});
 evidence.push({role,refresh:true,viewports:[360,768,1280,1920],pageErrors:errors});await context.close();
}
fs.writeFileSync('/tmp/ledger-visual/browser-evidence.json',JSON.stringify(evidence,null,2));await browser.close();console.log(JSON.stringify(evidence));})().catch(e=>{console.error(e);process.exit(1)});
