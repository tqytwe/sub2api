#!/usr/bin/env python3
"""Split GoReleaser builds across runners without requiring GoReleaser Pro."""
import argparse
import hashlib
import itertools
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tarfile
from datetime import datetime, timezone

import yaml

FULL_CONFIG = Path('.goreleaser.yaml')
SIMPLE_CONFIG = Path('.goreleaser.simple.yaml')
VERSION_FILE = Path('backend/cmd/server/VERSION')
SOURCE_LOCK_FILE = Path('docs/upstream-migrations/source-lock.json')
VERSION_RE = re.compile(r'\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?')


def config(simple=False):
    return yaml.safe_load((SIMPLE_CONFIG if simple else FULL_CONFIG).read_text())


def targets(simple=False):
    build = config()['builds'][0]
    result = []
    for goos, goarch in itertools.product(build['goos'], build['goarch']):
        item = {'goos': goos, 'goarch': goarch}
        if any(all(item.get(k) == v for k, v in rule.items()) for rule in build.get('ignore', [])):
            continue
        if not simple or item == {'goos': 'linux', 'goarch': 'amd64'}:
            result.append(item)
    if not result:
        raise ValueError('empty release target matrix')
    return result


def archive_name(version, target):
    if not VERSION_RE.fullmatch(version) or target not in targets():
        raise ValueError('invalid release version or target')
    suffix = 'zip' if target['goos'] == 'windows' else 'tar.gz'
    return f"sub2api_{version}_{target['goos']}_{target['goarch']}.{suffix}"


