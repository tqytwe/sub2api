const {fs,assert,chromium,sql,admin,api,pause}=require('./common.cjs');
async function main(){
 const profile=await api('GET','/auth/me');
 const target=JSON.parse(sql("SELECT row_to_json(x) FROM (SELECT id,name FROM accounts WHERE name LIKE '弹窗矩阵 %' ORDER BY id DESC LIMIT 1)x;"));
 const browser=await chromium.launch({executablePath:'/usr/bin/chromium',headless:true,args:['--no-sandbox']});
 try{
  const context=await browser.newContext({viewport:{width:1600,height:1000}});
  await context.addInitScript(({admin,profile})=>{localStorage.setItem('auth_token',admin);localStorage.setItem('auth_user',JSON.stringify(profile));localStorage.setItem(`admin_guide_${profile.id}_admin_v4_interactive`,'true');localStorage.setItem('locale','zh');localStorage.setItem('account-hidden-columns','["usage"]');},{admin,profile});
  const page=await context.newPage();let navigations=0,held,putCount=0;const errors=[];
  page.on('request',r=>{if(r.isNavigationRequest()&&r.resourceType()==='document')navigations++;if(r.method()==='PUT')putCount++});
  page.on('pageerror',e=>errors.push(e.message));
  let hit;const routed=new Promise(resolve=>{hit=resolve});
  await page.route('**/EditAccountModal-*.js',route=>{held=route;hit()});
  await page.goto('http://127.0.0.1:4176/admin/accounts');await page.getByPlaceholder('搜索账号...').fill(target.name);await pause(500);
  await page.getByRole('row').filter({has:page.getByText(target.name,{exact:true})}).getByRole('button',{name:'编辑',exact:true}).click();await routed;
  await page.locator('a[href="/admin/users"]').click();await page.waitForURL('**/admin/users');
  await page.evaluate(()=>{window.accountEditRouteSentinel='preserved'});
  await held.abort('failed');await pause(1200);
  assert.equal(navigations,1,'Late editor failure must not reload the unrelated page');
  assert.equal(await page.evaluate(()=>window.accountEditRouteSentinel),'preserved','Unrelated page state preserved');
  assert.equal(await page.getByText('编辑窗口加载失败，请重新加载页面后重试。',{exact:true}).count(),0);
  assert.equal(putCount,0);assert.equal(errors.length,0);
  console.log('PASS pending edit module -> navigate to users -> reject: no document reload, no unrelated error toast, state preserved, no PUT/runtime errors');
 }finally{await browser.close()}
}
main().catch(e=>{console.error(e.message);process.exit(1)});
