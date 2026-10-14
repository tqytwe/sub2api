#!/usr/bin/env python3
"""Repeat real cancellation recovery on one disposable full-migration PG.

Only the test entry point is overlaid; production code is never replaced.
Example: python docs/archive/completed-tasks/2026-10-11-p1-subscription-inflight/repeat-recovery.py --output-dir /tmp/p1-recovery --repetitions 40
"""
import argparse
import hashlib
import json
import os
import re
from pathlib import Path
import subprocess
import time

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--output-dir', type=Path, required=True)
parser.add_argument('--repetitions', type=int, default=40)
args = parser.parse_args()
if args.repetitions < 1 or args.repetitions > 100:
    parser.error('repetitions must be between 1 and 100')
root = Path(__file__).resolve().parents[4]
out = args.output_dir.resolve()
out.mkdir(parents=True, exist_ok=True)
source = root / 'backend/internal/billingacceptance/subscription_inflight_revoke_integration_test.go'
overlay_source = out / 'recovery-repeat-overlay.go'
overlay_source.write_text(source.read_text() + '''
func TestSubscriptionRecoveryStress(t *testing.T) {
 db, client := openIsolatedBillingDB(t)
 gin.SetMode(gin.TestMode)
 for i := 0; i < %d; i++ {
  t.Run(fmt.Sprintf("iteration_%%02d", i), func(t *testing.T) { runInflightBoundaries(t, db, client) })
 }
}
''' % args.repetitions)
overlay = out / 'recovery-repeat-overlay.json'
overlay.write_text(json.dumps({'Replace': {str(source): str(overlay_source)}}))
env = os.environ.copy()
if '-overlay' in env.get('GOFLAGS', ''):
    parser.error('unset an existing GOFLAGS overlay before running this probe')
env['GOFLAGS'] = env.get('GOFLAGS', '') + ' -overlay=' + str(overlay)
command = ['./scripts/test-subscription-counters.sh', '^TestSubscriptionRecoveryStress$/iteration_/cancel_after_durable_usage_production_worker_recovers$', '-race']
log = out / 'recovery-repeat.log'
pg_log = out / 'recovery-repeat-postgres.log'
with log.open('w') as stdout, pg_log.open('w') as pg_stdout:
    process = subprocess.Popen(command, cwd=root, env=env, stdout=stdout, stderr=subprocess.STDOUT)
    docker_log = None
    for _ in range(80):
        rows = subprocess.check_output(['docker', 'ps', '--filter', 'label=sub2api.test=subscription-counters', '--format', '{{.ID}} {{.Names}}'], text=True).splitlines()
        container = next((row.split()[0] for row in rows if row.split()[1].endswith('-' + str(process.pid))), None)
        if container:
            docker_log = subprocess.Popen(['docker', 'logs', '-f', container], stdout=pg_stdout, stderr=subprocess.STDOUT)
            break
        if process.poll() is not None:
            break
        time.sleep(0.1)
    code = process.wait()
    if docker_log:
        docker_log.wait(timeout=10)
pg_text = pg_log.read_text()
passed_cases = len(re.findall(r'--- PASS: TestSubscriptionRecoveryStress/iteration_\d+/cancel_after_durable_usage_production_worker_recovers ', log.read_text()))
valid = passed_cases == args.repetitions and code == 0 and docker_log is not None and 'deadlock detected' not in pg_text and 'ERROR:' not in pg_text
manifest = {'command': command, 'exit_code': code, 'repetitions': args.repetitions, 'passed_cases': passed_cases, 'postgres_log_captured': docker_log is not None, 'postgres_deadlock': 'deadlock detected' in pg_text, 'postgres_error': 'ERROR:' in pg_text, 'passed': valid,
            'sha256': {p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in [log, pg_log, overlay_source, overlay]}}
(out / 'recovery-repeat-validation.json').write_text(json.dumps(manifest, indent=2) + '\n')
print(json.dumps(manifest, indent=2))
raise SystemExit(0 if valid else 1)
