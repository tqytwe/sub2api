import hashlib
import json
from pathlib import Path
import tempfile
import unittest

try:
    from .check_upstream_dispositions import check
except ImportError:
    from check_upstream_dispositions import check


class DispositionPolicyTest(unittest.TestCase):
    def setUp(self):
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        self.root = Path(temp.name)
        self.release = self.root / 'docs/upstream-migrations/v1.2.3'
        self.release.mkdir(parents=True)
        (self.release / 'README.md').write_text('Release status\n')
        self.audit = self.release / 'DIFFERENCE_AUDIT.md'
        self.fixture_source = self.root / 'backend/internal/service/fixture.go'
        self.fixture_test = self.root / 'backend/internal/service/fixture_test.go'
        self.fixture_source.parent.mkdir(parents=True)
        self.fixture_source.write_text('package service\n')
        self.fixture_test.write_text('package service\n')
        runtime = self.root / 'backend/internal/service/update_service.go'
        runtime.parent.mkdir(parents=True, exist_ok=True)
        self.runtime = runtime
        self.start, self.earlier, self.target = ('a' * 40, 'b' * 40, 'c' * 40)
        records = {}
        for kind, version, start, commits in (
                ('release', 'v1.2.2', self.earlier, [self.target]),
                ('cumulative', 'v1.2.0', self.start, [self.earlier, self.target])):
            path = self.release / (version + '-first-parent.tsv')
            path.write_text('# {}..{}\ncommit\tsubject\n'.format(start, self.target)
                            + ''.join(sha + '\tFixture\n' for sha in commits))
            records[kind] = {'file': path.name, 'sha256': hashlib.sha256(path.read_bytes()).hexdigest()}
        self.lock = {
            'release_tag': 'v1.2.3', 'release_commit': self.target, 'fully_migrated': False,
            'status_snapshot': str((self.release / 'README.md').relative_to(self.root)),
            'current_target': {'release_tag': 'v1.2.3', 'release_commit': self.target,
                               'state': 'deployed_partial', 'fully_migrated': False, 'inventories': records},
        }
        self.findings = {'release_commit': self.target, 'entries': {
            sha: {'state': 'unreviewed', 'summary': 'Fixture'} for sha in (self.earlier, self.target)
        }}

    def write(self):
        (self.release.parent / 'source-lock.json').write_text(json.dumps(self.lock))
        (self.release / 'dispositions.json').write_text(json.dumps(self.findings))
        self.audit.write_text(''.join(
            f'`SYNC-{sha[:8].upper()}` `{sha}`\n' for sha in self.findings['entries']))
        self.runtime.write_text('upstreamReviewVersion = "1.2.3"\nupstreamFullyMigrated = {}\n'.format(
            str(self.lock['fully_migrated']).lower()))

    def audit_ref(self, sha):
        return f'{self.audit.relative_to(self.root)}#SYNC-{sha[:8].upper()}'

    def mark_equivalent_with_evidence(self):
        for sha, entry in self.findings['entries'].items():
            entry.update(state='equivalent', evidence=[self.audit_ref(sha),
                                                       str(self.fixture_test.relative_to(self.root))])

    def test_open_dispositions_match_cumulative_inventory(self):
        self.write()
        self.assertEqual(check(self.root), (1, 2, 2))
        with self.assertRaisesRegex(ValueError, 'not fully migrated \(2 open dispositions\)'):
            check(self.root, require_complete=True)
        del self.findings['entries'][self.earlier]
        self.write()
        with self.assertRaisesRegex(ValueError, 'cover every cumulative commit'):
            check(self.root)

    def test_inventory_cannot_change_without_updating_lock(self):
        self.write()
        inventory = self.release / 'v1.2.0-first-parent.tsv'
        inventory.write_text(inventory.read_text().replace(self.earlier + '\tFixture\n', ''))
        with self.assertRaisesRegex(ValueError, 'inventory differs from source lock'):
            check(self.root)

    def test_merge_gate_does_not_require_future_production_evidence(self):
        self.write()
        with self.assertRaisesRegex(ValueError, '2 open dispositions'):
            check(self.root, require_merge_ready=True)
        for entry in self.findings['entries'].values():
            entry.update(state='equivalent', evidence=['docs/upstream-migrations/v1.2.3/DIFFERENCE_AUDIT.md'])
        self.write()
        with self.assertRaisesRegex(ValueError, 'commit-specific audit anchor'):
            check(self.root, require_merge_ready=True)
        for sha, entry in self.findings['entries'].items():
            entry['evidence'] = [self.audit_ref(sha)]
        self.write()
        self.assertEqual(check(self.root, require_merge_ready=True), (1, 2, 0))
        with self.assertRaisesRegex(ValueError, 'not fully migrated'):
            check(self.root, require_complete=True)
        self.runtime.write_text('upstreamReviewVersion = "1.2.3"\nupstreamFullyMigrated = true\n')
        with self.assertRaisesRegex(ValueError, 'admin runtime differs'):
            check(self.root, require_merge_ready=True)

    def test_merge_gate_still_checks_evidence(self):
        for entry in self.findings['entries'].values():
            entry.update(state='migrated', evidence=['missing.md'])
        self.write()
        with self.assertRaisesRegex(ValueError, 'evidence file missing'):
            check(self.root, require_merge_ready=True)

    def test_migrated_requires_runtime_and_test_evidence(self):
        sha = self.target
        self.mark_equivalent_with_evidence()
        entry = self.findings['entries'][sha]
        entry.update(state='migrated', evidence=[self.audit_ref(sha),
                                                 str(self.fixture_test.relative_to(self.root))])
        self.write()
        with self.assertRaisesRegex(ValueError, 'runtime implementation evidence'):
            check(self.root, require_merge_ready=True)

        entry['evidence'].append(str(self.fixture_source.relative_to(self.root)))
        self.write()
        self.assertEqual(check(self.root, require_merge_ready=True), (1, 2, 0))

    def test_audit_anchor_must_identify_the_inventory_commit(self):
        sha = self.target
        self.mark_equivalent_with_evidence()
        self.findings['entries'][sha].update(
            state='equivalent',
            evidence=['docs/upstream-migrations/v1.2.3/DIFFERENCE_AUDIT.md#SYNC-BADBAD00',
                      str(self.fixture_test.relative_to(self.root))])
        self.write()
        with self.assertRaisesRegex(ValueError, 'commit-specific audit anchor'):
            check(self.root, require_merge_ready=True)

    def test_completion_requires_all_dispositions_and_passing_delivery(self):
        self.lock['fully_migrated'] = True
        self.lock['current_target'].update(state='complete', fully_migrated=True)
        self.write()
        with self.assertRaisesRegex(ValueError, 'open dispositions'):
            check(self.root)
        self.mark_equivalent_with_evidence()
        self.findings['delivery'] = {
            'merge_commit': self.earlier, 'zeabur_commit': self.target, 'pr': 1, 'deployment_id': 2,
            **{field: {'status': 'passed', 'evidence': 'Fixture'} for field in (
                'ci', 'zeabur', 'health', 'critical_api', 'data_reconciliation', 'guest', 'user', 'admin')},
        }
        self.write()
        with self.assertRaisesRegex(ValueError, 'exact deployment SHA'):
            check(self.root)
        self.findings['delivery']['zeabur_commit'] = self.earlier
        self.findings['delivery']['admin']['status'] = 'pending'
        self.write()
        with self.assertRaisesRegex(ValueError, 'missing: admin'):
            check(self.root)
        with self.assertRaisesRegex(ValueError, 'missing: admin'):
            check(self.root, require_merge_ready=True)
        self.findings['delivery']['admin']['status'] = 'passed'
        self.write()
        self.assertEqual(check(self.root), (1, 2, 0))
        self.assertEqual(check(self.root, require_complete=True), (1, 2, 0))
