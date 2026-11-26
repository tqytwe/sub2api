#!/usr/bin/env python3
"""Validate coverage and completion evidence for the current upstream target."""

import argparse
import csv
import json
import hashlib
from pathlib import Path
import re
import subprocess


SHA = re.compile(r"[0-9a-f]{40}")
TERMINAL = {"migrated", "equivalent", "approved_exclusion"}
OPEN = {"unreviewed", "partial", "blocked"}


def require(condition, message):
    if not condition:
        raise ValueError(message)


def inventory(path, target, verify_git):
    with path.open(encoding="utf-8") as stream:
        start, end = stream.readline()[2:].strip().split("..")
        require(SHA.fullmatch(start) and end == target, f"{path}: wrong comparison range")
        reader = csv.DictReader(stream, delimiter="\t")
        require(reader.fieldnames == ["commit", "subject"], f"{path}: wrong columns")
        rows = list(reader)
    commits = [row["commit"] for row in rows]
    require(commits and commits[-1] == target and len(set(commits)) == len(commits)
            and all(SHA.fullmatch(commit) for commit in commits), f"{path}: invalid commit list")
    if verify_git:
        subprocess.run(["git", "merge-base", "--is-ancestor", start, end], check=True)
        actual = subprocess.check_output(
            ["git", "rev-list", "--first-parent", "--reverse", f"{start}..{end}"], universal_newlines=True
        ).splitlines()
        require(commits == actual, f"{path}: inventory differs from upstream Git history")
    return set(commits)


