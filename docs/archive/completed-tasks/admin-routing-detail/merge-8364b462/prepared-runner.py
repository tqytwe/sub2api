#!/usr/bin/env python3
"""Prepare/run an isolated routing HTTP/PG/UI combination, never an unpublished dependency."""
import argparse,datetime,hashlib,json,os,pathlib,re,shutil,signal,subprocess,sys,time,uuid
BUNDLE=pathlib.Path(__file__).resolve().parent
FROZEN_UI='7cc5c016d409aaccec82ec6b222a4c54b620b6e3'
CONTRACTS={
 'backend/internal/requestledger/routing.go':'b464a8489614817e0898dca44909183c24bc88a5de84356d479ae759054a9058',
 'backend/internal/requestledger/routing_query.go':'39e5c5d43060103a69c1078995a14dd28cf8670d1272e7502f92e07a89132db5',
 'backend/internal/handler/routing_handler.go':'3e6ae154bf6a3c75e7f0f63314464e54968630d99839b3ee7be946ffa0410964',
}
EXTERNAL=('SUB2API_TEST_EXTERNAL_MODE','SUB2API_TEST_POSTGRES_DSN','SUB2API_TEST_REDIS_ADDR')
class Blocked(Exception):pass
def git(repo,*args,check=True):
 proc=subprocess.Popen(['git','-C',str(repo),*args],stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True,start_new_session=True)
 try:stdout,stderr=proc.communicate(timeout=60)
 except subprocess.TimeoutExpired:raise Blocked('Git operation timed out: '+args[0])
 finally:stop_group(proc)
 r=subprocess.CompletedProcess(proc.args,proc.returncode,stdout,stderr)
 if check and r.returncode:raise Blocked('Required git object/operation is unavailable: '+args[0])
 return r
def validate_sha(value):
 if not re.fullmatch('[0-9a-f]{40}',value or ''):raise Blocked('An explicit full reviewed commit SHA is required')
 return value
def reject_external(env):
 if any(env.get(k) for k in EXTERNAL):raise Blocked('External database/Redis mode is forbidden; no values are logged')
def safe_output(repo,output):
 target=output.resolve();frozen=repo.resolve()
 if target==frozen or frozen in target.parents:raise Blocked('Output inside the frozen repository is forbidden; no files were written')
 if target.exists():raise Blocked('Output must be a new exclusive directory; old evidence is never overwritten')
 return target
def write_new(path,content):
 with path.open('x') as stream:stream.write(content)
def stop_group(proc):
 # Every managed command starts a new session. Descendants may remain after the
 # direct process exits, so kill the owned group even if wait() already returns.
 try:os.killpg(proc.pid,signal.SIGTERM)
 except ProcessLookupError:pass
 try:proc.wait(timeout=2)
 except subprocess.TimeoutExpired:pass
 try:os.killpg(proc.pid,signal.SIGKILL)
 except ProcessLookupError:pass
 try:proc.wait(timeout=5)
 except subprocess.TimeoutExpired:raise Blocked('Owned process group could not be reaped')
def finish_fixture(proc,stop):
 try:
  write_new(stop,'stop\n')
  try:return proc.wait(timeout=30)
  except subprocess.TimeoutExpired:stop_group(proc);return proc.returncode
 finally:
  # Disk-full or a conflicting stop file must not skip process cleanup.
  stop_group(proc)
def docker_environment(out,env,tag):
 real=shutil.which('docker')
 if not real:raise Blocked('Docker is required for disposable loopback PG fixtures')
 bindir=out/'owned-bin';bindir.mkdir()
 wrapper=bindir/'docker'
 # The fixture helper invokes docker by PATH. Label only containers created by
 # this invocation, preserving all other arguments and the real docker binary.
 code='#!/usr/bin/env python3\nimport os,sys\nargs=sys.argv[1:]\nif args[:1]==["run"]:args[1:1]=["--label",'+repr('sub2api.routing363.run='+tag)+']\nos.execv('+repr(real)+', ['+repr(real)+']+args)\n'
 write_new(wrapper,code);wrapper.chmod(0o700)
 prepared=env.copy();prepared['PATH']=str(bindir)+os.pathsep+prepared.get('PATH','')
 return prepared,real
def cleanup_docker(real,tag,out):
 # Query an unguessable run label rather than deleting a name prefix, so other
 # agents' fixtures cannot be removed. Never use production/application DSNs.
 proc=subprocess.Popen([real,'ps','-aq','--filter','label=sub2api.routing363.run='+tag],stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True,start_new_session=True)
 try:stdout,_=proc.communicate(timeout=15)
 except subprocess.TimeoutExpired:stop_group(proc);raise Blocked('Owned container lookup timed out')
 finally:stop_group(proc)
 if proc.returncode:raise Blocked('Owned container lookup failed')
 ids=stdout.split()
 if any(not re.fullmatch('[0-9a-f]{12,64}',value) for value in ids):raise Blocked('Unexpected container IDs; cleanup stopped')
 if ids:run_logged([real,'rm','-f','-v',*ids],out,out/'container-cleanup.log',timeout=30)
 return {'label':'sub2api.routing363.run='+tag,'leftover_containers_removed':ids,'status':'completed'}
