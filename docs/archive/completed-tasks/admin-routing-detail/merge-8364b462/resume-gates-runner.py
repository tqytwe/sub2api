#!/usr/bin/env python3
from pathlib import Path
import json,subprocess,hashlib,datetime,shutil
w=Path('/workspace/sub2api-routing363-pg-20261011T124516847659Z');out=Path('/workspace/routing363-836-full')
old=json.loads((out/'gates.json').read_text());assert len(old)==4 and old[-1]['name']=='embed-build' and old[-1]['exit_code']==1 and all(x['exit_code']==0 for x in old[:3])
assert 'no space left on device' in (out/'embed-build.log').read_text()
with (out/'prior-gates-enospc.json').open('x') as f:json.dump(old,f,indent=2);f.write('\n')
(out/'embed-build.log').rename(out/'embed-build-enospc.log')
tmp=Path('/tmp/routing363-embed-tmp');tmp.mkdir(exist_ok=False)
rows=old[:3]
for name,cmd in [('embed-build',['go','-C','backend','build','-tags=embed','-buildvcs=false','-trimpath','-o','/tmp/routing363-836-embed','./cmd/server']),('embed-test',['go','-C','backend','test','-tags=embed','./internal/web','./cmd/server']),('fork',['./scripts/check-fork-integrity.sh'])]:
 args=['env','GOTMPDIR='+str(tmp),*cmd];start=datetime.datetime.now(datetime.timezone.utc);log=out/(name+'.log')
 with log.open('x') as stream:r=subprocess.run(['/tmp/p0-run',*args],cwd=w,stdout=stream,stderr=subprocess.STDOUT)
 rows.append({'name':name,'command':args,'started_at':start.isoformat(),'finished_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'exit_code':r.returncode,'log':log.name,'sha256':hashlib.sha256(log.read_bytes()).hexdigest()})
 (out/'gates.json').write_text(json.dumps(rows,indent=2)+'\n');print(name,r.returncode,flush=True)
 if r.returncode:raise SystemExit(r.returncode)
