const {fs,assert,chromium,sql,admin,api,pause}=require('./common.cjs');
async function main(){
 const profile=await api('GET','/auth/me');
 const target=JSON.parse(sql("SELECT row_to_json(x) FROM (SELECT id,name FROM accounts WHERE name LIKE '弹窗复现%' ORDER BY id DESC LIMIT 1)x;"));
 const browser=await chromium.launch({executablePath:'/usr/bin/chromium',headless:true,args:['--no-sandbox']});
 try{
  const context=await browser.newContext({viewport:{width:1280,height:900},reducedMotion:'reduce'});
  await context.addInitScript(({admin,profile})=>{localStorage.setItem('auth_token',admin);localStorage.setItem('auth_user',JSON.stringify(profile));localStorage.setItem(`admin_guide_${profile.id}_admin_v4_interactive`,'true');localStorage.setItem('locale','zh');localStorage.setItem('account-hidden-columns','["usage"]');},{admin,profile});
  const page=await context.newPage();let navigations=0,blocked=0;
  page.on('request',request=>{if(request.isNavigationRequest()&&request.resourceType()==='document')navigations++});
  await page.route('**/EditAccountModal-*.js',route=>{blocked++;return route.abort('failed')});
  const edit=async()=>{await page.getByPlaceholder('搜索账号...').fill(target.name);await pause(500);await page.getByRole('row').filter({has:page.getByText(target.name,{exact:true})}).getByRole('button',{name:'编辑',exact:true}).click()};
  await page.goto('http://127.0.0.1:4176/admin/accounts');await edit();await pause(1000);
  assert.equal(navigations,2,'Exactly one automatic reload');
  await edit();await page.getByText('编辑窗口加载失败，请重新加载页面后重试。',{exact:true}).waitFor();await pause(500);
  assert.equal(navigations,2,'Persistent failure must not loop reloads');assert.equal(await page.locator('#edit-account-form').count(),0);
  await page.screenshot({path:'/workspace/edit-fix-evidence/green-persistent-error.png'});
  await page.unroute('**/EditAccountModal-*.js');await page.reload();await edit();await page.locator('#edit-account-form').waitFor();
  assert.equal(navigations,3);await page.getByRole('button',{name:'取消',exact:true}).click();await edit();await page.locator('#edit-account-form').waitFor();
  const result={status:'pass',blocked,navigations,automatic_reloads:1,error_visible:true,manual_reload_recovered:true,cancel_reopen:true};
  fs.writeFileSync('/workspace/edit-fix-evidence/persistent-chunk-results.json',JSON.stringify(result,null,2));console.log(JSON.stringify(result));
 }finally{await browser.close()}
}
main().catch(e=>{console.error(e.message);process.exit(1)});
