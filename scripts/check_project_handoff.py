#!/usr/bin/env python3
"""Offline record checks only. Pending evidence is reported, never a CI gate."""

import argparse
from datetime import datetime
import json
from pathlib import Path
import re
import sys
from urllib.parse import unquote, urlsplit

ROOT = Path(__file__).resolve().parents[1]
SNAPSHOT = "docs/upstream-migrations/v2.10.3/handoff.json"
STATES = {"设计", "实现中", "可测", "已验证", "已部署", "弃用中", "已移除"}
LAYERS = ("ui", "api", "schema", "tests", "deployment", "production")


def validate(data, root, snapshot=SNAPSHOT):
    errors, pending = [], []
    root = root.resolve()

    def require(condition, label):
        if not condition:
            errors.append(label)
        return bool(condition)

    def obj(value, label):
        return value if require(isinstance(value, dict), label + ": expected object") else {}

    def items(value, label):
        return value if require(isinstance(value, list), label + ": expected list") else []

    def text(value, label):
        return require(isinstance(value, str) and bool(value.strip()), label + ": required text")

    def sha(value, label):
        require(isinstance(value, str) and re.fullmatch(r"[0-9a-f]{40}", value) is not None,
                label + ": expected full SHA")

    def ref(value, label):
        if not text(value, label):
            return
        try:
            url = urlsplit(value)
            if url.scheme or url.netloc:
                require(url.scheme == "https" and bool(url.hostname) and not url.username
                        and not url.password and not any(c.isspace() for c in value),
                        label + ": expected HTTPS URL without credentials")
            else:
                path = (root / unquote(url.path)).resolve()
                require(path.is_relative_to(root) and path.is_file(),
                        label + ": missing/non-repository local file " + value)
        except ValueError:
            errors.append(label + ": invalid link")

    def refs(value, label):
        result = items(value, label)
        for entry in result:
            ref(entry, label)
        return result

    def slot(value, label):
        value = obj(value, label)
        status = value.get("status")
        require(status in ("recorded", "pending", "na"), label + ": invalid status")
        links = refs(value.get("refs"), label + ".refs")
        text(value.get("note"), label + ".note")
        if status == "recorded":
            require(bool(links), label + ".refs: recorded requires evidence")
        if status == "pending":
            pending.append(label + ": " + str(value.get("note", "")))

    def unique(records, field, label):
        seen = set()
        for entry in records:
            item = obj(entry, label)
            key = item.get(field)
            if text(key, label + "." + field):
                require(key not in seen, label + ": duplicate " + key)
                seen.add(key)
        return seen

    data = obj(data, "snapshot")
    require(data.get("schema_version") == 1, "schema_version: expected 1")
    try:
        timestamp = data.get("checked_at")
        datetime.strptime(timestamp, "%Y-%m-%dT%H:%M:%SZ")
    except (TypeError, ValueError):
        errors.append("checked_at: expected valid UTC timestamp YYYY-MM-DDTHH:MM:SSZ")
    text(data.get("scope"), "scope")
    ref(data.get("source_lock"), "source_lock")
    try:
        lock = json.loads((root / data["source_lock"]).read_text())
        require(lock.get("status_snapshot") == snapshot, "source_lock.status_snapshot: wrong pointer")
    except (OSError, ValueError, KeyError, TypeError, AttributeError):
        errors.append("source_lock: unreadable lock")
    production = obj(data.get("production"), "production")
    text(production.get("branch"), "production.branch")
    for field in ("branch_sha", "last_verified_deployed_sha"):
        sha(production.get(field), "production." + field)
    require(production.get("deployment_status") in ("recorded", "pending"), "production.deployment_status: invalid status")
    require(bool(refs(production.get("evidence"), "production.evidence")), "production.evidence: required")
    text(production.get("note"), "production.note")

    features = items(data.get("features"), "features")
    require(bool(features), "features: required records")
    ids = unique(features, "id", "features")
    for value in features:
        feature = obj(value, "feature")
        label = str(feature.get("id", "feature"))
        for field in ("title", "owner", "risks", "next", "rollback"):
            text(feature.get(field), label + "." + field)
        for field in ("backend_state", "product_state"):
            require(feature.get(field) in tuple(STATES), label + "." + field + ": invalid state")
        for field in ("pr", "sha"):
            require(field in feature, label + "." + field + ": required; use null if unknown")
            if feature.get(field) is None:
                pending.append(label + "." + field + ": unknown; see next")
            elif field == "pr":
                ref(feature[field], label + ".pr")
            else:
                sha(feature[field], label + ".sha")
        evidence = obj(feature.get("evidence"), label + ".evidence")
        for layer in LAYERS:
            slot(evidence.get(layer), label + "." + layer)

    deprecations = items(data.get("deprecations"), "deprecations")
    unique(deprecations, "id", "deprecations")
    linked, removed = set(), set()
    for value in deprecations:
        item = obj(value, "deprecation")
        label = str(item.get("id", "deprecation"))
        fid = item.get("feature_id")
        if text(fid, label + ".feature_id"):
            require(fid in ids, label + ".feature_id: unknown feature")
            linked.add(fid)
        for field in ("owner", "reason", "version", "removal_scope"):
            text(item.get(field), label + "." + field)
        ref(item.get("decision"), label + ".decision")
        require(item.get("state") in ("设计", "弃用中", "已移除"), label + ": invalid deprecation state")
        for field in ("inventory", "replacement", "compatibility", "removal_approval", "retained_tests"):
            slot(item.get(field), label + "." + field)
        if item.get("state") == "已移除":
            approval = obj(item.get("removal_approval"), label + ".removal_approval")
            require(approval.get("status") == "recorded", label + ".removal_approval: required before removal")
            if isinstance(fid, str) and approval.get("status") == "recorded" and approval.get("refs"):
                removed.add(fid)
    for value in features:
        feature = obj(value, "feature")
        if any(feature.get(field) in ("弃用中", "已移除") for field in ("backend_state", "product_state")):
            require(feature.get("id") in tuple(linked), str(feature.get("id")) + ": missing deprecation")
        if any(feature.get(field) == "已移除" for field in ("backend_state", "product_state")):
            require(feature.get("id") in tuple(removed), str(feature.get("id")) + ": missing completed deprecation/removal_approval")

    routes = obj(data.get("route_review"), "route_review")
    for field in ("source", "target"):
        ref(routes.get(field), "route_review." + field)
    text(routes.get("method"), "route_review.method")
    source_only = items(routes.get("source_only"), "route_review.source_only")
    unique(source_only, "path", "route_review")
    for value in source_only:
        item = obj(value, "route_review")
        for field in ("classification", "reason", "owner"):
            text(item.get(field), "route_review." + field)
    counts = obj(routes.get("counts"), "route_review.counts")
    if all(type(counts.get(k)) is int and counts[k] >= 0
           for k in ("source", "target", "common", "source_only", "target_only")):
        require(counts["source_only"] == len(source_only), "route_review.count: missing classification")
        require(counts["common"] + counts["source_only"] == counts["source"]
                and counts["common"] + counts["target_only"] == counts["target"], "route_review.count: inconsistent totals")
    else:
        errors.append("route_review.counts: expected nonnegative integers")

    try:
        index = (root / "docs/README.md").read_text()
        for path in ("./PROJECT_HYGIENE.md", "./" + str(Path(snapshot).relative_to("docs"))):
            require("](" + path + ")" in index, "docs index: missing " + path)
    except (OSError, ValueError):
        errors.append("docs index: unreadable index or invalid snapshot path")
    return errors, pending


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--file", default=SNAPSHOT, help="repository-relative snapshot path")
    args = parser.parse_args()
    try:
        data = json.loads((ROOT / args.file).read_text())
        errors, pending = validate(data, ROOT, args.file)
    except (OSError, ValueError) as error:
        print("Handoff record check failed: " + str(error), file=sys.stderr)
        return 1
    for error in errors:
        print("ERROR " + error, file=sys.stderr)
    for item in pending:
        print("PENDING " + item)
    if errors:
        return 1
    print("Snapshot " + data["checked_at"] + " (not live):")
    for item in data["features"]:
        print(f'{item["id"]}: backend={item["backend_state"]}; product={item["product_state"]}; owner={item["owner"]}')
    print(f"Record structure/local paths OK; {len(pending)} pending entries. No runtime, remote-link or production validation.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
