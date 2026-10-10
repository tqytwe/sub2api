const {fs,crypto,assert,chromium,sql,admin,api,pause}=require('./common.cjs');
const green=process.env.EXPECT_RACE_FIXED==='1';const root='/workspace/edit-fix-evidence';
async function main(){
 const stamp=Date.now(),profile=await api('GET','/auth/me');const compliance=await api('GET','/admin/compliance');if(compliance.required)await api('POST','/admin/compliance/accept',{phrase:compliance.ack_phrase_en,language:'en'});const accounts=[];
 for(const label of ['A','B'])accounts.push(await api('POST','/admin/accounts',{name:`详情竞争 ${label} ${stamp}`,notes:`original ${label}`,platform:'openai',type:'apikey',status:'inactive',schedulable:false,concurrency:2,priority:50,credentials:{api_key:crypto.randomBytes(24).toString('hex'),base_url:'http://127.0.0.1:9'},extra:{upstream_billing_probe_enabled:false}}));
 const [a,b]=accounts;const events=[];const browser=await chromium.launch({executablePath:'/usr/bin/chromium',headless:true,args:['--no-sandbox']});
 try{
  const context=await browser.newContext({viewport:{width:1600,height:1000}});
  await context.addInitScript(({admin,profile})=>{localStorage.setItem('auth_token',admin);localStorage.setItem('auth_user',JSON.stringify(profile));localStorage.setItem(`admin_guide_${profile.id}_admin_v4_interactive`,'true');localStorage.setItem('locale','zh');localStorage.setItem('account-hidden-columns','["usage"]');},{admin,profile});
  const page=await context.newPage();const puts=[];page.on('request',r=>{if(r.method()==='PUT'&&/\/admin\/accounts\/\d+$/.test(new URL(r.url()).pathname))puts.push(Number(new URL(r.url()).pathname.split('/').at(-1)))});
  await page.goto('http://127.0.0.1:4176/admin/accounts');await page.getByPlaceholder('搜索账号...').fill(String(stamp));await pause(500);
  const click=c=>page.getByRole('row').filter({has:page.getByText(c.name,{exact:true})}).getByRole('button',{name:'编辑',exact:true}).click();
  const accountName=()=>page.locator('#edit-account-form input').first().inputValue();
  async function race(close){
   let release,hit;const ready=new Promise(resolve=>{hit=resolve});
   const path=url=>url.pathname===`/api/v1/admin/accounts/${a.id}`;
   await page.route(path,async route=>{const response=await route.fetch();assert.equal(response.status(),200);release=()=>route.fulfill({response});hit()});
   await click(a);await ready;await click(b);await page.locator('#edit-account-form').waitFor();assert.equal(await accountName(),b.name);
   if(close){await page.getByRole('button',{name:'取消',exact:true}).click();await page.locator('#edit-account-form').waitFor({state:'hidden'})}
   else await page.getByPlaceholder('请输入备注').fill('draft intended for B');
   await release();await page.unroute(path);await pause(400);
   const opened=await page.locator('#edit-account-form').count();const current=opened?await accountName():null;
   events.push({scenario:close?'cancel B then late A':'select A then B, late A',last_selected:b.id,modal_name:current,opened});
   if(close){assert.equal(opened,green?0:1);if(opened)await page.getByRole('button',{name:'取消',exact:true}).click();return}
   assert.equal(current,green?b.name:a.name);
   const retainedDraft=await page.getByPlaceholder('请输入备注').inputValue()==='draft intended for B';assert.equal(retainedDraft,green);
   await page.getByPlaceholder('请输入备注').fill('save intended for latest selection B');
   const saved=page.waitForResponse(r=>r.request().method()==='PUT'&&/\/admin\/accounts\/\d+$/.test(new URL(r.url()).pathname));
   await page.getByRole('button',{name:'更新',exact:true}).click();assert.equal((await saved).status(),200);await page.locator('#edit-account-form').waitFor({state:'hidden'});
   const actual=puts.at(-1),expected=green?b.id:a.id;assert.equal(actual,expected);
   const stored=JSON.parse(sql(`SELECT json_object_agg(id,notes) FROM accounts WHERE id IN (${a.id},${b.id});`));
   assert.equal(stored[expected],'save intended for latest selection B');assert.equal(stored[green?a.id:b.id],green?'original A':'original B');
   const detail=await api('GET',`/admin/accounts/${expected}`);assert.equal(detail.notes,stored[expected]);
   events.push({scenario:'save destination',last_selected:b.id,actual_put_account:actual,sql_and_get_confirmed:true,draft_preserved:retainedDraft});
   await page.reload();await page.getByPlaceholder('搜索账号...').fill(String(stamp));await pause(500);
  }
  await race(false);await race(true);
  assert.equal(puts.length,1);console.log(JSON.stringify({status:green?'GREEN':'RED defect reproduced',events},null,2));
 }finally{await browser.close()}
}
main().catch(e=>{console.error(e.message);process.exit(1)});
