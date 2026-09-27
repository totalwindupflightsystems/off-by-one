#!/usr/bin/env python3
"""REVIEW-OB-006 regression test for sanitize_host_paths() in
scripts/export-answers.py.

The answer corpus is public (github.com/totalwindupflightsystems/off-by-one),
so no operator host path may leak into it. This test proves the mapping:

  /home/kara/.local/bin/pi-agent   -> ~/.local/bin/pi-agent
  /home/bunker-eduos-agent/x       -> ~/x
  /home/runner/work/...            -> ~/work/...
  /home/user                       -> ~
  /home/<any account matching [A-Za-z0-9_-]+> -> ~

and that non-path text is left untouched.

Pure in-process unittest: no network, no DB, no credentials. The end-to-end
proof that the corpus is clean is `make check-corpus-hygiene` over the
regenerated data/ + site/ trees.
"""
import importlib.util
import json
import os
import unittest

_SELF_PATH = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "export-answers.py")
_spec = importlib.util.spec_from_file_location("export_answers", _SELF_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)
sanitize_host_paths = _mod.sanitize_host_paths


class SanitizeHostPathsTest(unittest.TestCase):
    def test_brief_example_mapping(self):
        self.assertEqual(
            sanitize_host_paths("/home/kara/.local/bin/pi-agent"),
            "~/.local/bin/pi-agent",
        )

    def test_known_operator_accounts(self):
        cases = {
            "/home/kara/repo": "~/repo",
            "/home/bunker/x": "~/x",
            "/home/bunker-eduos-agent/voice-harness": "~/voice-harness",
            "/home/bunker-2cdce4d0/.bunker/ports": "~/.bunker/ports",
            "/home/runner/work/sdk-python/sdk-python": "~/work/sdk-python/sdk-python",
            "/home/user/repo/engine": "~/repo/engine",
            "/home/agent/solutions": "~/solutions",
        }
        for src, want in cases.items():
            with self.subTest(src=src):
                self.assertEqual(sanitize_host_paths(src), want)

    def test_generic_account_names(self):
        # Uppercase, digits, underscores — any plausible account name.
        self.assertEqual(sanitize_host_paths("/home/OLDUSER/.venv/bin"), "~/.venv/bin")
        self.assertEqual(sanitize_host_paths("/home/svc_01/data"), "~/data")

    def test_multiple_paths_in_one_text(self):
        src = "copy /home/kara/a to /home/bunker-deadbeef/b and keep /usr/bin/c"
        self.assertEqual(sanitize_host_paths(src), "copy ~/a to ~/b and keep /usr/bin/c")

    def test_non_path_text_untouched(self):
        untouched = [
            "/usr/bin/env python3",
            "/etc/hostname",
            "home/kara is not absolute",          # no leading slash
            "/home",                              # bare root, no account
            "/home/<you>/project",                # template placeholder, no account match
            "see $HOME/.config and ~/bin",
            "no paths at all",
        ]
        for src in untouched:
            with self.subTest(src=src):
                self.assertEqual(sanitize_host_paths(src), src)

    def test_empty_and_none_pass_through(self):
        self.assertEqual(sanitize_host_paths(""), "")
        self.assertIsNone(sanitize_host_paths(None))

    def test_json_shape_unchanged(self):
        # Sanitizing inside a JSON string value must keep the document valid
        # and leave every key and non-path value byte-identical.
        src = json.dumps({
            "model": "deepseek-v4-flash",
            "passed": True,
            "log": "wrote /home/kara/out.json; ran /usr/bin/true",
        })
        out = sanitize_host_paths(src)
        doc = json.loads(out)
        self.assertEqual(doc["model"], "deepseek-v4-flash")
        self.assertIs(doc["passed"], True)
        self.assertEqual(doc["log"], "wrote ~/out.json; ran /usr/bin/true")

    def test_record_build_uses_sanitizer(self):
        # Guard against regression to raw field passthrough: the SELECT row
        # assembly in main() must route every text field through the
        # sanitizer. Source-level assertion, like the exclusions test's
        # reliance on the module's public surface.
        import inspect
        src = inspect.getsource(_mod.main)
        for field in ('r["title"]', 'r["description"]', 'r["lang"]', 'r["env"]',
                      'r["version"]', 'r["solution"]', 'r["evidence"]', 'r["signatures"]'):
            with self.subTest(field=field):
                self.assertIn(f"sanitize_host_paths({field}", src)


if __name__ == "__main__":
    unittest.main()