def sha256(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def source_policy(required=False):
    if not SOURCE_LOCK_FILE.is_file():
        if required:
            raise ValueError('reviewed source lock is required for publication')
        return None
    lock = json.loads(SOURCE_LOCK_FILE.read_text())
    if (lock.get('schema_version') != 1 or lock.get('source_repository') != 'ranxi2001/sub2api'
            or lock.get('source_remote') != 'https://github.com/ranxi2001/sub2api.git'
            or lock.get('production_repository') != 'tqytwe/sub2api'
            or lock.get('production_branch') != 'play/main'
            or not re.fullmatch(r'v[0-9]+\.[0-9]+\.[0-9]+', lock.get('release_tag', ''))
            or lock.get('release_tag') != 'v' + lock.get('version_file', '')):
        raise ValueError('invalid source lock repository, branch or stable version')
    for field in ('tag_object', 'release_commit', 'analysis_base'):
        if not re.fullmatch(r'[0-9a-f]{40}', lock.get(field, '')):
            raise ValueError('source lock requires full SHA values')
    runtime = Path('backend/internal/service/update_service.go').read_text()
    match = re.search(r'upstreamReviewVersion\s*=\s*"([^"]+)"', runtime)
    if not match or match.group(1) != lock['version_file']:
        raise ValueError('runtime upstream baseline must match the reviewed source lock')
    return lock


def source_lock_digest():
    return sha256(SOURCE_LOCK_FILE) if SOURCE_LOCK_FILE.is_file() else None


def plan(args):
    sha = subprocess.check_output(['git', 'rev-parse', 'HEAD'], text=True).strip()
    if args.dry_run:
        version = VERSION_FILE.read_text().strip()
        tag = 'v' + version
    else:
        tag = args.ref
        version = tag.removeprefix('v')
        if not tag.startswith('v') or not VERSION_RE.fullmatch(version):
            raise ValueError('publishing requires a v-prefixed release version tag')
        tagged_sha = subprocess.check_output(['git', 'rev-parse', '--verify', f'refs/tags/{tag}^{{commit}}'], text=True).strip()
        if sha != tagged_sha:
            raise ValueError('checkout does not match the selected release tag')
    if not VERSION_RE.fullmatch(version):
        raise ValueError('invalid VERSION')
    if not args.dry_run:
        if os.environ.get('GITHUB_REPOSITORY') != 'tqytwe/sub2api':
            raise ValueError('publication requires the tqytwe/sub2api repository')
        reviewed_sha = subprocess.check_output(['git', 'rev-parse', '--verify', 'refs/remotes/origin/play/main^{commit}'], text=True).strip()
        if sha != reviewed_sha:
            raise ValueError('publication requires a tag on the current reviewed origin/play/main head')
        if VERSION_FILE.read_text().strip() != version:
            raise ValueError('VERSION must match the tag in the reviewed source; change it through a play/main PR')
    source_policy(required=not args.dry_run)
    if not args.dry_run:
        # Read the remote directly: --no-tags branch fetches intentionally leave
        # local tags unchanged, so they cannot detect a tag moved during a build.
        tag_ref = f'refs/tags/{tag}'
        lines = subprocess.check_output(['git', 'ls-remote', '--tags', 'origin', tag_ref, tag_ref + '^{}'], text=True)
        refs = dict(line.split()[::-1] for line in lines.splitlines() if line.strip())
        if refs.get(tag_ref + '^{}', refs.get(tag_ref)) != sha:
            raise ValueError('remote fork tag is missing or no longer points to the reviewed source')
    result = {'sha': sha, 'tag': tag, 'version': version,
              'owner_lower': os.environ.get('GITHUB_REPOSITORY_OWNER', '').lower(),
              'simple': str(args.simple).lower(), 'dry_run': str(args.dry_run).lower(),
              'date': datetime.now(timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ'),
              'matrix': json.dumps({'include': targets(args.simple)}, separators=(',', ':'))}
    with Path(os.environ['GITHUB_OUTPUT']).open('a') as output:
        for key, value in result.items():
            output.write(f'{key}={value}\n')


def generate_config(args):
    data = config(args.simple if args.mode == 'publish' else False)
    data['snapshot'] = {'version_template': '{{ .Env.RELEASE_VERSION }}'}
    data['dockers'] = []
    data['docker_manifests'] = []
    if args.mode == 'build':
        target = {'goos': args.goos, 'goarch': args.goarch}
        if target not in targets():
            raise ValueError('unsupported build target')
        for build in data['builds']:
            build['goos'], build['goarch'], build['ignore'] = [args.goos], [args.goarch], []
            build['ldflags'] = [re.sub(r'{{\s*\.Date\s*}}', '{{ .Env.RELEASE_DATE }}', flag)
                                for flag in build.get('ldflags', [])]
    else:
        # Artifacts are supplied through the OSS extra_files mechanism. No build
        # is repeated on the publishing runner, and release templates stay intact.
        data['before'] = {'hooks': []}
        data['builds'] = [{'id': 'sub2api', 'skip': True}]
        data['archives'] = []
        extra = [{'glob': 'release-input/sub2api_*.tar.gz'}, {'glob': 'release-input/sub2api_*.zip'},
                 {'glob': '.release-context/release-provenance.json'}]
        if args.simple:
            data['checksum'] = {'disable': True}
        else:
            data['release']['extra_files'] = extra
            data['checksum'] = {'name_template': 'checksums.txt', 'algorithm': 'sha256', 'extra_files': extra}
    Path(args.output).write_text(yaml.safe_dump(data, sort_keys=False, allow_unicode=True))


def collect(args):
    target = {'goos': args.goos, 'goarch': args.goarch}
    name = archive_name(args.version, target)
    source = Path('dist') / name
    checksums = {line.split()[1].lstrip('*'): line.split()[0] for line in Path('dist/checksums.txt').read_text().splitlines()}
    digest = sha256(source)
    if checksums.get(name) != digest:
        raise ValueError('archive does not match the build checksum')
    output = Path(args.output)
    output.mkdir(parents=True, exist_ok=True)
    shutil.copy2(source, output / name)
    manifest = {'sha': args.sha, 'version': args.version, 'target': target, 'archive': name, 'sha256': digest,
                'repository': os.environ.get('GITHUB_REPOSITORY', ''),
                'workflow_sha': os.environ.get('GITHUB_WORKFLOW_SHA', ''),
                'run_id': os.environ.get('GITHUB_RUN_ID', ''),
                'source_lock_sha256': source_lock_digest()}
    (output / f"manifest-{args.goos}-{args.goarch}.json").write_text(json.dumps(manifest) + '\n')


def verify(args):
    directory = Path(args.input)
    expected = set()
    for target in targets(args.simple):
        name = archive_name(args.version, target)
        manifest_name = f"manifest-{target['goos']}-{target['goarch']}.json"
        expected.update((name, manifest_name))
        manifest = json.loads((directory / manifest_name).read_text())
        if manifest != {'sha': args.sha, 'version': args.version, 'target': target,
                        'archive': name, 'sha256': sha256(directory / name),
                        'repository': os.environ.get('GITHUB_REPOSITORY', ''),
                        'workflow_sha': os.environ.get('GITHUB_WORKFLOW_SHA', ''),
                        'run_id': os.environ.get('GITHUB_RUN_ID', ''),
                        'source_lock_sha256': source_lock_digest()}:
            raise ValueError(f'build provenance or checksum mismatch: {name}')
    if {p.name for p in directory.iterdir()} != expected:
        raise ValueError('missing or unexpected release artifacts')


def contexts(args):
    verify(args)
    for target in targets(args.simple):
        if target['goos'] != 'linux':
            continue
        dest = Path(args.output) / target['goarch']
        dest.mkdir(parents=True, exist_ok=True)
        with tarfile.open(Path(args.input) / archive_name(args.version, target), 'r:gz') as archive:
            members = [member for member in archive.getmembers() if member.name in ('sub2api', './sub2api')]
            if len(members) != 1 or not members[0].isfile():
                raise ValueError('archive must contain one regular sub2api binary')
            with archive.extractfile(members[0]) as source, (dest / 'sub2api').open('wb') as output:
                shutil.copyfileobj(source, output)
        (dest / 'sub2api').chmod(0o755)
        shutil.copy2('Dockerfile.goreleaser', dest / 'Dockerfile')
        (dest / 'deploy').mkdir(exist_ok=True)
        shutil.copy2('deploy/docker-entrypoint.sh', dest / 'deploy/docker-entrypoint.sh')
        shutil.copytree('backend/resources', dest / 'backend/resources', dirs_exist_ok=True)
    provenance = {
        'schema_version': 1,
        'repository': os.environ.get('GITHUB_REPOSITORY', ''),
        'commit': args.sha,
        'version': args.version,
        'workflow_sha': os.environ.get('GITHUB_WORKFLOW_SHA', ''),
        'run_id': os.environ.get('GITHUB_RUN_ID', ''),
        'source_lock': source_policy(),
        'source_lock_sha256': source_lock_digest(),
        'upstream_policy': 'ranxi2001/sub2api stable releases; source pin is not proof of integration',
        'artifacts': [json.loads(p.read_text()) for p in sorted(Path(args.input).glob('manifest-*.json'))],
    }
    (Path(args.output) / 'release-provenance.json').write_text(json.dumps(provenance, indent=2) + '\n')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest='command', required=True)
    p = commands.add_parser('plan')
    p.add_argument('--ref', required=True)
    p.add_argument('--simple', action='store_true')
    p.add_argument('--dry-run', action='store_true')
    p.set_defaults(run=plan)
    p = commands.add_parser('config')
    p.add_argument('mode', choices=['build', 'publish'])
    p.add_argument('--goos')
    p.add_argument('--goarch')
    p.add_argument('--simple', action='store_true')
    p.add_argument('--output', required=True)
    p.set_defaults(run=generate_config)
    p = commands.add_parser('collect')
    for arg in ('version', 'sha', 'goos', 'goarch', 'output'):
        p.add_argument('--' + arg, required=True)
    p.set_defaults(run=collect)
    for command, handler in [('verify', verify), ('contexts', contexts)]:
        p = commands.add_parser(command)
        for arg in ('version', 'sha', 'input'):
            p.add_argument('--' + arg, required=True)
        p.add_argument('--simple', action='store_true')
        if command == 'contexts':
            p.add_argument('--output', required=True)
        p.set_defaults(run=handler)
    args = parser.parse_args()
    args.run(args)


if __name__ == '__main__':
    main()
