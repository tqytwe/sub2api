#!/usr/bin/env python3
"""Offline, deterministic source coverage check; never a behavioral compatibility claim."""
import argparse
import hashlib
import json
import re
from pathlib import Path
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / 'docs/upstream-migrations/v2.10.5'
BASE = 'fd1b5ee4eeb20961fbb783fa6f136a1704271e90'
TARGET = 'd977e7a5e24e5536db6ce8b8478743e0c4b11f77'
INITIAL_FORK = '8364b462feda15f3bf8f3bb2029314d2193b9dba'
FORK = '16a11abd29b1f37f3be28820b948f2d8400e4e70'
TAG = 'e7f36d0ed124fccbf55382166c116a89ef79dd83'

def git(*args):
    return subprocess.check_output(['git', *args], cwd=ROOT)

def tree(ref):
    return {p.decode(): meta.decode().split()[2] for meta, p in
            (line.split(b'\t', 1) for line in git('ls-tree', '-rz', ref).split(b'\0') if line)}

def layer(p):
    if p.endswith('.sql') and '/migrations/' in p: return 'database'
    if '/ent/' in p: return 'schema-generated'
    if 'test' in p.lower() or '__tests__' in p: return 'tests'
    if '/config/' in p or p.startswith('deploy/'): return 'config-deploy'
    if '/handler/' in p or '/routes/' in p or '/api/' in p or '/types/' in p: return 'api-dto'
    if p.startswith('frontend/'): return 'frontend-entry-ui'
    if p.startswith('backend/'): return 'backend'
    return 'engineering'

def artifacts():
    a, b, f = tree(BASE), tree(TARGET), tree(FORK)
    protection = json.loads((OUT/'protected.json').read_text())
    protected = set(protection['paths'])
    inherited = set(protection.get('inherited_paths', [])) - set(protection.get('inflight_paths', []))
    sql_decisions = {d['source_path']: d for d in json.loads((OUT/'migration-decisions.json').read_text())}
    rows = []
    for p in sorted(set(a)|set(b)|set(f)):
        if a.get(p) == b.get(p) == f.get(p): continue
        delta = a.get(p) != b.get(p)
        if b.get(p) is not None and b.get(p) == f.get(p):
            status, reason = 'absorbed-bytes', 'Exact target blob at fixed Fork baseline; runtime acceptance still required.'
        elif p not in b:
            status, reason = 'not-applicable-preserve', 'Fork-only or upstream-deleted path; no deletion authorized.'
        else:
            status, reason = 'needs-port-review', 'Different/missing target blob; inspect semantics and cross-layer dependencies, never overwrite whole file.'
        if p in sql_decisions:
            status, reason = sql_decisions[p]['status'], sql_decisions[p]['reason']
        if p in protected or '/migrations/280_' in p or '/migrations/281_' in p or '/requestledger/' in p or 'request_ledger' in p:
            status, reason = 'pending-composition', 'Reserved parallel owner; inventory only, no overwrite or duplicate repair.'
        if p in inherited:
            status, reason = 'protected-inherited', 'Reviewed mainline (#363/#365) already merged and inherited byte-for-byte; not an open task. Any upstream overlay requires separate semantic review.'
        rows.append({'path':p,'layer':layer(p),'source_base_blob':a.get(p),'target_blob':b.get(p),'fork_blob':f.get(p),'release_delta':delta,'status':status,'reason':reason})
    migrations = []
    for p in sorted(set(b)|set(f)):
        if not p.startswith('backend/migrations/') or not p.endswith('.sql'): continue
        num = Path(p).name.split('_')[0]
        collisions = [q for q in f if q.startswith('backend/migrations/'+num+'_') and q != p]
        migrations.append({'path':p,'source_blob':b.get(p),'fork_blob':f.get(p),'source_sha256':hashlib.sha256(git('show',TARGET+':'+p)).hexdigest() if p in b else None,'fork_sha256':hashlib.sha256(git('show',FORK+':'+p)).hexdigest() if p in f else None,'same_number_fork_paths':sorted(collisions),'release_delta':a.get(p)!=b.get(p)})
    commits = git('log','--reverse','--format=%H%x09%s',BASE+'..'+TARGET).decode()
    diff = git('diff','--no-renames','--name-status',BASE,TARGET).decode()
    full = git('diff','--no-renames','--name-status',FORK,TARGET).decode()
    def routes(ref):
        source = git('show', ref+':frontend/src/router/index.ts').decode()
        block = source[source.index('const routes:'):source.index('const router = createRouter')]
        found = set(re.findall(r"path:\s*['\"]([^'\"]+)['\"]", block))
        if 'path: adminPromptAuditPath' in block:
            assert "const adminPromptAuditPath = '/admin/pro' + 'mpt-audit'" in source
            found.add('/admin/prompt-audit')
        return found
    upstream_routes, fork_routes = routes(TARGET), routes(FORK)
    route_inventory = {'method':'Fixed route declarations only, includes known adminPromptAuditPath constant; not runtime/API permission validation.', 'source':sorted(upstream_routes), 'fork':sorted(fork_routes), 'source_only':[{'path':p,'status':'needs-port-review','reason':'Inspect backend/schema/permissions/UI entry and tests together; alias is not independent implementation.'} for p in sorted(upstream_routes-fork_routes)], 'fork_only':[{'path':p,'status':'preserve','reason':'Fork customization; no removal authorized.'} for p in sorted(fork_routes-upstream_routes)]}
    data={'schema_version':1,'source_repository':'ranxi2001/sub2api','source_base':BASE,'target_tag':'v2.10.5','target_tag_object':TAG,'target_commit':TARGET,'fork_baseline':FORK,'initial_fork_baseline':INITIAL_FORK,'fully_migrated':False,'meaning':'Exhaustive file/blob accounting; needs-port-review is conservative, not an assertion of missing behavior. No runtime compatibility inferred.','files':rows}
    return {'routes.json':json.dumps(route_inventory,ensure_ascii=False,indent=2)+'\n','coverage.json':json.dumps(data,ensure_ascii=False,indent=2)+'\n','migrations.json':json.dumps(migrations,ensure_ascii=False,indent=2)+'\n','release-commits.tsv':commits,'release-diff.tsv':diff,'fork-target-diff.tsv':full}

