#!/usr/bin/env python3
"""DF-OFF-BY-ONE-33 regression test for the dotless user@host identifier
classifier shared by scripts/export-answers.py (sanitize_common_pii) and
scripts/check-public-pii.py (scan_text).

The dotted-TLD email pattern misses user@host forms (user@localhost,
kara@builder-machine), but a bare local@host regex also matches legitimate
programming syntax — CI action tags (actions/checkout@v4), package/module
versions (vitest@4, net/http@go1.26.5), VCS specs (pkg@git+https://…),
workflow scope ids (run@step1) and decorator-adjacent fragments
(n@dataclass). The classifier must redact the former and keep the latter,
and the exporter and the guard must agree on every input (a mismatch makes
the guard fail on the exporter's own output).

Pure in-process unittest: no network, no DB, no credentials. The end-to-end
proof that the corpus is clean is `make check-corpus-hygiene` over the
regenerated data/ + site/ trees.
"""
import importlib.util
import os
import unittest

_HERE = os.path.dirname(os.path.abspath(__file__))


def _load(name, filename):
    spec = importlib.util.spec_from_file_location(name, os.path.join(_HERE, "..", filename))
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


_export = _load("export_answers", "export-answers.py")
_guard = _load("check_public_pii", "check-public-pii.py")

sanitize_common_pii = _export.sanitize_common_pii
sanitize_dotless_identifiers = _export.sanitize_dotless_identifiers
scan_text = _guard.scan_text

# Real dotless user@host identifiers — must be redacted on export and
# flagged by the guard.
REDACT_CASES = {
    "user@host": "<email>",
    "ssh user@localhost": "ssh <email>",
    "scp out.tar user@builder-machine:/tmp": "scp out.tar <email>:/tmp",
    "Trace archived from kara@builder-machine. Done": "Trace archived from <email>. Done",
    "postgres://duckbrain:pw@localhost:5432/db": "postgres://duckbrain:<email>:5432/db",
    "contact admin@localhost, not later": "contact <email>, not later",
    "two: user@host and ops@builder-machine": "two: <email> and <email>",
}

# Programming syntax that superficially matches local@host — must pass
# through unchanged and must NOT be flagged by the guard.
KEEP_CASES = [
    # CI action tags and version refs
    "uses: actions/checkout@v4",
    "uses: actions/setup-go@v5",
    "golangci/golangci-lint-action@v6 with version: v1.64.8",
    "pin vitest@4 and zod@3, upgrade pnpm@11",
    "bar@v0.3.4 matches module@version; go get pkg@vX.Y.Z",
    "v8@latest and consensus@v0",
    # commit pins and branch refs
    "repo.git@main and h3src@main",
    "duckbrain@77b5b8b and main@85e8a61",
    # VCS install spec
    "pip install gitreins@git+https://example.invalid/repo.git@6f7a02b",
    # Go module version continuation (dotless match stops at the dot)
    "Found in: net/http@go1.26.5 Fixed in: net/http@go1.26.6",
    # scoped npm package with version
    "@vitest/coverage-v8@2.1.9 from lock file",
    # version-bearing model@provider routing labels
    "worker glm-5.3-flash@zai-glm-default and gpt-5.6-luna@openai-codex",
    # workflow run-scope placeholders
    'run scope "run@step1" and nested "run@each@item-1@step1" collide on run_id@step',
    # single-letter fragments from escapes next to decorators/keywords
    "escaped \\n@dataclass and \\t@echo read raw as n@dataclass t@echo",
    "raw t@mkdir and t@if fragments",
    # regex fragment locals
    r"const re = /PORT=(\d+)\s+@heading\/api/",
    # dotted email is owned by the standard email rule, not the dotless one
    "alice@example.net",
]


class SanitizeDotlessIdentifiersTest(unittest.TestCase):
    def test_real_user_host_identifiers_are_redacted(self):
        for src, want in REDACT_CASES.items():
            with self.subTest(src=src):
                self.assertEqual(sanitize_dotless_identifiers(src), want)

    def test_programming_syntax_is_kept(self):
        for src in KEEP_CASES:
            with self.subTest(src=src):
                self.assertEqual(sanitize_dotless_identifiers(src), src)

    def test_composed_scrub_mixes_dotless_and_standard_rules(self):
        src = "run actions/checkout@v4 then ssh user@localhost and mail alice@example.net"
        self.assertEqual(
            sanitize_common_pii(src),
            "run actions/checkout@v4 then ssh <email> and mail <email>",
        )

    def test_empty_and_none_pass_through(self):
        self.assertEqual(sanitize_dotless_identifiers(""), "")
        self.assertIsNone(sanitize_dotless_identifiers(None))


class GuardDotlessParityTest(unittest.TestCase):
    """check-public-pii.scan_text must classify exactly like the exporter:
    flag what the exporter would redact, ignore what it keeps."""

    def test_guard_flags_real_user_host_identifiers(self):
        for src in REDACT_CASES:
            with self.subTest(src=src):
                counts = scan_text(src)
                self.assertIn("dotless host identifier", counts)
                self.assertGreaterEqual(counts["dotless host identifier"], 1)

    def test_guard_ignores_programming_syntax(self):
        for src in KEEP_CASES:
            with self.subTest(src=src):
                self.assertNotIn("dotless host identifier", scan_text(src))

    def test_exporter_and_guard_agree_on_mixed_text(self):
        # Whatever survives the exporter must not trip the guard.
        for src in list(REDACT_CASES) + KEEP_CASES:
            with self.subTest(src=src):
                exported = sanitize_common_pii(src)
                self.assertNotIn("dotless host identifier", scan_text(exported))


if __name__ == "__main__":
    unittest.main()
