// Local-only integration evidence. Requires disposable containers named
// policy-ui-postgres and policy-ui-redis, the migrated fixture database,
// the actual backend on 127.0.0.1:8080 and built frontend on 127.0.0.1:4174.
// Never point these fixed local endpoints at production. Fixtures are mutated.
// Credentials are generated in memory, never recorded or printed.
const fs=require('fs'),crypto=require('crypto'),assert=require('assert/strict'),{execFileSync}=require('child_process');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const sql=q=>{try{return execFileSync('docker',['exec','-i','policy-ui-postgres','psql','-U','postgres','-d','sub2api_policy_test','-At','-v','ON_ERROR_STOP=1'],{input:q,encoding:'utf8',stdio:['pipe','pipe','pipe']}).trim()}catch{throw new Error('Local fixture SQL failed (details suppressed to protect generated credentials)')}};
const fixtureSecret=sql("SELECT value FROM security_secrets WHERE key='jwt_secret';");
const ids=sql("INSERT INTO users(email,password_hash,role,status) VALUES('local-admin@example.invalid','disabled-fixture','admin','active'),('local-user@example.invalid','disabled-fixture','user','active') ON CONFLICT (email) WHERE deleted_at IS NULL DO UPDATE SET status='active' RETURNING id;").split('\n').filter(x=>/^\d+$/.test(x)).map(Number);
function token(id,role){const version=(crypto.createHash('sha256').update(`local-${role}@example.invalid\ndisabled-fixture`).digest().readBigUInt64BE()&0x7fffffffffffffffn).toString();const b=o=>Buffer.from(JSON.stringify(o).replace('"TOKEN_VERSION"',version)).toString('base64url');const unsigned=b({alg:'HS256',typ:'JWT'})+'.'+b({user_id:id,email:`local-${role}@example.invalid`,role,token_version:"TOKEN_VERSION",sid:crypto.randomBytes(8).toString('hex'),exp:Math.floor(Date.now()/1000)+3600});return unsigned+'.'+crypto.createHmac('sha256',fixtureSecret).update(unsigned).digest('base64url')}
const admin=token(ids[0],'admin'),user=token(ids[1],'user');
const headers=t=>({'Content-Type':'application/json','Idempotency-Key':crypto.randomUUID(),'X-Admin-UI-Request':'1',Authorization:`Bearer ${t}`});
async function api(method,path,body,t=admin,expected=200){const r=await fetch('http://127.0.0.1:8080/api/v1'+path,{method,headers:headers(t),body:body===undefined?undefined:JSON.stringify(body)});const data=await r.json();assert.equal(r.status,expected,`${method} ${path}: unexpected HTTP status ${r.status}`);return data.data;}
const pause=ms=>new Promise(r=>setTimeout(r,ms));
async function until(fn){for(let i=0;i<60;i++){if(await fn())return;await pause(500)}throw new Error('condition timeout');}
async function main(){
 const profile=await api('GET','/auth/me');console.log('Real auth middleware: local admin accepted');sql("DELETE FROM usage_logs WHERE request_id LIKE 'policy-%'; DELETE FROM account_groups WHERE account_id IN(SELECT id FROM accounts WHERE name LIKE '本地%' OR name LIKE '分页夹具%'); DELETE FROM accounts WHERE name LIKE '本地%' OR name LIKE '分页夹具%'; DELETE FROM groups WHERE name LIKE '本地%';");const compliance=await api('GET','/admin/compliance');if(compliance.required)await api('POST','/admin/compliance/accept',{phrase:compliance.ack_phrase_en,language:'en'});
 const createGroup=name=>api('POST','/admin/groups',{name,platform:'openai',rate_multiplier:1,subscription_type:'standard'});
 const suffix=Date.now();const first=await createGroup('本地目标组 '+suffix),second=await createGroup('本地受限组 '+suffix),source=await createGroup('本地来源组 '+suffix);
 const makeAccount=(name,groups)=>api('POST','/admin/accounts',{name,platform:'openai',type:'apikey',credentials:{base_url:'http://127.0.0.1:9',api_key:crypto.randomBytes(16).toString('hex'),model_mapping:{'gpt-5.5':'gpt-5.5','gpt-5.3-codex':'gpt-5.3-codex'}},group_ids:groups,concurrency:1,priority:50,upstream_billing_probe_enabled:false});
 const account=await makeAccount('本地闭环账号 '+suffix,[first.id,second.id,source.id]);
 const newAccount=await makeAccount('本地复制新账号 '+suffix,[source.id]);
 const original={ [first.id]:['gpt-5.5'],[second.id]:['gpt-5.3-*'],[source.id]:['gpt-5.5'] };
 await api('PUT',`/admin/accounts/${account.id}`,{group_allowed_models:original});
 await api('PUT',`/admin/accounts/${newAccount.id}`,{group_allowed_models:{[source.id]:['gpt-5.3-*']}});
 const dbPolicy=()=>JSON.parse(sql(`SELECT COALESCE(jsonb_object_agg(group_id::text,allowed_models),'{}') FROM account_groups WHERE account_id=${account.id};`));
 assert.deepEqual(dbPolicy(),original);
 const key=Number(sql(`INSERT INTO api_keys(user_id,key,name,status) VALUES(${ids[1]},'disabled-policy-fixture-${suffix}','local fixture','inactive') RETURNING id;`).split('\n')[0]);
 sql(`INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,input_tokens,output_tokens,total_cost,account_stats_cost,account_rate_multiplier,actual_cost,created_at) VALUES(${ids[1]},${key},${account.id},'policy-old-${suffix}','gpt-5.5',1000000,200000,10,5,2,99,NOW()-INTERVAL '2 days'),(${ids[1]},${key},${account.id},'policy-today-${suffix}','gpt-5.5',3000,4000,2,1,2,9,NOW());`);
 const stats=await api('GET',`/admin/accounts/${account.id}/today-stats`);assert.equal(stats.lifetime_tokens,1207000);assert.equal(stats.lifetime_cost,12);assert.equal(stats.cost,2);assert.equal(stats.user_cost,9);console.log('Real usage HTTP / DB: retained tokens=1207000 account cost=12; today account cost=2; distinct user charges');
 const assertZero=s=>{assert.equal(s.lifetime_tokens,0);assert.equal(s.lifetime_cost,0)};
 assertZero(await api('GET',`/admin/accounts/${newAccount.id}/today-stats`));
 const zeroBatch=await api('POST','/admin/accounts/today-stats/batch',{account_ids:[newAccount.id]});assertZero(zeroBatch.stats[newAccount.id]);
 console.log('PASS no-history lifetime fields explicitly zero in real single and batch HTTP');
 // Model discovery is local-only and exercises policy/account intersections without forwarding.
 const modelKey='sk-local-'+crypto.randomBytes(24).toString('hex');
 sql(`UPDATE users SET balance=100 WHERE id=${ids[1]}; INSERT INTO api_keys(user_id,key,name,group_id,status) VALUES(${ids[1]},'${modelKey}','local model discovery',${first.id},'active');`);
 const modelIDs=async()=>{const r=await fetch('http://127.0.0.1:8080/v1/models',{headers:{Authorization:`Bearer ${modelKey}`}});assert.equal(r.status,200,'local model discovery');return (await r.json()).data.map(x=>x.id)};
 await api('PUT',`/admin/accounts/${account.id}`,{group_allowed_models:{...original,[first.id]:['gpt-5.5','unsupported-local-model']}});
 await until(async()=>{const models=await modelIDs();return models.includes('gpt-5.5')&&!models.includes('gpt-5.3-codex')&&!models.includes('unsupported-local-model')});
 await api('PUT',`/admin/accounts/${account.id}`,{group_allowed_models:{...original,[first.id]:['gpt-5.3-*']}});
 await until(async()=>{const models=await modelIDs();return models.includes('gpt-5.3-codex')&&!models.includes('gpt-5.5')});
 await api('PUT',`/admin/accounts/${account.id}`,{group_allowed_models:original});
 console.log('PASS real local /v1/models wildcard and account-support intersection; no forwarding call');
 const browser=await chromium.launch({executablePath:'/usr/bin/chromium',headless:true,args:['--no-sandbox']});
 try {
 const context=await browser.newContext({viewport:{width:1600,height:1000},reducedMotion:'reduce'});
 await context.addInitScript(({admin,profile})=>{localStorage.setItem('auth_token',admin);localStorage.setItem('auth_user',JSON.stringify(profile));localStorage.setItem(`admin_guide_${profile.id}_admin_v4_interactive`,'true');localStorage.setItem('locale','zh');localStorage.setItem('theme','light');localStorage.setItem('account-hidden-columns','["proxy","notes","scheduler_score","created_at","last_used_at","rate_multiplier"]');},{admin,profile});
 const page=await context.newPage();page.on('pageerror',e=>console.log('Browser error:',e.message));
 let requests=[];page.on('request',req=>{if(req.method()==='PUT'&&new URL(req.url()).pathname===`/api/v1/admin/accounts/${account.id}`)requests.push(req.postDataJSON());});
 await page.goto('http://127.0.0.1:4174/admin/accounts');
 await page.getByPlaceholder('搜索账号...').fill(account.name);await pause(600);
 const edit=async()=>{const row=page.getByRole('row').filter({has:page.getByText(account.name,{exact:true})});await row.getByRole('button',{name:'编辑',exact:true}).click();await page.locator(`#group-models-${first.id}`).waitFor();await page.locator('[data-testid="account-group-model-limits"]').scrollIntoViewIfNeeded();};
 await edit();assert.equal(await page.locator(`#group-models-${first.id}`).inputValue(),'gpt-5.5');await page.locator(`#group-models-${first.id}`).focus();assert.equal(await page.evaluate(()=>document.activeElement.id),`group-models-${first.id}`);await page.keyboard.press('Tab');assert.notEqual(await page.evaluate(()=>document.activeElement.id),`group-models-${first.id}`);
 // Real update followed by SQL, GET, cache readback, refresh and reopen.
 await page.locator(`#group-models-${first.id}`).fill('gpt-5.3-*\ngpt-5.5');
 const clickCount=requests.length;let response=page.waitForResponse(r=>r.request().method()==='PUT'&&new URL(r.url()).pathname===`/api/v1/admin/accounts/${account.id}`);
 await page.locator('#edit-account-form').evaluate(form=>{form.requestSubmit();form.requestSubmit()});assert.equal((await response).status(),200);await page.locator('#edit-account-form').waitFor({state:'hidden'});assert.equal(requests.length,clickCount+1);
 const changed={...original,[first.id]:['gpt-5.3-*','gpt-5.5']};assert.deepEqual(requests.at(-1).group_allowed_models,changed);assert.deepEqual(dbPolicy(),changed);
 const detail=await api('GET',`/admin/accounts/${account.id}`);assert.deepEqual(detail.account_groups.find(x=>x.group_id===first.id).allowed_models,changed[first.id]);
 await until(()=>{const raw=execFileSync('docker',['exec','policy-ui-redis','redis-cli','--raw','GET',`sched:acc:${account.id}`],{encoding:'utf8'}).trim();if(!raw)return false;const cached=JSON.parse(raw);return JSON.stringify(cached.AccountGroups?.find(x=>x.GroupID===first.id)?.AllowedModels)===JSON.stringify(changed[first.id]);});
 console.log('PASS UI edit -> real PUT -> PostgreSQL -> GET detail -> live Redis scheduler cache');
 await page.reload();await page.getByPlaceholder('搜索账号...').fill(account.name);await pause(600);await edit();assert.equal(await page.locator(`#group-models-${first.id}`).inputValue(),'gpt-5.3-*\ngpt-5.5');
 const assets=require('path').resolve(__dirname, '../../../visual-reviews/assets/account-policy-ui');
 assert.ok(!(await page.locator('#edit-account-form').innerText()).includes('admin.accounts.groupModelLimits'));await page.screenshot({path:assets+'/updated-edit-1600-light.png'});
 await page.getByRole('button',{name:'取消',exact:true}).click();await page.getByTitle('浅色模式',{exact:true}).click();await page.getByRole('button',{name:'深色模式',exact:true}).last().click();await pause(500);await edit();await page.setViewportSize({width:1280,height:900});await page.screenshot({path:assets+'/updated-edit-1280-dark.png'});
 await page.getByRole('button',{name:'取消',exact:true}).click();
 // Cancellation performs no HTTP write, and an unchanged save does not replace policies.
 await edit();let before=requests.length;await page.locator(`#group-models-${first.id}`).fill('cancelled-model');await page.getByRole('button',{name:'取消',exact:true}).click();assert.equal(requests.length,before);assert.deepEqual(dbPolicy(),changed);
 await edit();response=page.waitForResponse(r=>r.request().method()==='PUT'&&new URL(r.url()).pathname===`/api/v1/admin/accounts/${account.id}`);await page.locator('[data-tour="account-form-submit"]').click();await response;await page.locator('#edit-account-form').waitFor({state:'hidden'});assert.equal(requests.at(-1).group_allowed_models,undefined);assert.deepEqual(dbPolicy(),changed);
 console.log('PASS refresh/reopen; cancel no request/write; unrelated save preserves policies');
 // Fail activation, credentials and policy together at the real outbox boundary.
 await api('PUT',`/admin/accounts/${account.id}`,{status:'inactive'});
 const redisAccount=()=>{const raw=execFileSync('docker',['exec','policy-ui-redis','redis-cli','--raw','GET',`sched:acc:${account.id}`],{encoding:'utf8'}).trim();return raw?JSON.parse(raw):null};
 await until(()=>redisAccount()?.Status==='inactive');
 const dbAccount=()=>sql(`SELECT jsonb_build_array(name,status,credentials,proxy_id,rate_multiplier) FROM accounts WHERE id=${account.id};`);
 const beforeAccount=dbAccount();
 await page.reload();await page.getByPlaceholder('搜索账号...').fill(account.name);await pause(600);await edit();await page.locator(`#group-models-${first.id}`).fill('gpt-5.5');
 await page.locator('#edit-account-form label').filter({hasText:/^状态$/}).locator('..').locator('button.select-trigger').click();
 await page.getByRole('option',{name:'启用',exact:true}).click();
 const editedBase='http://127.0.0.1:9/atomic-retry';await page.getByPlaceholder('https://api.openai.com',{exact:true}).fill(editedBase);
 sql(`ALTER TABLE scheduler_outbox ADD CONSTRAINT policy_ui_failure_${account.id} CHECK(account_id IS DISTINCT FROM ${account.id} OR event_type<>'account_groups_changed') NOT VALID;`);
 try{response=page.waitForResponse(r=>r.request().method()==='PUT'&&new URL(r.url()).pathname===`/api/v1/admin/accounts/${account.id}`);await page.locator('[data-tour="account-form-submit"]').click();const failed=await response;assert.equal(failed.status(),500);assert.equal(requests.at(-1).status,'active');assert.equal(requests.at(-1).credentials.base_url,editedBase);assert.ok(dbAccount()===beforeAccount,'account snapshot must remain unchanged');assert.deepEqual(dbPolicy(),changed);const unchanged=await api('GET',`/admin/accounts/${account.id}`);assert.equal(unchanged.status,'inactive');assert.equal(unchanged.credentials.base_url,'http://127.0.0.1:9');assert.equal(redisAccount().Status,'inactive');assert.equal(redisAccount().Credentials.base_url,'http://127.0.0.1:9');assert.equal(await page.locator(`#group-models-${first.id}`).inputValue(),'gpt-5.5');assert.equal(await page.getByPlaceholder('https://api.openai.com',{exact:true}).inputValue(),editedBase);}
 finally{sql(`ALTER TABLE scheduler_outbox DROP CONSTRAINT policy_ui_failure_${account.id};`);}
 response=page.waitForResponse(r=>r.request().method()==='PUT'&&new URL(r.url()).pathname===`/api/v1/admin/accounts/${account.id}`);await page.locator('[data-tour="account-form-submit"]').click();assert.equal((await response).status(),200);await page.locator('#edit-account-form').waitFor({state:'hidden'});assert.deepEqual(dbPolicy(),original);const retried=await api('GET',`/admin/accounts/${account.id}`);assert.equal(retried.status,'active');assert.equal(retried.credentials.base_url,editedBase);await until(()=>redisAccount()?.Status==='active'&&redisAccount()?.Credentials.base_url===editedBase);
 console.log('PASS UI activation + credentials + policy failure: SQL/GET/Redis unchanged; retained form; retry commits all');
 // Illegal fields and ordinary-user permissions hit real HTTP middleware/service.
 await api('PUT',`/admin/accounts/${account.id}`,{group_allowed_models:{'-1':['gpt-5.5']}},admin,400);
 await api('PUT',`/admin/accounts/${account.id}`,{group_allowed_models:{[first.id]:['x'.repeat(201)]}},admin,400);
 await api('PUT',`/admin/accounts/${account.id}`,{group_allowed_models:{[first.id]:'not-an-array'}},admin,400);
 await api('PUT',`/admin/accounts/${account.id}`,{group_allowed_models:{}},user,403);
 await api('GET','/admin/groups',undefined,user,403);assert.deepEqual(dbPolicy(),original);console.log('PASS invalid policies rejected and normal user denied; database unchanged');
 // Real copy endpoint retains target policy and carries restricted new members.
 await page.goto('http://127.0.0.1:4174/admin/groups');await page.reload();await page.getByPlaceholder('搜索分组...').fill(String(suffix));await pause(700);
 await page.getByRole('row').filter({hasText:first.name}).getByRole('button',{name:'编辑',exact:true}).click();
 const copySelect=page.locator('#edit-group-form select').filter({has:page.locator('option').filter({hasText:'选择分组以复制其账号'})});
 await copySelect.selectOption(String(source.id));await copySelect.scrollIntoViewIfNeeded();
 assert.ok((await page.locator('#edit-group-form').innerText()).includes('留存账号保留目标组模型限制'));
 await page.screenshot({path:assets+'/updated-copy-1280-light.png'});
 response=page.waitForResponse(r=>r.request().method()==='PUT'&&new URL(r.url()).pathname===`/api/v1/admin/groups/${first.id}`);
 await page.locator('button[form="edit-group-form"]').click();assert.equal((await response).status(),200);await page.locator('#edit-group-form').waitFor({state:'hidden'});assert.deepEqual(dbPolicy()[first.id],original[first.id]);
 const copied=await api('GET',`/admin/accounts/${newAccount.id}`);assert.deepEqual(copied.account_groups.find(x=>x.group_id===first.id).allowed_models,['gpt-5.3-*']);console.log('PASS Group Copy Accounts real HTTP / DB / GET preserves and inherits policy');
 // Membership-only and bulk operations preserve surviving policy; removal/re-add clears only the removed binding.
 await api('POST','/admin/accounts/bulk-update',{account_ids:[account.id],group_ids:[first.id,second.id,source.id]});assert.deepEqual(dbPolicy(),original);
 await api('PUT',`/admin/accounts/${account.id}`,{group_ids:[first.id,second.id]});assert.deepEqual(dbPolicy(),{[first.id]:original[first.id],[second.id]:original[second.id]});
 await api('PUT',`/admin/accounts/${account.id}`,{group_ids:[first.id,second.id,source.id]});assert.equal(dbPolicy()[source.id],null);
 await api('PUT',`/admin/accounts/${account.id}`,{group_allowed_models:original});
 console.log('PASS bulk rebind, membership removal/re-add readback');
 const clone=await api('POST',`/admin/accounts/${account.id}/duplicate`,{});
 const clonedDetail=await api('GET',`/admin/accounts/${clone.id}`);
 assertZero(await api('GET',`/admin/accounts/${clone.id}/today-stats`));
 const freeClone=await api('POST',`/admin/accounts/${account.id}/duplicate`,{});
 sql(`INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,input_tokens,output_tokens,total_cost,account_stats_cost,account_rate_multiplier,actual_cost,created_at) VALUES(${ids[1]},${key},${freeClone.id},'policy-free-${suffix}','gpt-5.5',3000,4000,2,0,2,9,NOW());`);
 const freeStats=await api('GET',`/admin/accounts/${freeClone.id}/today-stats`);assert.equal(freeStats.lifetime_tokens,7000);assert.equal(freeStats.lifetime_cost,0);assert.equal(freeStats.user_cost,9);
 const batch=await api('POST','/admin/accounts/today-stats/batch',{account_ids:[clone.id,freeClone.id]});assertZero(batch.stats[clone.id]);assert.equal(batch.stats[freeClone.id].lifetime_tokens,7000);assert.equal(batch.stats[freeClone.id].lifetime_cost,0);
 console.log('PASS retained tokens with zero account cost and distinct nonzero user charges; single/batch HTTP');
 for(const [id,models] of Object.entries(original))assert.deepEqual(clonedDetail.account_groups.find(x=>x.group_id===Number(id)).allowed_models,models);
 const clonedGroup=await api('POST',`/admin/groups/${first.id}/duplicate`,{});
 assert.deepEqual(JSON.parse(sql(`SELECT allowed_models FROM account_groups WHERE group_id=${clonedGroup.id} AND account_id=${account.id};`)),original[first.id]);
 console.log('PASS account/group clone HTTP -> database -> detail policy preservation');
 // Two individually valid sources may form an invalid >500-item union.
 const unionA=await createGroup('本地并集甲 '+suffix),unionB=await createGroup('本地并集乙 '+suffix);
 const unionAccount=await makeAccount('本地并集账号 '+suffix,[unionA.id,unionB.id]);
 await api('PUT',`/admin/accounts/${unionAccount.id}`,{group_allowed_models:{[unionA.id]:Array.from({length:300},(_,i)=>'local-a-'+i),[unionB.id]:Array.from({length:300},(_,i)=>'local-b-'+i)}});
 const invalidName='本地失败创建 '+suffix;
 await api('POST','/admin/groups',{name:invalidName,platform:'openai',rate_multiplier:1,copy_accounts_from_group_ids:[unionA.id,unionB.id]},admin,400);
 assert.equal(sql(`SELECT count(*) FROM groups WHERE name='${invalidName}';`),'0');
 const beforeCopy=sql(`SELECT jsonb_agg(jsonb_build_array(account_id,allowed_models) ORDER BY account_id) FROM account_groups WHERE group_id=${first.id};`);
 await api('PUT',`/admin/groups/${first.id}`,{name:'本地失败更新 '+suffix,rate_multiplier:2,copy_accounts_from_group_ids:[unionA.id,unionB.id]},admin,400);
 assert.equal((await api('GET',`/admin/groups/${first.id}`)).name,first.name);
 assert.equal(sql(`SELECT jsonb_agg(jsonb_build_array(account_id,allowed_models) ORDER BY account_id) FROM account_groups WHERE group_id=${first.id};`),beforeCopy);
 console.log('PASS real HTTP oversize-union 400; create no orphan; update attributes/membership/policies all rolled back');

 await page.goto('http://127.0.0.1:4174/admin/accounts');
 await page.reload();await page.getByPlaceholder('搜索账号...').fill(account.name);await pause(700);
 await page.locator('[data-testid="lifetime-tokens"]').first().waitFor();assert.ok((await page.locator('[data-testid="lifetime-tokens"]').first().innerText()).includes('1.21M'));assert.ok((await page.locator('[data-testid="lifetime-cost"]').first().innerText()).includes('$12.00'));
 const statsCells=page.locator('[data-testid="lifetime-cost"]');assert.equal(await statsCells.count(),3);
 const tokenTexts=await page.locator('[data-testid="lifetime-tokens"]').allTextContents();assert.ok(tokenTexts.some(x=>x.includes('7.0K')));assert.ok(tokenTexts.some(x=>/累计 Token.*0$/.test(x.trim())));assert.equal((await statsCells.allTextContents()).filter(x=>x.includes('$0.00')).length,2);
 const assertStatsVisible=async()=>{
   await statsCells.first().evaluate(cell=>{for(let el=cell.parentElement;el;el=el.parentElement){if(el.scrollWidth>el.clientWidth&&/auto|scroll/.test(getComputedStyle(el).overflowX)){
     const wrapper=el.getBoundingClientRect(),target=cell.getBoundingClientRect();let left=wrapper.left,right=wrapper.right;
     for(const header of el.querySelectorAll('thead .sticky-col')){const r=header.getBoundingClientRect();if(header.classList.contains('sticky-col-right'))right=Math.min(right,r.left);else left=Math.max(left,r.right)}
     el.scrollTo({left:el.scrollLeft+target.left+target.width/2-(left+right)/2,behavior:'instant'});break;
   }}});
   for(const cell of await statsCells.all()){
     assert.ok(await cell.evaluate(el=>{const r=el.getBoundingClientRect();if(r.x<0||r.right>innerWidth||r.y<0||r.bottom>innerHeight)return false;return [r.left+2,r.right-2].every(x=>el.contains(document.elementFromPoint(x,r.y+r.height/2)))}),'cumulative stats must be inside viewport and unobscured by sticky columns');
   }
 };
 // Two zero-cost rows fit the smaller viewport; use the real table's horizontal scroll.
 await page.getByPlaceholder('搜索账号...').fill(clone.name);await pause(700);assert.equal(await statsCells.count(),2);await assertStatsVisible();await page.screenshot({path:assets+'/updated-stats-1280-light.png'});
 await page.setViewportSize({width:1600,height:1000});await page.getByPlaceholder('搜索账号...').fill(account.name);await pause(700);assert.equal(await statsCells.count(),3);await page.getByTitle('浅色模式',{exact:true}).click();await page.getByRole('button',{name:'深色模式',exact:true}).last().click();await pause(500);await assertStatsVisible();await page.screenshot({path:assets+'/updated-stats-1600-dark.png'});
 console.log('PASS visible real UI retained totals: 1.21M/$12.00, 7.0K/$0.00, 0/$0.00; 1280 light and 1600 dark');
 await edit();await page.locator(`#group-models-${first.id}`).fill('');response=page.waitForResponse(r=>r.request().method()==='PUT'&&new URL(r.url()).pathname===`/api/v1/admin/accounts/${account.id}`);await page.locator('[data-tour="account-form-submit"]').click();await response;await page.locator('#edit-account-form').waitFor({state:'hidden'});assert.equal(dbPolicy()[first.id],null);assert.deepEqual(dbPolicy()[second.id],original[second.id]);
 await api('PUT',`/admin/accounts/${account.id}`,{group_allowed_models:null});assert.equal(dbPolicy()[first.id],null);
 await api('PUT',`/admin/accounts/${account.id}`,{group_allowed_models:{}});assert.ok(Object.values(dbPolicy()).every(x=>x===null));
 await api('PUT',`/admin/accounts/${account.id}`,{group_allowed_models:original});
 console.log('PASS UI empty-list clear preserves other groups; null preserves; {} clears all');
 sql(`INSERT INTO accounts(name,platform,type,credentials,status) SELECT '分页夹具 ${suffix} '||i,'openai','apikey','{}','inactive' FROM generate_series(1,23) i;`);
 await page.getByPlaceholder('搜索账号...').fill('分页夹具 '+suffix);await pause(700);assert.equal(await page.getByRole('row').filter({hasText:'分页夹具'}).count(),20);
 response=page.waitForResponse(r=>new URL(r.url()).pathname==='/api/v1/admin/accounts'&&new URL(r.url()).searchParams.get('page')==='2');await page.getByRole('button',{name:'下一页',exact:true}).click();await response;await pause(300);assert.equal(await page.getByRole('row').filter({hasText:'分页夹具'}).count(),3);
 await page.getByPlaceholder('搜索账号...').fill('不存在的本地账号');await pause(700);assert.equal(await page.getByRole('row').filter({hasText:'分页夹具'}).count(),0);
 console.log('PASS real pagination and filtering including empty result');
 const normal=await browser.newContext();await normal.addInitScript(({user,id})=>{localStorage.setItem('auth_token',user);localStorage.setItem('auth_user',JSON.stringify({id,email:'local-user@example.invalid',role:'user',status:'active'}));},{user,id:ids[1]});const userPage=await normal.newPage();await userPage.goto('http://127.0.0.1:4174/admin/accounts');await pause(800);assert.ok(!userPage.url().includes('/admin/accounts'));await normal.close();
 console.log('PASS normal-user browser cannot enter admin account management');
 console.log(JSON.stringify({account_id:account.id,group_ids:[first.id,second.id,source.id],fixture:'local PostgreSQL 18.1 / Redis 8.4; no upstream requests',status:'pass'}));
 } finally {await browser.close()}
}
main().catch(e=>{console.error(e.message);process.exitCode=1});