def inspect(repo,main,dependency):
 validate_sha(main)
 if git(repo,'rev-parse','HEAD').stdout.strip()!=FROZEN_UI:raise Blocked('Frozen #363 worktree HEAD changed; review inputs before using this bundle')
 if git(repo,'status','--porcelain').stdout.strip():raise Blocked('Frozen #363 worktree is dirty; nothing was modified')
 if git(repo,'rev-parse','origin/play/main').stdout.strip()!=main:raise Blocked('Requested main SHA differs from fetched origin/play/main')
 actual={};missing=[]
 for p in CONTRACTS:
  r=git(repo,'show',main+':'+p,check=False)
  if r.returncode:missing.append(p)
  else:actual[p]=hashlib.sha256(r.stdout.encode()).hexdigest()
 if missing:raise Blocked('Formal main lacks routing dependency: '+', '.join(missing))
 if actual!=CONTRACTS:raise Blocked('Published routing contract differs; reread and review the final #359 source before execution')
 for required in ['backend/migrations/279_gateway_routing_evidence.sql','backend/internal/server/routes/admin.go']:
  if git(repo,'cat-file','-e',main+':'+required,check=False).returncode:raise Blocked('Published dependency file missing: '+required)
 registration=git(repo,'show',main+':backend/internal/server/routes/admin.go').stdout
 if 'admin.GET("/requests/:id/routing", h.RequestLedger.RoutingAdmin)' not in registration:raise Blocked('Published admin route registration differs; review before running')
 validate_sha(dependency)
 if git(repo,'merge-base','--is-ancestor',dependency,main,check=False).returncode:raise Blocked('Reviewed dependency is not an ancestor of formal main; unpublished branch is forbidden')
 return {'main_sha':main,'ui_sha':FROZEN_UI,'reviewed_dependency_sha':dependency,'contract_sha256':actual}
def run_logged(args,cwd,log,env=None,timeout=900):
 with log.open('x') as out:
  proc=subprocess.Popen(args,cwd=cwd,env=env,stdout=out,stderr=subprocess.STDOUT,start_new_session=True)
  try:code=proc.wait(timeout=timeout)
  except subprocess.TimeoutExpired:
   stop_group(proc);raise Blocked('Command timed out; owned process group stopped; retained log '+str(log))
  finally:
   # Commands may exit while leaving descendants; this session is ours alone.
   stop_group(proc)
 if code:raise Blocked('Command failed; retained log '+str(log))
