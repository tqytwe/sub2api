#!/usr/bin/env python3
import pathlib,subprocess,datetime,json,hashlib,sys,ast,re
w=pathlib.Path('/workspace/sub2api-routing363-pg-20261011T124516847659Z')
out=pathlib.Path('/workspace/routing363-836-full');out.mkdir(exist_ok=False)
cmds=[('make-test',['make','test']),('tag-unit',['make','test-backend-unit']),('ledger-contracts',['go','-C','backend','test','-tags=integration','./internal/requestledger/...','./internal/handler','./internal/server/routes','./internal/repository','./internal/service','-run','RequestLedger|LedgerPostgres|LedgerKilledProcess|LedgerStorage|BatchImageHTTP|BatchImageReconciliation|BatchImageLedgerMigration','-count=1','-timeout=12m','-json']),('embed-build',['go','-C','backend','build','-tags=embed','-buildvcs=false','-trimpath','-o','/tmp/routing363-836-embed','./cmd/server']),('embed-test',['go','-C','backend','test','-tags=embed','./internal/web','./cmd/server']),('fork',['./scripts/check-fork-integrity.sh'])]
rows=[]
for name,cmd in cmds:
 start=datetime.datetime.now(datetime.timezone.utc);log=out/(name+'.log')
 with log.open('x') as stream:r=subprocess.run(['/tmp/p0-run',*cmd],cwd=w,stdout=stream,stderr=subprocess.STDOUT)
 rows.append({'name':name,'command':cmd,'started_at':start.isoformat(),'finished_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'exit_code':r.returncode,'log':log.name,'sha256':hashlib.sha256(log.read_bytes()).hexdigest()})
 (out/'gates.json').write_text(json.dumps(rows,indent=2)+'\n')
 print(name,r.returncode,flush=True)
 if r.returncode:sys.exit(r.returncode)
 if name=='ledger-contracts':
  source=(w/'.github/workflows/request-ledger-integration.yml').read_text()
  required=ast.literal_eval(re.search(r'required = (\{.*?\n          \})',source,re.S).group(1))
  events=[json.loads(line) for line in log.read_text().splitlines() if line.startswith('{')]
  passed={e.get('Test') for e in events if e.get('Action')=='pass'}
  skipped=[e.get('Test') for e in events if e.get('Action')=='skip']
  missing=sorted(required-passed)
  (out/'ledger-required.json').write_text(json.dumps({'required':sorted(required),'required_count':len(required),'missing':missing,'skipped':skipped,'passed_top_level':sum(1 for e in events if e.get('Action')=='pass' and e.get('Test') and '/' not in e['Test'])},indent=2)+'\n')
  if missing:sys.exit('Required contracts were not executed')
