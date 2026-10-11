import contextlib,hashlib,importlib.util,io,json,pathlib,subprocess,sys,tempfile,time,types,unittest
from unittest.mock import patch
spec=importlib.util.spec_from_file_location('routing_preflight',pathlib.Path(__file__).with_name('run.py'))
m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
MAIN='1'*40;DEP='2'*40
class PreflightTests(unittest.TestCase):
 def fake(self,*unused,missing=False,changed=False,ancestor=True,dirty=False,head=None):
  def call(repo,*args,check=True):
   code=0;value=''
   if args==('rev-parse','HEAD'):value=head or m.FROZEN_UI
   elif args==('rev-parse','origin/play/main'):value=MAIN
   elif args==('status','--porcelain'):value=' M frozen-file' if dirty else ''
   elif args[0]=='show' and args[1].endswith(':routing.go'):code=1 if missing else 0;value='changed' if changed else 'contract'
   elif args[0]=='show':value='admin.GET("/requests/:id/routing", h.RequestLedger.RoutingAdmin)'
   elif args[0]=='merge-base':code=0 if ancestor else 1
   elif args[0]!='cat-file':self.fail('Unexpected mutating git action: '+repr(args))
   return types.SimpleNamespace(returncode=code,stdout=value)
  return call
 def run_inspect(self,**kwargs):
  with patch.object(m,'CONTRACTS',{'routing.go':hashlib.sha256(b'contract').hexdigest()}),patch.object(m,'git',side_effect=self.fake(**kwargs)):
   return m.inspect(pathlib.Path('/not-used'),MAIN,DEP)
 def test_ready_is_read_only(self):self.assertEqual(self.run_inspect()['reviewed_dependency_sha'],DEP)
 def test_missing_dependency_blocks(self):
  with self.assertRaisesRegex(m.Blocked,'lacks routing'):self.run_inspect(missing=True)
 def test_changed_contract_requires_review(self):
  with self.assertRaisesRegex(m.Blocked,'contract differs'):self.run_inspect(changed=True)
 def test_unpublished_dependency_blocks(self):
  with self.assertRaisesRegex(m.Blocked,'not an ancestor'):self.run_inspect(ancestor=False)
 def test_frozen_head_and_clean_tree_required(self):
  for kwargs in [{'dirty':True},{'head':'3'*40}]:
   with self.assertRaises(m.Blocked):self.run_inspect(**kwargs)
 def test_external_services_rejected(self):
  for key in m.EXTERNAL:
   with self.assertRaises(m.Blocked):m.reject_external({key:'never-log-this'})
  m.reject_external({})
 def test_only_exact_sha_allowed(self):
  for value in ['',None,'play/main','--all','abc; command','abc']:
   with self.assertRaises(m.Blocked):m.validate_sha(value)
  self.assertEqual(m.validate_sha(MAIN),MAIN)
 def test_output_rejects_frozen_tree_and_resolved_symlink(self):
  with tempfile.TemporaryDirectory() as tmp:
   root=pathlib.Path(tmp);repo=root/'frozen';repo.mkdir();alias=root/'alias';alias.symlink_to(repo,target_is_directory=True)
   for target in [repo,repo/'docs/result',alias/'docs/result']:
    with self.assertRaises(m.Blocked):m.safe_output(repo,target)
   self.assertEqual(m.safe_output(repo,root/'outside'),root/'outside')
 def test_unsafe_output_has_no_write_side_effect(self):
  with tempfile.TemporaryDirectory() as tmp:
   repo=pathlib.Path(tmp)/'frozen';repo.mkdir();output=repo/'new-evidence'
   with patch.object(sys,'argv',['run.py','--repo',str(repo),'--output',str(output)]),contextlib.redirect_stdout(io.StringIO()) as stdout:
    self.assertEqual(m.main(),3)
   self.assertFalse(output.exists());self.assertFalse(json.loads(stdout.getvalue())['executed'])
 def test_attempted_execution_failure_is_not_reported_as_unexecuted(self):
  with tempfile.TemporaryDirectory() as tmp:
   root=pathlib.Path(tmp);repo=root/'frozen';repo.mkdir();output=root/'out'
   def fake_git(repo,*args,**kwargs):
    if args[0]=='worktree':raise m.Blocked('synthetic worktree failure before mutation')
    return types.SimpleNamespace(returncode=0,stdout='')
   with patch.object(sys,'argv',['run.py','--repo',str(repo),'--output',str(output),'--execute','--main-sha',MAIN,'--dependency-sha',DEP]),patch.object(m,'inspect',return_value={}),patch.object(m,'git',side_effect=fake_git),patch.dict(m.os.environ,{},clear=True),contextlib.redirect_stdout(io.StringIO()):
    self.assertEqual(m.main(),3)
   result=json.loads((output/'result.json').read_text());self.assertTrue(result['executed']);self.assertFalse(result['fixture_started']);self.assertFalse(result['routing_HTTP_PG_passed']);self.assertEqual(result['status'],'failed');self.assertEqual(result['failed_stage'],'isolated_worktree_merge')
 def test_existing_output_symlink_cannot_modify_frozen_file(self):
  with tempfile.TemporaryDirectory() as tmp:
   root=pathlib.Path(tmp);repo=root/'frozen';repo.mkdir();victim=repo/'file';victim.write_text('unchanged');out=root/'out';out.mkdir();(out/'result.json').symlink_to(victim)
   with patch.object(sys,'argv',['run.py','--repo',str(repo),'--output',str(out)]),contextlib.redirect_stdout(io.StringIO()):self.assertEqual(m.main(),3)
   self.assertEqual(victim.read_text(),'unchanged');self.assertTrue((out/'result.json').is_symlink())
 def test_artifact_exclusive_create_refuses_symlinks(self):
  with tempfile.TemporaryDirectory() as tmp:
   root=pathlib.Path(tmp);victim=root/'victim';victim.write_text('unchanged');link=root/'artifact';link.symlink_to(victim)
   with self.assertRaises(FileExistsError):m.write_new(link,'wrong')
   self.assertEqual(victim.read_text(),'unchanged')
 def test_timeout_stops_owned_child_even_when_it_ignores_term(self):
  with tempfile.TemporaryDirectory() as tmp:
   root=pathlib.Path(tmp);pidfile=root/'child.pid'
   child='import signal,time;signal.signal(signal.SIGTERM,signal.SIG_IGN);time.sleep(60)'
   parent='import subprocess,sys,time,pathlib;p=subprocess.Popen([sys.executable,"-c",'+repr(child)+']);pathlib.Path('+repr(str(pidfile))+').write_text(str(p.pid));time.sleep(60)'
   with self.assertRaisesRegex(m.Blocked,'timed out'):m.run_logged([sys.executable,'-c',parent],root,root/'timeout.log',timeout=1)
   pid=int(pidfile.read_text());stat=pathlib.Path('/proc')/str(pid)/'stat'
   for _ in range(40):
    if not stat.exists() or stat.read_text().split()[2]=='Z':break
    time.sleep(.025)
   self.assertTrue(not stat.exists() or stat.read_text().split()[2]=='Z','owned child still running')
 def test_docker_wrapper_labels_only_current_run(self):
  with tempfile.TemporaryDirectory() as tmp:
   root=pathlib.Path(tmp);fake=root/'fake-docker';fake.write_text('#!'+sys.executable+'\nimport sys,json;print(json.dumps(sys.argv[1:]))\n');fake.chmod(0o700)
   with patch.object(m.shutil,'which',return_value=str(fake)):env,real=m.docker_environment(root,dict(m.os.environ),'own-run')
   out=subprocess.check_output([str(root/'owned-bin/docker'),'run','--rm','postgres:test'],env=env,text=True)
   self.assertEqual(json.loads(out),['run','--label','sub2api.routing363.run=own-run','--rm','postgres:test']);self.assertEqual(real,str(fake))
 def test_container_cleanup_uses_only_exact_label_and_returned_ids(self):
  with tempfile.TemporaryDirectory() as tmp:
   root=pathlib.Path(tmp);record=root/'calls';fake=root/'fake-docker'
   fake.write_text('#!'+sys.executable+'\nimport sys,json\nwith open('+repr(str(record))+',"a") as f:f.write(json.dumps(sys.argv[1:])+"\\n")\nif sys.argv[1]=="ps":print("abcdef123456")\n');fake.chmod(0o700)
   result=m.cleanup_docker(str(fake),'own-run',root);calls=[json.loads(s) for s in record.read_text().splitlines()]
   self.assertEqual(calls,[['ps','-aq','--filter','label=sub2api.routing363.run=own-run'],['rm','-f','-v','abcdef123456']]);self.assertEqual(result['leftover_containers_removed'],['abcdef123456'])
 def test_stop_file_write_failure_still_stops_fixture_group(self):
  proc=types.SimpleNamespace(pid=12345)
  with patch.object(m,'write_new',side_effect=OSError('synthetic disk full')),patch.object(m,'stop_group') as cleanup:
   with self.assertRaisesRegex(OSError,'synthetic disk full'):m.finish_fixture(proc,pathlib.Path('/not-written'))
  cleanup.assert_called_once_with(proc)
if __name__=='__main__':unittest.main()
