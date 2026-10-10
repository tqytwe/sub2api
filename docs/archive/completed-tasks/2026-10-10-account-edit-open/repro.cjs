const {fs,crypto,assert,chromium,sql,admin,api,pause}=require('./common.cjs');
const root='/workspace/edit-fix-evidence';
const safe=s=>s.replace(/Bearer\s+[^\s"']+/g,'Bearer [redacted]');
async function main(){
 const profile=await api('GET','/auth/me');
 const compliance=await api('GET','/admin/compliance');
 if(compliance.required)await api('POST','/admin/compliance/accept',{phrase:compliance.ack_phrase_en,language:'en'});
 const accounts=[];
 for(const type of ['apikey','oauth']){
  const credentials={base_url:'http://127.0.0.1:9',expires_at:'2099-01-01T00:00:00Z',model_mapping:{'gpt-5.5':'gpt-5.5'}};
  credentials[type==='apikey'?'api_key':'access_token']=crypto.randomBytes(24).toString('hex');
  const name='弹窗复现 '+type;
  const row=sql(`INSERT INTO accounts(name,platform,type,credentials,extra,status,schedulable) VALUES('${name}','openai','${type}','${JSON.stringify(credentials)}','{}','inactive',false) RETURNING id;`);
  accounts.push({id:Number(row.split('\n')[0]),name});
 }
 fs.writeFileSync(root+'/fixture-ids.json',JSON.stringify(accounts));
 const browser=await chromium.launch({executablePath:'/usr/bin/chromium',headless:true,args:['--no-sandbox']});
 try{
  const context=await browser.newContext({viewport:{width:1600,height:1000},reducedMotion:'reduce'});
  await context.addInitScript(({admin,profile})=>{
   localStorage.setItem('auth_token',admin);localStorage.setItem('auth_user',JSON.stringify(profile));
   localStorage.setItem(`admin_guide_${profile.id}_admin_v4_interactive`,'true');localStorage.setItem('locale','zh');
   localStorage.setItem('account-hidden-columns',JSON.stringify(['proxy','notes','scheduler_score','created_at','last_used_at','rate_multiplier','usage']));
  },{admin,profile});
  const page=await context.newPage();const events=[];let blocks=0;
  page.on('console',m=>{if(m.type()==='error')events.push({event:'console',message:safe(m.text())})});
  page.on('pageerror',e=>events.push({event:'pageerror',message:safe(e.message)}));
  page.on('response',r=>{const p=new URL(r.url()).pathname;if(/\/admin\/accounts\/\d+$/.test(p))events.push({event:'detail',path:p,status:r.status()})});
  await page.route('**/EditAccountModal-*.js',r=>{blocks++;events.push({event:'blocked-chunk',path:new URL(r.request().url()).pathname});return r.abort('failed')});
  await page.goto('http://127.0.0.1:4176/admin/accounts');
  await page.getByPlaceholder('搜索账号...').fill('弹窗复现');await pause(800);
  const click=async a=>{await page.getByRole('row').filter({has:page.getByText(a.name,{exact:true})}).getByRole('button',{name:'编辑',exact:true}).click();await pause(700)};
  await click(accounts[0]);
  events.push({event:'first-click',dialog:await page.locator('#edit-account-form').count(),blocks});
  await page.screenshot({path:root+'/red-chunk-first.png'});
  await page.unroute('**/EditAccountModal-*.js');
  await click(accounts[1]);
  events.push({event:'network-restored-second-account',dialog:await page.locator('#edit-account-form').count(),blocks});
  await click(accounts[0]);
  events.push({event:'network-restored-third-click',dialog:await page.locator('#edit-account-form').count(),blocks});
  await page.screenshot({path:root+'/red-chunk-retry.png'});
  fs.writeFileSync(root+'/red-browser-events.json',JSON.stringify(events,null,2));
  for(const e of events)console.log(JSON.stringify(e));
  assert.equal(await page.locator('#edit-account-form').count(),1,'Editing should recover after the chunk network failure clears');
 }finally{await browser.close()}
}
main().catch(e=>{console.error(e.message);process.exit(1)});