def check(root, verify_git=False, require_complete=False, *, require_merge_ready=False):
    lock = json.loads((root / "docs/upstream-migrations/source-lock.json").read_text(encoding="utf-8"))
    target = lock["release_commit"]
    tag = lock["release_tag"]
    current = lock["current_target"]
    require(SHA.fullmatch(target) and re.fullmatch(r"v\d+\.\d+\.\d+", tag), "invalid source target")
    complete = lock["fully_migrated"]
    require(type(complete) is bool and current["fully_migrated"] is complete
            and current["release_commit"] == target and current["release_tag"] == tag,
            "current target differs from source lock")
    runtime = (root / "backend/internal/service/update_service.go").read_text(encoding="utf-8")
    require(re.search(rf'upstreamReviewVersion\s*=\s*"{re.escape(tag[1:])}"', runtime)
            and re.search(rf'upstreamFullyMigrated\s*=\s*{str(complete).lower()}\b', runtime),
            "admin runtime differs from source lock")
    folder = root / "docs/upstream-migrations" / tag
    require(root / lock["status_snapshot"] == folder / "README.md" and (folder / "README.md").is_file(),
            "missing release status snapshot")
    inventories = current["inventories"]
    require(set(inventories) == {"release", "cumulative"}, "release and cumulative inventories are required")
    commits = {}
    for kind, record in inventories.items():
        filename = record["file"]
        require(re.fullmatch(r"v\d+\.\d+\.\d+-first-parent\.tsv", filename), "invalid inventory filename")
        path = folder / filename
        require(hashlib.sha256(path.read_bytes()).hexdigest() == record["sha256"],
                f"{filename}: inventory differs from source lock")
        commits[kind] = inventory(path, target, verify_git)
    release = commits["release"]
    cumulative = commits["cumulative"]
    require(release <= cumulative,
            "release and cumulative inventories must be nested")
    findings = json.loads((folder / "dispositions.json").read_text(encoding="utf-8"))
    require(findings["release_commit"] == target, "dispositions must match the locked source")
    entries = findings["entries"]
    require(set(entries) == cumulative, "dispositions must cover every cumulative commit exactly once")
    audit_path = folder / "DIFFERENCE_AUDIT.md"
    audit_text = audit_path.read_text(encoding="utf-8") if audit_path.is_file() else ""
    for commit, entry in entries.items():
        state = entry["state"]
        require(state in TERMINAL | OPEN, f"{commit}: invalid disposition")
        require(entry.get("summary", "").strip(), f"{commit}: missing summary")
        if state in TERMINAL:
            evidence = entry.get("evidence", [])
            require(isinstance(evidence, list) and evidence, f"{commit}: missing evidence")
            concrete = []
            has_commit_anchor = False
            for name in evidence:
                require(isinstance(name, str), f"{commit}: evidence references must be strings")
                evidence_path, separator, anchor = name.partition("#")
                path = root / evidence_path
                require(root.resolve() in path.resolve().parents and path.is_file(),
                        f"{commit}: evidence file missing: {name}")
                if path == audit_path:
                    expected_anchor = f"SYNC-{commit[:8].upper()}"
                    require(separator and anchor == expected_anchor
                            and any(expected_anchor in line and commit in line for line in audit_text.splitlines()),
                            f"{commit}: missing commit-specific audit anchor {expected_anchor}")
                    has_commit_anchor = True
                else:
                    concrete.append(path)
            require(has_commit_anchor,
                    f"{commit}: terminal disposition requires a commit-specific audit anchor")
            if state == "migrated":
                require(concrete,
                        f"{commit}: migrated disposition requires concrete evidence file beyond the audit")
                has_runtime = any(path.suffix in {".go", ".ts", ".tsx", ".vue", ".sql", ".py"}
                                  and not path.name.endswith(("_test.go", ".spec.ts", ".test.ts"))
                                  for path in concrete)
                has_test = any(path.name.endswith("_test.go") or ".spec." in path.name
                               or ".test." in path.name or path.name.startswith("test_")
                               for path in concrete)
                require(has_runtime,
                        f"{commit}: migrated disposition requires runtime implementation evidence")
                require(has_test,
                        f"{commit}: migrated disposition requires regression test evidence")
        if state == "approved_exclusion":
            require(re.fullmatch(r"https://github\.com/tqytwe/sub2api/(issues|pull)/\d+(?:#[-\w]+)?",
                                 entry.get("approval", "")), f"{commit}: missing approval reference")
    remaining = {commit for commit, entry in entries.items() if entry["state"] in OPEN}
    require((current["state"] == "complete") == complete, "completion status differs from source lock flag")
    if require_merge_ready:
        require(not remaining,
                f"upstream sync is not ready to merge ({len(remaining)} open dispositions)")
    if require_complete:
        require(complete and not remaining,
                f"upstream sync delivery is blocked: release is not fully migrated ({len(remaining)} open dispositions)")
    if complete:
        require(not remaining, f"cannot close release with {len(remaining)} open dispositions")
        delivery = findings["delivery"]
        require(SHA.fullmatch(delivery["merge_commit"]) and delivery["zeabur_commit"] == delivery["merge_commit"],
                "completion requires exact deployment SHA")
        require(type(delivery["pr"]) is int and delivery["pr"] > 0 and str(delivery["deployment_id"]).isdigit(),
                "completion requires PR and deployment IDs")
        for field in ("ci", "zeabur", "health", "critical_api", "data_reconciliation", "guest", "user", "admin"):
            proof = delivery[field]
            statuses = {"passed", "not_applicable"} if field == "data_reconciliation" else {"passed"}
            require(proof["status"] in statuses and proof["evidence"].strip(), f"completion evidence missing: {field}")
    return len(release), len(cumulative), len(remaining)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--verify-git", action="store_true", help="compare inventories with fetched upstream Git history")
    gates = parser.add_mutually_exclusive_group()
    gates.add_argument("--require-merge-ready", action="store_true",
                       help="fail if any cumulative disposition is open; production proof is not yet required")
    gates.add_argument("--require-complete", action="store_true",
                       help="fail unless every cumulative disposition and production delivery gate is complete")
    args = parser.parse_args()
    try:
        release, cumulative, remaining = check(Path.cwd(), args.verify_git, args.require_complete,
                                                require_merge_ready=args.require_merge_ready)
    except (ValueError, KeyError, OSError, subprocess.CalledProcessError) as error:
        parser.exit(1, f"Upstream dispositions: {error}\n")
    print(f"Upstream dispositions: {release} release commits, {cumulative} cumulative commits, {remaining} open")


if __name__ == "__main__":
    main()
