"""Record integrity tests; no application or production acceptance claims."""

import copy
import importlib.util
import pathlib
import tempfile
import unittest


spec = importlib.util.spec_from_file_location(
    "handoff", pathlib.Path(__file__).with_name("check_project_handoff.py"))
check = importlib.util.module_from_spec(spec)
spec.loader.exec_module(check)


class HandoffTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = pathlib.Path(self.temp.name)
        (self.root / "docs").mkdir()
        (self.root / "docs/README.md").write_text(
            "[entry](./PROJECT_HYGIENE.md) [snapshot](./status.json)")
        (self.root / "docs/PROJECT_HYGIENE.md").write_text("entry")
        (self.root / "docs/status.json").write_text("{}")
        (self.root / "docs/source-lock.json").write_text(
            '{"status_snapshot":"docs/status.json"}')
        evidence = {name: {"status": "pending", "refs": [], "note": "await evidence"}
                    for name in check.LAYERS}
        self.data = {
            "schema_version": 1, "checked_at": "2026-10-09T22:48:54Z", "scope": "bounded",
            "source_lock": "docs/source-lock.json",
            "production": {"branch": "play/main", "branch_sha": "a" * 40,
                           "last_verified_deployed_sha": "b" * 40, "deployment_status": "pending",
                           "evidence": ["https://github.com/o/r/commit/" + "a" * 40], "note": "deployment only"},
            "features": [{"id": "sample", "title": "Sample", "backend_state": "可测",
                          "product_state": "实现中", "owner": "待分配", "pr": None, "sha": None,
                          "evidence": evidence, "risks": "unknown", "next": "assign", "rollback": "stop"}],
            "deprecations": [],
            "route_review": {"source": "https://github.com/o/r/blob/a/routes.ts",
                             "target": "docs/PROJECT_HYGIENE.md", "method": "manual snapshot",
                             "counts": {"source": 1, "target": 0, "common": 0, "source_only": 1, "target_only": 0},
                             "source_only": [{"path": "/sample", "classification": "后续评估", "reason": "unreviewed", "owner": "待分配"}]}}

    def validate(self):
        return check.validate(self.data, self.root, "docs/status.json")

    def test_pending_is_visible_but_not_a_failure(self):
        errors, pending = self.validate()
        self.assertEqual(errors, [])
        self.assertTrue(any("sample.ui" in item for item in pending))

    def test_missing_layer_and_invalid_state_fail(self):
        del self.data["features"][0]["evidence"]["schema"]
        self.data["features"][0]["product_state"] = "100%"
        errors, _ = self.validate()
        self.assertTrue(any("schema" in item for item in errors))
        self.assertTrue(any("product_state" in item for item in errors))

    def test_invalid_timestamp_sha_duplicate_and_missing_owner(self):
        self.data["checked_at"] = "2026-02-30T00:00:00Z"
        self.data["features"][0]["sha"] = "short"
        self.data["features"].append(copy.deepcopy(self.data["features"][0]))
        del self.data["features"][0]["owner"]
        errors, _ = self.validate()
        for word in ("checked_at", "sha", "duplicate", "owner"):
            self.assertTrue(any(word in item for item in errors), errors)

    def test_local_paths_and_external_url_shape_are_checked(self):
        slot = self.data["features"][0]["evidence"]["tests"]
        slot.update(status="recorded", refs=["docs/missing.md", "../escape", "https://", "https://secret@example.org/x"])
        errors, _ = self.validate()
        self.assertEqual(len(errors), 4, errors)
        slot["refs"] = ["docs/PROJECT_HYGIENE.md", "https://example.invalid/not-network-checked"]
        self.assertEqual(self.validate()[0], [])

    def test_index_and_source_lock_pointer_must_agree(self):
        (self.root / "docs/README.md").write_text("[entry](./PROJECT_HYGIENE.md)")
        (self.root / "docs/source-lock.json").write_text('{"status_snapshot":"elsewhere"}')
        errors, _ = self.validate()
        self.assertTrue(any("index" in item for item in errors))
        self.assertTrue(any("status_snapshot" in item for item in errors))

    def test_deprecation_cannot_disappear_or_claim_unapproved_removal(self):
        self.data["features"][0]["product_state"] = "已移除"
        self.assertTrue(any("deprecation" in e for e in self.validate()[0]))
        slot = {"status": "recorded", "refs": ["docs/PROJECT_HYGIENE.md"], "note": "evidence"}
        item = {"id": "legacy", "feature_id": "sample", "state": "已移除", "owner": "待分配",
                "reason": "replacement", "decision": "https://github.com/o/r/pull/1", "version": "v1",
                "inventory": slot, "replacement": slot, "compatibility": slot, "retained_tests": slot,
                "removal_scope": "old endpoint", "removal_approval": {"status": "pending", "refs": [], "note": "not approved"}}
        self.data["deprecations"] = [item]
        self.assertTrue(any("removal_approval" in e for e in self.validate()[0]))
        item["state"] = "弃用中"
        self.assertTrue(any("removal_approval" in e for e in self.validate()[0]))
        item["state"] = "已移除"
        item["removal_approval"] = slot
        self.assertEqual(self.validate()[0], [])
        item["feature_id"] = "unknown"
        self.assertTrue(any("feature_id" in e for e in self.validate()[0]))

    def test_route_omission_and_duplicate_are_detected_without_parsing_code(self):
        self.data["route_review"]["source_only"].append(copy.deepcopy(self.data["route_review"]["source_only"][0]))
        errors, _ = self.validate()
        self.assertTrue(any("duplicate" in e for e in errors))
        self.assertTrue(any("count" in e for e in errors))

    def test_malformed_collections_and_empty_recorded_evidence_fail_cleanly(self):
        for field in ("features", "deprecations"):
            before = self.data[field]
            self.data[field] = None
            self.assertTrue(self.validate()[0])
            self.data[field] = before
        self.data["features"][0]["evidence"]["api"]["status"] = "recorded"
        self.assertTrue(any("refs" in e for e in self.validate()[0]))


if __name__ == "__main__":
    unittest.main()
