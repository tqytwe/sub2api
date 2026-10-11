#!/usr/bin/env python3
import sys,pathlib,os,uuid,subprocess,re,json,hashlib,time,shutil,datetime
sys.path.insert(0,'/workspace/routing-363-pg-harness')
from run import docker_environment,cleanup_docker,run_logged,write_new,finish_fixture,reject_external
work=pathlib.Path('/workspace/sub2api-routing363-pg-20261011T124516847659Z')
out=pathlib.Path(sys.argv[1]);out.mkdir(parents=True,exist_ok=False)
tag=uuid.uuid4().hex
report={'started_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'main':'8364b462feda15f3bf8f3bb2029314d2193b9dba','ui':'7cc5c016d409aaccec82ec6b222a4c54b620b6e3','worktree':str(work),'passed':False}
reject_external(os.environ)
env,docker=docker_environment(out,os.environ.copy(),tag)
dist=pathlib.Path('/tmp/ledger-routing363-'+tag);shutil.copytree(work/'backend/internal/web/dist',dist)
manifest={str(p.relative_to(dist)):hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(dist.rglob('*')) if p.is_file()}
write_new(out/'assets.json',json.dumps(manifest,indent=2)+'\n')
stop=pathlib.Path('/tmp/ledger-routing363-'+tag+'.stop')
env.update(ROUTING_UI_PG_FIXTURE='1',LEDGER_BROWSER_DIST=str(dist),ROUTING_UI_STOP_FILE=str(stop),SUB2API_TEST_POSTGRES_IMAGE='postgres:18.1-alpine3.23',GIN_MODE='release')
try:
 with (out/'fixture.log').open('x') as log:
  proc=subprocess.Popen(['/tmp/p0-run','go','-C','backend','test','-tags=integration','./internal/requestledger/acceptance','-run','^TestRoutingUIRealPGFixture363$','-count=1','-v'],cwd=work,env=env,stdout=log,stderr=subprocess.STDOUT,start_new_session=True)
  try:
   deadline=time.monotonic()+180
   while True:
    match=re.search(r'^ROUTING_PG_READY (.+)$',(out/'fixture.log').read_text(),re.M)
    if match:break
    if proc.poll() is not None:raise RuntimeError('fixture exited before readiness')
    if time.monotonic()>deadline:raise RuntimeError('fixture readiness timeout')
    time.sleep(.2)
   write_new(out/'fixture.json',match.group(1)+'\n')
   run_logged(['node','/workspace/routing363-real-browser.cjs',str(out/'fixture.json'),str(out/'browser')],work,out/'browser.log',timeout=240)
  finally:code=finish_fixture(proc,stop)
 if code:raise RuntimeError('fixture final assertions failed')
 assert 'ROUTING_PG_FINAL read_only_balance_unchanged=true' in (out/'fixture.log').read_text()
 assert manifest=={str(p.relative_to(dist)):hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(dist.rglob('*')) if p.is_file()}
 report.update(passed=True,resource_count=len(manifest),resource_hashes_unchanged=True,auth_scope='real JWT/AdminAuth/PG user checks, fixture-only token issuance; no password/MFA/binding/refresh/prod proof')
except Exception as e:
 report['error']=str(e)
 raise
finally:
 report['cleanup']=cleanup_docker(docker,tag,out)
 report['finished_at']=datetime.datetime.now(datetime.timezone.utc).isoformat()
 write_new(out/'result.json',json.dumps(report,indent=2)+'\n')
 print(json.dumps(report))