def validate_features(rows, features):
    expected = {r['path'] for r in rows if r['release_delta']}
    actual = [p for f in features for p in f['source_paths']]
    errors = []
    if set(actual) != expected or len(actual) != len(set(actual)):
        errors.append('feature coverage missing/extra/duplicate')
    for f in features:
        for key in ['ui', 'api_dto', 'schema', 'config', 'tests']:
            item = f.get('associations', {}).get(key, {})
            if item.get('status') not in ['pending', 'recorded', 'na'] or not item.get('note'):
                errors.append(f['id'] + ': missing association ' + key)
    return errors

def main():
    parser=argparse.ArgumentParser(); parser.add_argument('--write',action='store_true'); args=parser.parse_args()
    assert git('rev-parse',TAG+'^{}').decode().strip()==TARGET
    assert git('show',TARGET+':backend/cmd/server/VERSION').decode().strip()=='2.10.5'
    assert git('show',FORK+':docs/upstream-migrations/source-lock.json') == (ROOT/'docs/upstream-migrations/source-lock.json').read_bytes(), 'formal source lock changed'
    composition = json.loads((OUT/'composition.json').read_text())
    assert composition['composition_base'] == FORK
    assert git('show','-s','--format=%P',composition['merge_commit']).decode().split() == composition['merge_parents']
    assert git('rev-parse','HEAD:frontend') == git('rev-parse',FORK+':frontend'), 'inherited frontend changed'
    for path, blob in composition['incoming_paths_exact_main'].items():
        assert git('hash-object',path).decode().strip() == blob, 'inherited main bytes changed: '+path
    for path, blob in composition['p3_source_blobs_unchanged'].items():
        assert git('hash-object',path).decode().strip() == blob, 'reviewed P3 bytes changed: '+path
    for entry in (OUT/'p3-1-source.sha256').read_text().splitlines():
        expected, path = entry.split('  ', 1)
        assert hashlib.sha256((ROOT/path).read_bytes()).hexdigest() == expected, 'validated P3 source changed: '+path
    failures=[]
    for name,content in artifacts().items():
        path=OUT/name
        if args.write: path.write_text(content)
        elif not path.exists() or path.read_text()!=content: failures.append(name)
    protected=json.loads((OUT/'protected.json').read_text())['paths']
    tracked=git('diff','--name-only',FORK).decode().splitlines() + git('ls-files','--others','--exclude-standard').decode().splitlines()
    forbidden=[p for p in tracked if p in protected or '/requestledger/' in p or 'request_ledger' in p or (p.startswith('backend/migrations/') and p.endswith('.sql'))]
    failures += ['protected mutation: '+p for p in forbidden]
    rows=json.loads((OUT/'coverage.json').read_text())['files']
    failures += validate_features(rows, json.loads((OUT/'features.json').read_text()))
    migrations = json.loads((OUT/'migrations.json').read_text())
    decisions = json.loads((OUT/'migration-decisions.json').read_text())
    expected_sql = {r['path'] for r in migrations if r['source_blob'] and not r['fork_blob']}
    assigned_sql = [d['source_path'] for d in decisions]
    if set(assigned_sql) != expected_sql or len(set(assigned_sql)) != len(assigned_sql):
        failures.append('SQL decision missing/duplicate/extra')
    for d in decisions:
        if not d.get('reason') or d.get('status') not in ['needs-port','pending-composition','absorbed-adapted-schema','not-applicable-current-scope']:
            failures.append('invalid SQL decision: '+d['source_path'])
        for ref in d['fork_evidence']:
            if not (ROOT/ref).is_file(): failures.append('missing SQL evidence: '+ref)
    print(json.dumps({'inventory_files':len(rows),'release_delta_files':sum(r['release_delta'] for r in rows),'statuses':{s:sum(r['status']==s for r in rows) for s in sorted({r['status'] for r in rows})},'failures':failures,'runtime_compatibility':'NOT VERIFIED'},ensure_ascii=False))
    return bool(failures)
if __name__=='__main__': sys.exit(main())
