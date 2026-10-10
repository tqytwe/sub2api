const {fs,crypto,assert,chromium,sql,admin,api,pause}=require('./common.cjs');
const root='/workspace/edit-fix-evidence';
async function main(){
 const safe=process.env.EXPECT_DRAFT_SAFE==='1';
 const profile=await api('GET','/auth/me');
 const compliance=await api('GET','/admin/compliance');
 if(compliance.required)await api('POST','/admin/compliance/accept',{phrase:compliance.ack_phrase_en,language:'en'});
 const target=await api('POST','/admin/accounts',{name:'同页草稿夹具 '+Date.now(),platform:'openai',type:'apikey',credentials:{base_url:'http://127.0.0.1:9',api_key:crypto.randomBytes(24).toString('hex')},extra:{},status:'inactive',schedulable:false,concurrency:1});
 const before=sql('SELECT count(*) FROM accounts;');
 const browser=await chromium.launch({executablePath:'/usr/bin/chromium',headless:true,args:['--no-sandbox']});
 try{
  const context=await browser.newContext({viewport:{width:1600,height:1000},reducedMotion:'reduce'});
  await context.addInitScript(({admin,profile})=>{localStorage.setItem('auth_token',admin);localStorage.setItem('auth_user',JSON.stringify(profile));localStorage.setItem(`admin_guide_${profile.id}_admin_v4_interactive`,'true');localStorage.setItem('locale','zh');localStorage.setItem('account-hidden-columns','["usage"]');},{admin,profile});
  const page=await context.newPage();let navigations=0,held,writeCount=0;let hit;const routed=new Promise(r=>hit=r);
  page.on('request',r=>{if(r.isNavigationRequest()&&r.resourceType()==='document')navigations++;const path=new URL(r.url()).pathname;if((r.method()==='POST'&&path==='/api/v1/admin/accounts')||(['PUT','PATCH','DELETE'].includes(r.method())&&/^\/api\/v1\/admin\/accounts\/\d+$/.test(path)))writeCount++});
  await page.route('**/EditAccountModal-*.js',route=>{held=route;hit()});
  await page.goto('http://127.0.0.1:4176/admin/accounts');
  const clickEdit=async()=>{await page.getByPlaceholder('搜索账号...').fill(target.name);await pause(500);await page.getByRole('row').filter({has:page.getByText(target.name,{exact:true})}).getByRole('button',{name:'编辑',exact:true}).click()};
  await clickEdit();await routed;
  await page.getByRole('button',{name:'添加账号',exact:true}).click();
  const input=page.locator('#create-account-form [data-tour="account-form-name"]');
  const draft='尚未保存的新增账号草稿';await input.fill(draft);
  await held.abort('failed');await pause(1200);
  const forms=await page.locator('#create-account-form').count();
  const retained=forms ? await input.inputValue() : null;
  const result={status:safe?'GREEN':'RED',scenario:'held Edit chunk -> create account draft -> late chunk failure',navigations,create_form_count:forms,draft_retained:retained===draft,writes:writeCount,sql_count_unchanged:sql('SELECT count(*) FROM accounts;')===before};
  console.log(JSON.stringify(result,null,2));
  await page.screenshot({path:root+'/same-page-draft-'+(safe?'green':'red')+'.png'});
  if(safe){
   assert.equal(navigations,1);assert.equal(retained,draft);assert.equal(writeCount,0);
   assert.equal(await page.getByText('编辑窗口加载失败。请先处理未保存的内容，再刷新页面重试。',{exact:true}).count(),1);
   await page.getByRole('button',{name:'取消',exact:true}).click();
   await page.unroute('**/EditAccountModal-*.js');await page.reload();
   await clickEdit();await page.locator('#edit-account-form').waitFor();
   await page.getByRole('button',{name:'取消',exact:true}).click();await clickEdit();await page.locator('#edit-account-form').waitFor();
   assert.equal(navigations,2,'only the explicit user reload navigates');assert.equal(writeCount,0);
   assert.equal(sql('SELECT count(*) FROM accounts;'),before);
   console.log('PASS draft preserved; cancel does not write; explicit reload after restoration opens editor; cancel/reopen succeeds; no automatic reload');
  }else{assert.equal(navigations,2);assert.equal(forms,0);assert.equal(retained,null);assert.equal(writeCount,0)}
 }finally{await browser.close()}
}
main().catch(e=>{console.error(e.message);process.exit(1)});
