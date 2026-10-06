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
sanitize_internal_names = _mod.sanitize_internal_names
sanitize_corpus_text = _mod.sanitize_corpus_text
sanitize_common_pii = _mod.sanitize_common_pii


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
            "/Users/alice/Library/Secrets/key": "~/Library/Secrets/key",
            r"C:\\Users\\alice\\Documents\\private.txt": r"~\\Documents\\private.txt",
            r"\\/home\\/alice\\private\\repo": r"~\\private\\repo",
            "/mnt/c/Users/alice/project": "~/project",
            "/cygdrive/c/Users/alice/project": "~/project",
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
        # composed scrub (host paths + internal names). Source-level
        # assertion, like the exclusions test's reliance on the module's
        # public surface.
        import inspect
        src = inspect.getsource(_mod.main)
        for field in ('r["title"]', 'r["description"]', 'r["lang"]', 'r["env"]',
                      'r["version"]', 'r["solution"]', 'r["evidence"]'):
            with self.subTest(field=field):
                self.assertIn(f"sanitize_corpus_text({field}", src)
        self.assertIn('sanitize_pii_value(json.loads(r["signatures"]))', src)


class SanitizeCommonPIITest(unittest.TestCase):
    """PUBLIC-PII-001: redact common contact and network identifiers."""

    def test_emails_phones_and_ip_addresses_are_replaced(self):
        src = "contact alice@example.net at (415) 555-1212; host 203.0.113.7; v6 2001:db8::1"
        self.assertEqual(
            sanitize_common_pii(src),
            "contact <email> at <phone>; host <ip-address>; v6 <ip-address>",
        )

    def test_dotted_versions_and_non_contact_text(self):
        self.assertEqual(sanitize_common_pii("tool v1.2.3 is at /usr/bin/tool"),
                         "tool v1.2.3 is at /usr/bin/tool")

    def test_common_pii_runs_inside_composed_scrub(self):
        self.assertEqual(
            sanitize_corpus_text("/home/alice/app uses alice@example.net via 10.0.0.4"),
            "~/app uses <email> via <ip-address>",
        )


class SanitizeInternalNamesTest(unittest.TestCase):
    """REVIEW-OB-009: internal project/tool names map to placeholders."""

    def test_project_names(self):
        cases = {
            "run hermes-dagger pipeline": "run <project> pipeline",
            "chimera-v2 namespace sync": "<project> namespace sync",
            "mounted warpfs share": "mounted <project> share",
            "crier bus route 404s": "<project> bus route 404s",
        }
        for src, want in cases.items():
            with self.subTest(src=src):
                self.assertEqual(sanitize_internal_names(src), want)

    def test_gitreins_tool_path_forms(self):
        # Every documented form collapses to <tool>, swallowing its prefix
        # so no /home/<user> account name can survive to the host-path pass.
        for src in ("/home/kara/.local/bin/gitreins",
                    "/home/bunker-deadbeef/.local/bin/gitreins",
                    "~/.local/bin/gitreins",
                    ".local/bin/gitreins"):
            with self.subTest(src=src):
                self.assertEqual(sanitize_internal_names(src), "<tool>")

    def test_bare_gitreins_word_untouched(self):
        # Only the install path is redacted; the tool's name in prose stays.
        src = "gitreins guard blocks the commit"
        self.assertEqual(sanitize_internal_names(src), src)

    def test_multiple_names_in_one_text(self):
        src = "crier + warpfs + hermes-dagger + chimera-v2"
        self.assertEqual(sanitize_internal_names(src),
                         "<project> + <project> + <project> + <project>")

    def test_unrelated_text_untouched(self):
        for src in ("off-by-one corpus export", "/usr/bin/env python3", "no names here"):
            with self.subTest(src=src):
                self.assertEqual(sanitize_internal_names(src), src)

    def test_empty_and_none_pass_through(self):
        self.assertEqual(sanitize_internal_names(""), "")
        self.assertIsNone(sanitize_internal_names(None))

    def test_composed_scrub_covers_both_classes(self):
        # sanitize_corpus_text removes names AND host paths in one pass.
        self.assertEqual(
            sanitize_corpus_text("edit /home/kara/x then run crier"),
            "edit ~/x then run <project>",
        )
        self.assertEqual(
            sanitize_corpus_text("/home/kara/.local/bin/gitreins"),
            "<tool>",
        )


if __name__ == "__main__":
    unittest.main()
