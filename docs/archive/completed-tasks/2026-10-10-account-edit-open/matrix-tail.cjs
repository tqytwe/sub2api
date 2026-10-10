const {fs,crypto,assert,chromium,sql,admin,user,api,pause}=require('./common.cjs');
const root='/workspace/edit-fix-evidence';
const same=(a,b,message)=>assert.equal(JSON.stringify(a),JSON.stringify(b),message);
const rowHash=id=>sql(`SELECT md5(row_to_json(a)::text) FROM accounts a WHERE id=${id};`);
async function main(){
 const profile=await api('GET','/auth/me');
 const cases=JSON.parse(sql("SELECT json_agg(x) FROM (SELECT id,name,type FROM accounts WHERE name LIKE '弹窗矩阵 %' ORDER BY id DESC LIMIT 6)x;")).reverse();
 const stamp=cases[0].name.split(' ').at(-1);
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
  fs.writeFileSync(root+'/matrix-tail-browser-events.json',JSON.stringify(events,null,2));fs.writeFileSync(root+'/matrix-tail-page-errors.json',JSON.stringify(errors,null,2));await browser.close();
 }
}
main().catch(e=>{console.error(e.message);process.exit(1)});
