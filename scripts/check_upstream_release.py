#!/usr/bin/env python3
"""Read-only ranxi release/source comparison. Never fetch, merge, tag, or publish."""

import argparse
import collections
import json
import pathlib
import re
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parents[1]
LOCK = ROOT / "docs/upstream-migrations/source-lock.json"


def run(*args):
    return subprocess.check_output(args, cwd=ROOT, text=True).strip()


def validate_release(lock, release, tag, commit, version):
    if lock["source_repository"] != "ranxi2001/sub2api" or lock["source_remote"] != "https://github.com/ranxi2001/sub2api.git":
        raise ValueError("unexpected direct upstream repository")
    if release.get("draft") is not False or release.get("prerelease") is not False:
        raise ValueError("only published stable releases are eligible")
    if release.get("tag_name") != lock["release_tag"]:
        raise ValueError("release tag does not match source lock")
    if tag != lock["tag_object"] or commit != lock["release_commit"]:
        raise ValueError("release tag moved or commit does not match source lock")
    if version.strip() != lock["version_file"] or "v" + version.strip() != lock["release_tag"]:
        raise ValueError("release VERSION does not match source lock; do not follow a later branch commit")


def migration_report(before, after):
    added = sorted(after.keys() - before.keys())
    numbers = collections.defaultdict(list)
    for name in sorted(before):
        numbers[name.split("_", 1)[0]].append(name)
    return {
        "upstream_only": added,
        "fork_only_preserve": sorted(before.keys() - after.keys()),
        "changed_existing": sorted(name for name in before.keys() & after.keys() if before[name] != after[name]),
        "number_collisions": {name: numbers[name.split("_", 1)[0]] for name in added if name.split("_", 1)[0] in numbers},
    }


def migrations(ref):
    result = {}
    for line in run("git", "ls-tree", "-r", ref, "backend/migrations").splitlines():
        metadata, name = line.split("\t", 1)
        if re.search(r"/\d+_.*\.sql$", name):
            result[pathlib.PurePosixPath(name).name] = metadata.split()[2]
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--offline", action="store_true", help="compare pinned local objects only; does not prove current remote release state")
    parser.add_argument("--strict", action="store_true", help="also require a clean isolated worktree before starting a batch")
    args = parser.parse_args()
    lock = json.loads(LOCK.read_text())
    ref = "refs/remotes/upstream-ranxi/releases/" + lock["release_tag"]
    tag = run("git", "rev-parse", ref)
    commit = run("git", "rev-parse", ref + "^{commit}")
    version = run("git", "show", commit + ":backend/cmd/server/VERSION")
    # Offline mode verifies recorded provenance, never claims live release validation.
    release = {"tag_name": lock["release_tag"], "draft": False, "prerelease": False}
    validate_release(lock, release, tag, commit, version)
    if not args.offline:
        release = json.loads(run("gh", "api", "repos/" + lock["source_repository"] + "/releases/tags/" + lock["release_tag"]))
        remote_refs = dict(line.split()[::-1] for line in run("git", "ls-remote", lock["source_remote"], "refs/tags/" + lock["release_tag"], "refs/tags/" + lock["release_tag"] + "^{}").splitlines())
        remote_tag = remote_refs.get("refs/tags/" + lock["release_tag"])
        remote_commit = remote_refs.get("refs/tags/" + lock["release_tag"] + "^{}", remote_tag)
        validate_release(lock, release, remote_tag, remote_commit, version)
    validate_release(lock, release, tag, commit, version)
    base = lock["analysis_base"]
    branch = run("git", "branch", "--show-current")
    git_dir = pathlib.Path(run("git", "rev-parse", "--absolute-git-dir")).resolve()
    common_dir = (ROOT / run("git", "rev-parse", "--git-common-dir")).resolve()
    clean = not run("git", "status", "--porcelain")
    isolated = git_dir != common_dir and branch not in ("", "main", "play/main")
    changes = run("git", "diff", "--no-renames", "--name-status", base, commit).splitlines()
    print(json.dumps({
        "source_repository": lock["source_repository"], "release_tag": lock["release_tag"],
        "release_commit": commit, "analysis_base": base,
        "live_release_verified": not args.offline, "fully_migrated": False,
        "branch": branch, "isolated_worktree": isolated, "working_tree_clean": clean,
        "file_status_counts": dict(collections.Counter(line.split("\t")[0] for line in changes)),
        "migrations": migration_report(migrations(base), migrations(commit)),
        "action": "report only; all collisions and schema changes require review; no production mutations",
    }, indent=2))
    return 2 if args.strict and not (clean and isolated) else 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (KeyError, ValueError, subprocess.CalledProcessError) as error:
        print("upstream check failed: " + str(error), file=sys.stderr)
        sys.exit(1)
