const {fs,crypto,assert,chromium,sql,admin,user,api,pause}=require('./common.cjs');
const root='/workspace/edit-fix-evidence';
const same=(a,b,message)=>assert.equal(JSON.stringify(a),JSON.stringify(b),message);
const rowHash=id=>sql(`SELECT md5(row_to_json(a)::text) FROM accounts a WHERE id=${id};`);
async function main(){
 const profile=await api('GET','/auth/me'),stamp=Date.now();
 const group=await api('POST','/admin/groups',{name:'弹窗矩阵组 '+stamp,platform:'openai',rate_multiplier:1,subscription_type:'standard'});
 const cases=[];
 for(const type of ['apikey','oauth'])for(const shape of ['complete','null','legacy']){
  const credentials=shape==='null'?null:{base_url:'http://127.0.0.1:9',expires_at:'2099-01-01T00:00:00Z',model_mapping:shape==='legacy'?null:{'gpt-5.5':'gpt-5.5'},temp_unschedulable_rules:null,openai_capabilities:null};
  if(credentials)credentials[type==='apikey'?'api_key':'access_token']=crypto.randomBytes(24).toString('hex');
  const extra=shape==='null'?null:{fixture_preserve:'kept',...(type==='apikey'?{upstream_billing_probe_enabled:false}:{}),auto_pause_5h_threshold:null,openai_compact_mode:'auto'};
  const name=`弹窗矩阵 ${type} ${shape} ${stamp}`;
  const legacyPrefix=shape==='null'?'SET session_replication_role=replica; ':'';
  const legacySuffix=shape==='null'?' SET session_replication_role=origin;':'';
  const id=Number(sql(`${legacyPrefix}INSERT INTO accounts(name,platform,type,credentials,extra,status,schedulable,concurrency,priority) VALUES('${name}','openai','${type}','${JSON.stringify(credentials)}','${JSON.stringify(extra)}','inactive',false,2,50) RETURNING id;${legacySuffix}`).split('\n').find(line=>/^\d+$/.test(line)));
  sql(`INSERT INTO account_groups(account_id,group_id,priority,allowed_models) VALUES(${id},${group.id},10,${shape==='complete'?"'[\"gpt-5.5\"]'::jsonb":'NULL'});`);
  cases.push({id,name,type,shape});
 }
 const browser=await chromium.launch({executablePath:'/usr/bin/chromium',headless:true,args:['--no-sandbox']});
 const events=[],errors=[];
 try{
  const context=await browser.newContext({viewport:{width:1600,height:1000},reducedMotion:'reduce'});
  await context.addInitScript(({admin,profile})=>{localStorage.setItem('auth_token',admin);localStorage.setItem('auth_user',JSON.stringify(profile));localStorage.setItem(`admin_guide_${profile.id}_admin_v4_interactive`,'true');localStorage.setItem('locale','zh');localStorage.setItem('theme','light');localStorage.setItem('account-hidden-columns',JSON.stringify(['proxy','notes','scheduler_score','created_at','last_used_at','rate_multiplier','usage']));},{admin,profile});
  const page=await context.newPage();let puts=[];
  page.on('pageerror',e=>errors.push(e.message));
  page.on('console',m=>{if(m.type()==='error')events.push({event:'console',message:m.text()})});
  page.on('request',r=>{if(r.method()==='PUT'&&/\/admin\/accounts\/\d+$/.test(new URL(r.url()).pathname))puts.push({id:Number(r.url().split('/').at(-1)),data:r.postDataJSON()})});
  page.on('response',r=>{const path=new URL(r.url()).pathname;if(/\/admin\/accounts\/\d+$/.test(path))events.push({method:r.request().method(),path,status:r.status()})});
  await page.goto('http://127.0.0.1:4176/admin/accounts');
  await page.getByPlaceholder('搜索账号...').fill(String(stamp));await pause(600);
  const edit=async c=>{await page.getByRole('row').filter({has:page.getByText(c.name,{exact:true})}).getByRole('button',{name:'编辑',exact:true}).click();await page.locator('#edit-account-form').waitFor();};
  const cancel=async()=>{await page.getByRole('button',{name:'取消',exact:true}).click();await page.locator('#edit-account-form').waitFor({state:'hidden'})};
  const fields=()=>page.locator('#edit-account-form').evaluate(form=>Array.from(form.querySelectorAll('input:not([type=password]),textarea,select,[role=button][aria-haspopup=listbox]')).map(e=>({tag:e.tagName,type:e.type||'',id:e.id,testid:e.getAttribute('data-testid'),value:'value'in e?e.value:e.innerText,checked:e.type==='checkbox'?e.checked:undefined})));
  for(const c of cases){
   const old=await api('GET',`/admin/accounts/${c.id}`);assert.equal(old.id,c.id);
   const hash=rowHash(c.id),writes=puts.length;
   await edit(c);await page.getByPlaceholder('请输入备注').fill('cancelled');await cancel();
   assert.equal(puts.length,writes);assert.equal(rowHash(c.id),hash);
   await edit(c);assert.equal(await page.getByPlaceholder('请输入备注').inputValue(),'');
   await page.getByPlaceholder('请输入备注').fill(`saved ${c.type} ${c.shape}`);
   await page.locator('[data-tour="account-form-priority"]').fill('73');
   await page.getByTestId('account-rate-multiplier').fill('1.25');
   await page.locator(`#group-models-${group.id}`).fill('gpt-5.5\ngpt-5.3-*');
   if(c.type==='apikey'){
    await page.getByPlaceholder('https://api.openai.com',{exact:true}).fill('http://127.0.0.1:9');
    if(c.shape==='null')await page.getByPlaceholder('sk-proj-...',{exact:true}).fill(crypto.randomBytes(24).toString('hex'));
   }
   const intended=await fields();
   const response=page.waitForResponse(r=>r.request().method()==='PUT'&&new URL(r.url()).pathname===`/api/v1/admin/accounts/${c.id}`);
   await page.locator('#edit-account-form').evaluate(form=>{form.requestSubmit();form.requestSubmit()});
   assert.equal((await response).status(),200);await page.locator('#edit-account-form').waitFor({state:'hidden'});
   assert.equal(puts.length,writes+1,'Duplicate submit must only write once');
   const sent=puts.at(-1).data,detail=await api('GET',`/admin/accounts/${c.id}`);
   const stored=JSON.parse(sql(`SELECT row_to_json(a) FROM accounts a WHERE id=${c.id};`));
   for(const key of ['name','notes','concurrency','priority','rate_multiplier','status','auto_pause_on_expired']){
    same(detail[key],sent[key],`${key} HTTP readback`);same(stored[key],sent[key],`${key} SQL readback`);
   }
   for(const key of ['credentials','extra'])for(const [field,value] of Object.entries(sent[key]||{})){
    // Never include generated credential values in assertion output.
    assert(JSON.stringify(stored[key]?.[field])===JSON.stringify(value),`${key}.${field}: SQL did not persist submitted value`);
   }
   same(detail.account_groups.find(g=>g.group_id===group.id).allowed_models,['gpt-5.5','gpt-5.3-*'],'Policy HTTP readback');
   same(JSON.parse(sql(`SELECT allowed_models FROM account_groups WHERE account_id=${c.id} AND group_id=${group.id};`)),['gpt-5.5','gpt-5.3-*'],'Policy SQL readback');
   if(c.shape!=='null')assert.equal(detail.extra.fixture_preserve,'kept');
   await page.reload();await page.getByPlaceholder('搜索账号...').fill(String(stamp));await pause(500);await edit(c);
   const reread=await fields();assert(JSON.stringify(reread)===JSON.stringify(intended),`All editor controls must survive refresh: ${c.type}/${c.shape}`);
   await cancel();console.log(`PASS ${c.type}/${c.shape}: open/cancel/reopen; double-submit single write; payload fields -> SQL -> GET -> full editor control readback`);
  }
  const target=cases[0],before=rowHash(target.id),count=puts.length;
  const detailURL=url=>url.pathname===`/api/v1/admin/accounts/${target.id}`;let detailFaults=0;
  await page.route(detailURL,r=>{detailFaults++;return r.fulfill({status:503,contentType:'application/json',body:JSON.stringify({code:503,message:'Local injected detail failure'})})});
  await page.getByRole('row').filter({has:page.getByText(target.name,{exact:true})}).getByRole('button',{name:'编辑',exact:true}).click();
  await page.getByText('Local injected detail failure',{exact:true}).waitFor();assert.equal(await page.locator('#edit-account-form').count(),0);
  assert.equal(detailFaults,1,'Detail fault injection must hit');await page.unroute(detailURL);await edit(target);await cancel();assert.equal(puts.length,count);assert.equal(rowHash(target.id),before);
  console.log('PASS detail HTTP 503: visible error, no dialog/write; real GET recovery opens on next click');
  const profileURL=url=>url.pathname==='/api/v1/admin/tls-fingerprint-profiles';let profileFaults=0;await page.route(profileURL,r=>{profileFaults++;return r.fulfill({status:503,contentType:'application/json',body:JSON.stringify({code:503,message:'Local optional profile failure'})})});
  await edit(target);await pause(300);await cancel();assert.equal(profileFaults,1,'Optional profile fault injection must hit');await page.unroute(profileURL);await edit(target);
  await page.screenshot({path:root+'/green-edit-light.png'});await cancel();
  await page.getByTitle('浅色模式',{exact:true}).click();await page.getByRole('button',{name:'深色模式',exact:true}).last().click();await page.setViewportSize({width:1280,height:900});await edit(cases[3]);
  await page.screenshot({path:root+'/green-edit-dark.png'});await cancel();
  console.log('PASS optional profile error does not block opening; Chinese light/dark and cancel/reopen');
  await api('GET',`/admin/accounts/${target.id}`,undefined,user,403);await api('PUT',`/admin/accounts/${target.id}`,{notes:'denied'},user,403);
  assert.equal(rowHash(target.id),before);assert.equal(errors.length,0,'No runtime page errors for valid null/legacy/detail-error cases');
  console.log('PASS real normal-user GET/PUT denied; account unchanged; no runtime page errors');
 }finally{
  fs.writeFileSync(root+'/matrix-browser-events.json',JSON.stringify(events,null,2));fs.writeFileSync(root+'/matrix-page-errors.json',JSON.stringify(errors,null,2));await browser.close();
 }
}
main().catch(e=>{console.error(e.message);process.exit(1)});
