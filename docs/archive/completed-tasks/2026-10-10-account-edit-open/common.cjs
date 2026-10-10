const fs=require('fs'),crypto=require('crypto'),assert=require('assert/strict'),{execFileSync}=require('child_process');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const sql=q=>{try{return execFileSync('docker',['exec','-i','edit-fix-postgres','psql','-U','postgres','-d','sub2api_edit_test','-At','-v','ON_ERROR_STOP=1'],{input:q,encoding:'utf8',stdio:['pipe','pipe','pipe']}).trim()}catch{throw new Error('Local fixture SQL failed (details suppressed to protect generated credentials)')}};
const fixtureSecret=sql("SELECT value FROM security_secrets WHERE key='jwt_secret';");
const ids=sql("INSERT INTO users(email,password_hash,role,status) VALUES('local-admin@example.invalid','disabled-fixture','admin','active'),('local-user@example.invalid','disabled-fixture','user','active') ON CONFLICT (email) WHERE deleted_at IS NULL DO UPDATE SET status='active' RETURNING id;").split('\n').filter(x=>/^\d+$/.test(x)).map(Number);
function token(id,role){const version=(crypto.createHash('sha256').update(`local-${role}@example.invalid\ndisabled-fixture`).digest().readBigUInt64BE()&0x7fffffffffffffffn).toString();const b=o=>Buffer.from(JSON.stringify(o).replace('"TOKEN_VERSION"',version)).toString('base64url');const unsigned=b({alg:'HS256',typ:'JWT'})+'.'+b({user_id:id,email:`local-${role}@example.invalid`,role,token_version:"TOKEN_VERSION",sid:crypto.randomBytes(8).toString('hex'),exp:Math.floor(Date.now()/1000)+3600});return unsigned+'.'+crypto.createHmac('sha256',fixtureSecret).update(unsigned).digest('base64url')}
const admin=token(ids[0],'admin'),user=token(ids[1],'user');
const headers=t=>({'Content-Type':'application/json','Idempotency-Key':crypto.randomUUID(),'X-Admin-UI-Request':'1',Authorization:`Bearer ${t}`});
async function api(method,path,body,t=admin,expected=200){const r=await fetch('http://127.0.0.1:8082/api/v1'+path,{method,headers:headers(t),body:body===undefined?undefined:JSON.stringify(body)});const data=await r.json();assert.equal(r.status,expected,`${method} ${path}: unexpected HTTP status ${r.status}`);return data.data;}
const pause=ms=>new Promise(r=>setTimeout(r,ms));
async function until(fn){for(let i=0;i<60;i++){if(await fn())return;await pause(500)}throw new Error('condition timeout');}

module.exports={fs,crypto,assert,execFileSync,chromium,sql,admin,user,api,pause,until};
