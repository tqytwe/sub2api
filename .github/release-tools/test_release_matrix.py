import argparse
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import tarfile
import tempfile
import unittest
from unittest.mock import patch

import yaml

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('release_matrix', Path(__file__).with_name('release_matrix.py'))
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)


class ReleaseMatrixTest(unittest.TestCase):
    def setUp(self):
        self.previous = Path.cwd()
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        os.chdir(self.temp.name)
        self.addCleanup(os.chdir, self.previous)
        for name in ('.goreleaser.yaml', '.goreleaser.simple.yaml'):
            shutil.copyfile(ROOT / name, name)
        Path('backend/cmd/server').mkdir(parents=True)
        release.VERSION_FILE.write_text('9.8.7\n')
        self.source_lock = Path('docs/upstream-migrations/source-lock.json')
        self.source_lock.parent.mkdir(parents=True)
        self.pin = {
            'schema_version': 1, 'source_repository': 'ranxi2001/sub2api',
            'source_remote': 'https://github.com/ranxi2001/sub2api.git',
            'release_tag': 'v2.10.3', 'version_file': '2.10.3',
            'tag_object': 'b' * 40, 'release_commit': 'c' * 40,
            'production_repository': 'tqytwe/sub2api', 'production_branch': 'play/main',
            'analysis_base': 'd' * 40, 'fully_migrated': False,
        }
        self.source_lock.write_text(json.dumps(self.pin))
        runtime = Path('backend/internal/service/update_service.go')
        runtime.parent.mkdir(parents=True)
        shutil.copyfile(ROOT / runtime, runtime)


    def fixture_artifacts(self, simple=False):
        directory = Path('release-input')
        directory.mkdir()
        for target in release.targets(simple):
            name = release.archive_name('9.8.7', target)
            archive = directory / name
            if target['goos'] == 'linux':
                with tarfile.open(archive, 'w:gz') as out:
                    info = tarfile.TarInfo('sub2api')
                    info.size = 7
                    info.mode = 0o755
                    out.addfile(info, io.BytesIO(b'fixture'))
            else:
                archive.write_bytes(b'fixture archive')
            metadata = {'version': '9.8.7', 'sha': 'a' * 40, 'target': target,
                        'archive': name, 'sha256': release.sha256(archive),
                        'repository': os.environ.get('GITHUB_REPOSITORY', ''),
                        'workflow_sha': os.environ.get('GITHUB_WORKFLOW_SHA', ''),
                        'run_id': os.environ.get('GITHUB_RUN_ID', ''),
                        'source_lock_sha256': release.sha256(self.source_lock)}
            (directory / f"manifest-{target['goos']}-{target['goarch']}.json").write_text(json.dumps(metadata))
        return argparse.Namespace(input='release-input', version='9.8.7', sha='a' * 40, simple=simple, output='contexts')

    def test_full_and_simple_matrix_match_existing_targets(self):
        full = release.targets()
        self.assertEqual(len(full), 5)
        self.assertNotIn({'goos': 'windows', 'goarch': 'arm64'}, full)
        self.assertEqual(release.targets(True), [{'goos': 'linux', 'goarch': 'amd64'}])

    def test_committed_source_lock_matches_runtime_baseline(self):
        os.chdir(ROOT)
        lock = release.source_policy(required=True)
        self.assertEqual(lock['source_repository'], 'ranxi2001/sub2api')
        self.assertEqual(lock['production_repository'], 'tqytwe/sub2api')

    def test_leaf_keeps_packaging_and_selects_only_one_target(self):
        original = release.config()
        release.generate_config(argparse.Namespace(mode='build', simple=False, goos='darwin', goarch='arm64', output='leaf.yaml'))
        leaf = yaml.safe_load(Path('leaf.yaml').read_text())
        self.assertEqual(leaf['builds'][0]['goos'], ['darwin'])
        self.assertEqual(leaf['builds'][0]['goarch'], ['arm64'])
        self.assertEqual(leaf['builds'][0]['ignore'], [])
        self.assertEqual(leaf['archives'], original['archives'])
        self.assertEqual(leaf['release'], original['release'])
        self.assertFalse(leaf['dockers'])
        self.assertIn('{{ .Env.RELEASE_DATE }}', '\n'.join(leaf['builds'][0]['ldflags']))

    def test_publication_config_has_no_compilation_or_docker_work(self):
        for simple in (False, True):
            with self.subTest(simple=simple):
                original = release.config(simple)
                release.generate_config(argparse.Namespace(mode='publish', simple=simple, output='publisher.yaml'))
                data = yaml.safe_load(Path('publisher.yaml').read_text())
                self.assertTrue(data['builds'][0]['skip'])
                self.assertFalse(data['archives'])
                self.assertFalse(data['dockers'])
                self.assertEqual(data['release']['header'], original['release']['header'])
                self.assertEqual(data['release']['footer'], original['release']['footer'])
                if simple:
                    self.assertTrue(data['checksum']['disable'])
                    self.assertTrue(data['release']['skip_upload'])
                else:
                    self.assertEqual(data['checksum']['extra_files'], data['release']['extra_files'])

    def test_collect_and_verify_hash_and_source_binding(self):
        args = self.fixture_artifacts()
        release.verify(args)
        file = next(Path(args.input).glob('*.tar.gz'))
        file.write_bytes(b'corrupted')
        with self.assertRaisesRegex(ValueError, 'checksum mismatch'):
            release.verify(args)

    def test_missing_extra_and_wrong_commit_artifacts_are_rejected(self):
        args = self.fixture_artifacts(True)
        args.sha = 'b' * 40
        with self.assertRaises(ValueError):
            release.verify(args)
        args.sha = 'a' * 40
        Path('release-input/unexpected').write_text('not an asset')
        with self.assertRaises(ValueError):
            release.verify(args)
        Path('release-input/unexpected').unlink()
        next(Path('release-input').glob('*.tar.gz')).unlink()
        with self.assertRaises(FileNotFoundError):
            release.verify(args)

    def test_linux_context_preserves_binary_executable_mode(self):
        args = self.fixture_artifacts()
        Path('Dockerfile.goreleaser').write_text('FROM scratch\nCOPY sub2api /sub2api\n')
        Path('deploy').mkdir()
        Path('deploy/docker-entrypoint.sh').write_text('#!/bin/sh\nexec /app/sub2api\n')
        Path('backend/resources').mkdir()
        Path('backend/resources/data').write_text('fixture')
        release.contexts(args)
        provenance = json.loads(Path('contexts/release-provenance.json').read_text())
        self.assertEqual(provenance['commit'], args.sha)
        self.assertEqual(provenance['source_lock'], self.pin)
        self.assertEqual(provenance['source_lock_sha256'], release.sha256(self.source_lock))
        self.assertEqual(len(provenance['artifacts']), 5)
        for arch in ('amd64', 'arm64'):
            binary = Path('contexts') / arch / 'sub2api'
            self.assertEqual(binary.read_bytes(), b'fixture')
            self.assertEqual(binary.stat().st_mode & 0o777, 0o755)

    def test_plan_requires_a_tag_for_publication(self):
        args = argparse.Namespace(ref='main', dry_run=False, simple=False)
        with patch.object(subprocess, 'check_output', return_value='a' * 40 + '\n'):
            with self.assertRaisesRegex(ValueError, 'version tag'):
                release.plan(args)
        args.ref = 'v9.8.7'
        with patch.object(subprocess, 'check_output', side_effect=['a' * 40 + '\n', 'b' * 40 + '\n']):
            with self.assertRaisesRegex(ValueError, 'does not match'):
                release.plan(args)

    def test_dry_run_plan_resolves_matrix_without_a_new_tag(self):
        with patch.dict(os.environ, {'GITHUB_OUTPUT': 'outputs', 'GITHUB_REPOSITORY_OWNER': 'ExampleOwner'}), patch.object(subprocess, 'check_output', return_value='a' * 40 + '\n'):
            release.plan(argparse.Namespace(ref='feature/matrix', dry_run=True, simple=False))
        output = dict(line.split('=', 1) for line in Path('outputs').read_text().splitlines())
        self.assertEqual(output['dry_run'], 'true')
        self.assertEqual(output['owner_lower'], 'exampleowner')
        self.assertEqual(len(json.loads(output['matrix'])['include']), 5)

    def test_publication_requires_reviewed_fork_head_and_version(self):
        args = argparse.Namespace(ref='v9.8.7', dry_run=False, simple=False)
        with patch.dict(os.environ, {'GITHUB_REPOSITORY': 'tqytwe/sub2api', 'GITHUB_OUTPUT': 'outputs'}):
            with patch.object(subprocess, 'check_output', side_effect=['a' * 40, 'a' * 40, 'b' * 40]):
                with self.assertRaisesRegex(ValueError, 'play/main'):
                    release.plan(args)
            release.VERSION_FILE.write_text('9.8.6\n')
            with patch.object(subprocess, 'check_output', return_value='a' * 40):
                with self.assertRaisesRegex(ValueError, 'VERSION'):
                    release.plan(args)
            self.assertEqual(release.VERSION_FILE.read_text(), '9.8.6\n')

    def test_publication_accepts_reviewed_fork_and_never_mutates_version(self):
        before = release.VERSION_FILE.stat().st_mtime_ns
        replies = ['a' * 40] * 3 + ['b' * 40 + '\trefs/tags/v9.8.7\n' + 'a' * 40 + '\trefs/tags/v9.8.7^{}\n']
        with patch.dict(os.environ, {'GITHUB_REPOSITORY': 'tqytwe/sub2api', 'GITHUB_OUTPUT': 'outputs', 'GITHUB_REPOSITORY_OWNER': 'tqytwe'}), patch.object(subprocess, 'check_output', side_effect=replies):
            release.plan(argparse.Namespace(ref='v9.8.7', dry_run=False, simple=False))
        self.assertEqual(release.VERSION_FILE.stat().st_mtime_ns, before)
        self.assertIn('owner_lower=tqytwe', Path('outputs').read_text())

    def test_publication_rejects_missing_or_drifted_remote_fork_tag(self):
        for remote in ('', 'c' * 40 + '\trefs/tags/v9.8.7\n',
                       'b' * 40 + '\trefs/tags/v9.8.7\n' + 'c' * 40 + '\trefs/tags/v9.8.7^{}\n'):
            with self.subTest(remote=remote), patch.dict(os.environ, {'GITHUB_REPOSITORY': 'tqytwe/sub2api', 'GITHUB_OUTPUT': 'outputs'}), patch.object(subprocess, 'check_output', side_effect=['a' * 40] * 3 + [remote]):
                with self.assertRaisesRegex(ValueError, 'remote fork tag'):
                    release.plan(argparse.Namespace(ref='v9.8.7', dry_run=False, simple=False))

    def test_manifest_cannot_cross_repository_or_workflow_identity(self):
        args = self.fixture_artifacts()
        for key in ('GITHUB_REPOSITORY', 'GITHUB_WORKFLOW_SHA', 'GITHUB_RUN_ID'):
            with patch.dict(os.environ, {key: 'different-source'}):
                with self.assertRaisesRegex(ValueError, 'provenance'):
                    release.verify(args)

    def test_artifacts_reject_source_lock_substitution(self):
        args = self.fixture_artifacts()
        self.source_lock.write_text(json.dumps(dict(self.pin, fully_migrated=True)))
        with self.assertRaisesRegex(ValueError, 'provenance'):
            release.verify(args)

    def test_publication_requires_source_lock_and_matching_runtime_policy(self):
        args = argparse.Namespace(ref='v9.8.7', dry_run=False, simple=False)
        with patch.dict(os.environ, {'GITHUB_REPOSITORY': 'tqytwe/sub2api', 'GITHUB_OUTPUT': 'outputs'}), patch.object(subprocess, 'check_output', return_value='a' * 40):
            for changed in [dict(self.pin, source_repository='Wei-Shaw/sub2api'),
                            dict(self.pin, production_branch='main'),
                            dict(self.pin, release_commit='c' * 7),
                            dict(self.pin, release_tag='v2.11.0', version_file='2.11.0')]:
                with self.subTest(pin=changed):
                    self.source_lock.write_text(json.dumps(changed))
                    with self.assertRaisesRegex(ValueError, 'source lock|runtime'):
                        release.plan(args)
            self.source_lock.unlink()
            with self.assertRaisesRegex(ValueError, 'source lock'):
                release.plan(args)

    def test_publication_rejects_upstream_repository(self):
        with patch.dict(os.environ, {'GITHUB_REPOSITORY': 'ranxi2001/sub2api', 'GITHUB_OUTPUT': 'outputs'}), patch.object(subprocess, 'check_output', return_value='a' * 40):
            with self.assertRaisesRegex(ValueError, 'repository'):
                release.plan(argparse.Namespace(ref='v9.8.7', dry_run=False, simple=False))

    def test_dry_run_never_writes_version(self):
        release.VERSION_FILE.write_text('9.8.7')
        before = release.VERSION_FILE.stat().st_mtime_ns
        with patch.dict(os.environ, {'GITHUB_OUTPUT': 'outputs'}), patch.object(subprocess, 'check_output', return_value='a' * 40):
            release.plan(argparse.Namespace(ref='feature/test', dry_run=True, simple=False))
        self.assertEqual(release.VERSION_FILE.read_text(), '9.8.7')
        self.assertEqual(release.VERSION_FILE.stat().st_mtime_ns, before)

    def test_workflow_cannot_publish_on_tag_import_or_push_version(self):
        workflow = (ROOT / '.github/workflows/release.yml').read_text()
        data = yaml.load(workflow, Loader=yaml.BaseLoader)
        self.assertEqual(set(data['on']), {'workflow_dispatch'})
        self.assertEqual(data['on']['workflow_dispatch']['inputs']['dry_run']['default'], 'true')
        self.assertNotIn('sync-version-file', data['jobs'])
        self.assertNotIn('git push', workflow)
        self.assertIn("github.ref == 'refs/heads/play/main'", data['jobs']['prepare']['if'])
        self.assertIn("github.repository == 'tqytwe/sub2api'", data['jobs']['prepare']['if'])
        steps = {step.get('name', ''): step for step in data['jobs']['release']['steps']}
        for name in ('Login to DockerHub', 'Login to GitHub Container Registry',
                     'Update DockerHub description', 'Send Telegram Notification'):
            self.assertIn("env.DRY_RUN != 'true'", steps[name]['if'])
        publish = steps['Publish existing archives and release notes']
        self.assertIn('--snapshot --skip=publish,announce', publish['with']['args'])
        self.assertIn("env.DRY_RUN != 'true'", publish['env']['GITHUB_TOKEN'])
        for job in data['jobs'].values():
            for step in job['steps']:
                if step.get('uses', '').startswith('actions/checkout@'):
                    self.assertEqual(step['with']['persist-credentials'], 'false')

    def test_docker_commands_do_not_publish_during_dry_run(self):
        fake_bin = Path('bin')
        fake_bin.mkdir()
        docker = fake_bin / 'docker'
        docker.write_text('#!/bin/sh\nprintf "%s\\n" "$*" >> "$DOCKER_LOG"\n')
        docker.chmod(0o755)
        env = {**os.environ, 'PATH': str(fake_bin.resolve()) + os.pathsep + os.environ['PATH'],
               'DOCKER_LOG': str(Path('docker.log').resolve()), 'RUNNER_TEMP': self.temp.name,
               'RELEASE_VERSION': '9.8.7', 'RELEASE_SHA': 'a' * 40, 'GITHUB_REPOSITORY': 'ExampleOwner/sub2api',
               'DRY_RUN': 'true', 'SIMPLE_RELEASE': 'false', 'DOCKERHUB_USERNAME': 'skip'}
        subprocess.run(['bash', str(ROOT / '.github/release-tools/release-images.sh')], env=env, check=True)
        log = Path('docker.log').read_text()
        self.assertEqual(log.count('buildx build'), 2)
        self.assertIn('linux/arm64', log)
        self.assertNotIn('--push', log)
        self.assertNotIn('imagetools', log)
        self.assertNotIn('skip/sub2api', log)
        self.assertIn('ghcr.io/exampleowner/sub2api', log)

        # A forgotten variable must never turn a local check into a publication.
        Path('docker.log').unlink()
        del env['DRY_RUN']
        subprocess.run(['bash', str(ROOT / '.github/release-tools/release-images.sh')], env=env, check=True)
        log = Path('docker.log').read_text()
        self.assertNotIn('--push', log)
        self.assertNotIn('imagetools', log)
        Path('docker.log').unlink()
        env['DRY_RUN'] = 'typo'
        result = subprocess.run(['bash', str(ROOT / '.github/release-tools/release-images.sh')], env=env, capture_output=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(Path('docker.log').exists())


    def test_published_full_and_simple_image_tags(self):
        fake_bin = Path('bin')
        fake_bin.mkdir()
        docker = fake_bin / 'docker'
        docker.write_text('#!/bin/sh\nprintf "%s\\n" "$*" >> "$DOCKER_LOG"\n')
        docker.chmod(0o755)
        for simple in (False, True):
            with self.subTest(simple=simple):
                log_path = Path(f'docker-{simple}.log').resolve()
                env = {**os.environ, 'PATH': str(fake_bin.resolve()) + os.pathsep + os.environ['PATH'],
                       'DOCKER_LOG': str(log_path), 'RUNNER_TEMP': self.temp.name,
                       'RELEASE_VERSION': '9.8.7', 'RELEASE_SHA': 'a' * 40, 'GITHUB_REPOSITORY': 'ExampleOwner/sub2api',
                       'DRY_RUN': 'false', 'SIMPLE_RELEASE': str(simple).lower(), 'DOCKERHUB_USERNAME': 'fixturehub'}
                subprocess.run(['bash', str(ROOT / '.github/release-tools/release-images.sh')], env=env, check=True)
                log = log_path.read_text()
                self.assertIn('--push', log)
                self.assertEqual(log.count('buildx build'), 1 if simple else 2)
                if simple:
                    self.assertNotIn('fixturehub', log)
                    self.assertNotIn('imagetools', log)
                    self.assertIn('ghcr.io/exampleowner/sub2api:latest', log)
                else:
                    self.assertEqual(log.count('imagetools create'), 2)
                    self.assertIn('fixturehub/sub2api:9.8', log)
                    self.assertIn('ghcr.io/exampleowner/sub2api:9', log)



if __name__ == '__main__':
    unittest.main()
