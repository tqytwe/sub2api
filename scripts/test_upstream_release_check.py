import importlib.util
import pathlib
import unittest


path = pathlib.Path(__file__).with_name("check_upstream_release.py")
spec = importlib.util.spec_from_file_location("upstream_check", path)
check = importlib.util.module_from_spec(spec)
spec.loader.exec_module(check)


class ReleaseContractTests(unittest.TestCase):
    def setUp(self):
        self.lock = {
            "source_repository": "ranxi2001/sub2api",
            "source_remote": "https://github.com/ranxi2001/sub2api.git",
            "release_tag": "v2.10.3", "tag_object": "a" * 40,
            "release_commit": "b" * 40, "version_file": "2.10.3",
        }
        self.release = {"tag_name": "v2.10.3", "draft": False, "prerelease": False}

    def test_accepts_only_the_pinned_formal_release(self):
        check.validate_release(self.lock, self.release, "a" * 40, "b" * 40, "2.10.3")

    def test_rejects_wrong_source_unpublished_and_moved_tags(self):
        cases = [
            ({"source_repository": "Wei-Shaw/sub2api"}, {}, "a" * 40, "b" * 40, "2.10.3"),
            ({"source_remote": "https://github.com/other/sub2api.git"}, {}, "a" * 40, "b" * 40, "2.10.3"),
            ({}, {"draft": True}, "a" * 40, "b" * 40, "2.10.3"),
            ({}, {"prerelease": True}, "a" * 40, "b" * 40, "2.10.3"),
            ({}, {"tag_name": "v2.10.4"}, "a" * 40, "b" * 40, "2.10.3"),
            ({}, {}, "c" * 40, "b" * 40, "2.10.3"),
            ({}, {}, "a" * 40, "c" * 40, "2.10.3"),
            ({}, {}, "a" * 40, "b" * 40, "2.10.2"),
        ]
        for lock_patch, release_patch, tag, commit, version in cases:
            with self.subTest(lock=lock_patch, release=release_patch, tag=tag, commit=commit):
                with self.assertRaises(ValueError):
                    check.validate_release(self.lock | lock_patch, self.release | release_patch, tag, commit, version)

    def test_reports_number_collision_and_changed_deployed_file_separately(self):
        before = {"270_fork.sql": "old", "100_existing.sql": "old", "271_fork.sql": "keep"}
        after = {"270_upstream.sql": "new", "100_existing.sql": "changed", "272_safe.sql": "new"}
        result = check.migration_report(before, after)
        self.assertEqual(result["changed_existing"], ["100_existing.sql"])
        self.assertEqual(result["number_collisions"], {"270_upstream.sql": ["270_fork.sql"]})
        self.assertEqual(result["fork_only_preserve"], ["270_fork.sql", "271_fork.sql"])
        self.assertEqual(before["100_existing.sql"], "old")
        self.assertEqual(result, check.migration_report(before, after))


if __name__ == "__main__":
    unittest.main()