def main():
 ap=argparse.ArgumentParser(description=__doc__)
 ap.add_argument('--repo',type=pathlib.Path,default=pathlib.Path('/workspace/sub2api-routing-ui'))
 ap.add_argument('--main-sha');ap.add_argument('--dependency-sha')
 ap.add_argument('--execute',action='store_true',help='Build/run only after the exact reviewed dependency is on formal main')
 ap.add_argument('--output',type=pathlib.Path,default=BUNDLE/'runs'/datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%S%fZ'))
 ap.add_argument('--runner',default='/tmp/p0-run')
 args=ap.parse_args();out=None;stage='preflight';docker_real=None;docker_tag=None
 report={'checked_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'executed':False,'fixture_started':False,'routing_HTTP_PG_passed':False,'production_authentication_proven':False}
 try:
  candidate=safe_output(args.repo,args.output);candidate.mkdir(parents=True,exist_ok=False);out=candidate
  reject_external(os.environ)
  if args.execute:
   validate_sha(args.main_sha);validate_sha(args.dependency_sha)
   git(args.repo,'fetch','origin','refs/heads/play/main:refs/remotes/origin/play/main')
  main_sha=args.main_sha or git(args.repo,'rev-parse','origin/play/main').stdout.strip()
  report.update(inspect(args.repo,main_sha,args.dependency_sha))
  if not args.execute:
   report['status']='preflight_ready_not_executed';return 0
  report['executed']=True;stage='isolated_worktree_merge'
  tag=datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%S%fZ')
  work=pathlib.Path('/workspace')/('sub2api-routing363-pg-'+tag)
  branch='codex/routing363-pg-'+tag
  git(args.repo,'worktree','add','-b',branch,str(work),FROZEN_UI)
  report.update(worktree=str(work),local_only_branch=branch)
  # Only main is merged. The unpublished PR359 branch is never fetched or merged.
  git(work,'merge','--no-ff','--no-commit',main_sha)
  target=work/'backend/internal/requestledger/acceptance/routing_ui_pg_fixture_test.go'
  if target.exists():raise Blocked('Fixture destination already exists; refusing to overwrite')
  shutil.copyfile(BUNDLE/'routing_ui_pg_fixture_test.go',target)
  modules=args.repo/'frontend/node_modules'
  if not modules.exists():raise Blocked('Install the existing locked frontend dependencies in the isolated environment first')
  (work/'frontend/node_modules').symlink_to(modules.resolve(),target_is_directory=True)
  stage='build';run_logged([args.runner,'make','build'],work,out/'build.log')
  dist=pathlib.Path('/tmp')/('ledger-routing363-'+tag);shutil.copytree(work/'backend/internal/web/dist',dist)
  manifest={str(p.relative_to(dist)):hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(dist.rglob('*')) if p.is_file()}
  write_new(out/'assets.json',json.dumps(manifest,indent=2)+'\n')
  report['resource_count']=len(manifest)
  env=os.environ.copy()
  for key in EXTERNAL:env.pop(key,None)
  docker_tag=uuid.uuid4().hex
  env,docker_real=docker_environment(out,env,docker_tag)
  # Two known HTTP/PG contracts, not a claim about the backend author's complete
  # 75-contract suite. Align that final suite separately with the reviewed SHA.
  stage='published_HTTP_contracts'
  run_logged([args.runner,'go','-C','backend','test','-tags=integration','./internal/requestledger/acceptance','-run','^TestRequestLedgerRouting(HTTPAdminIsolationAndDetailFailure|RepositoryPoolIsPrefiltered)$','-count=1'],work,out/'published-routing-tests.log',env=env)
  stop=pathlib.Path('/tmp')/('ledger-routing363-'+tag+'.stop')
  env.update(ROUTING_UI_PG_FIXTURE='1',LEDGER_BROWSER_DIST=str(dist),ROUTING_UI_STOP_FILE=str(stop),SUB2API_TEST_POSTGRES_IMAGE='postgres:18.1-alpine3.23')
  log=out/'fixture.log';fixture=None
  stage='fixture_start'
  with log.open('x') as stream:
   fixture=subprocess.Popen([args.runner,'go','-C','backend','test','-tags=integration','./internal/requestledger/acceptance','-run','^TestRoutingUIRealPGFixture363$','-count=1','-v'],cwd=work,env=env,stdout=stream,stderr=subprocess.STDOUT,start_new_session=True)
   report['fixture_started']=True;stage='fixture_readiness'
   try:
    deadline=time.monotonic()+180
    while True:
     match=re.search(r'^ROUTING_PG_READY (.+)$',log.read_text(),re.M)
     if match:break
     if fixture.poll() is not None:raise Blocked('Real fixture exited before readiness; see fixture.log')
     if time.monotonic()>deadline:raise Blocked('Real fixture readiness timeout')
     time.sleep(.25)
    metadata=out/'fixture.json';write_new(metadata,match.group(1)+'\n')
    stage='browser_HTTP_PG_UI'
    run_logged(['node',str(BUNDLE/'browser.cjs'),str(metadata),str(out/'browser')],work,out/'browser.log',timeout=240)
   finally:
    code=finish_fixture(fixture,stop)
   stage='fixture_final_assertions'
   if code:raise Blocked('Real fixture final assertions failed; see fixture.log')
  if 'ROUTING_PG_FINAL read_only_balance_unchanged=true' not in log.read_text():raise Blocked('Missing final database read-only balance assertion')
  after={str(p.relative_to(dist)):hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(dist.rglob('*')) if p.is_file()}
  if after!=manifest:raise Blocked('Served resources changed during capture')
  report.update(status='passed_scoped_HTTP_PG_UI',executed=True,routing_HTTP_PG_passed=True,resource_hashes_unchanged=True)
  return 0
 except (Blocked,OSError,subprocess.TimeoutExpired) as e:
  report.update(status='failed' if report['executed'] else 'blocked',reason=str(e),failed_stage=stage);return 3
 finally:
  cleanup_failed=False
  if docker_real:
   try:report['owned_container_cleanup']=cleanup_docker(docker_real,docker_tag,out)
   except (Blocked,OSError,subprocess.TimeoutExpired) as e:
    report.update(status='failed',cleanup_error=str(e));cleanup_failed=True
  if out is not None and out.is_dir():write_new(out/'result.json',json.dumps(report,ensure_ascii=False,indent=2)+'\n')
  print(json.dumps(report,ensure_ascii=False))
  if cleanup_failed:return 3
if __name__=='__main__':sys.exit(main())
