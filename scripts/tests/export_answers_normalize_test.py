#!/usr/bin/env python3
"""REVIEW-OB-010 regression test for normalize_env()/normalize_version() in
scripts/export-answers.py.

environment/version are exact-match discovery filters (discovery.go ranks
answers by (env, lang, version) tuple; empty = wildcard). The corpus
carried free-text prose + host context in both fields, which made those
answers unreachable via tuple-scoped discovery and leaked operator context
into the public catalog. This test pins the canonical mapping:

  "docker"                                        -> "docker"   (clean token)
  "Linux, crier repo ~/crier"                     -> "linux"    (clean leading segment)
  "~/auger; ...; GitHub Actions ... SHA passed"   -> "github-actions"
  "gitreins 0.14.0"                               -> "0.14.0"
  "api.deepseek.com/v1, verified 2026-09-15..."   -> ""         (host prose dropped)
  "warpfs (hilo) Rust workspace"                  -> ""         (unrecognizable prose dropped)

and proves the output contract the hygiene guard enforces: a normalized
value never contains whitespace, ';' or '~'.

Pure in-process unittest: no network, no DB, no credentials.
"""
import importlib.util
import os
import re
import unittest

_SELF_PATH = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "export-answers.py")
_spec = importlib.util.spec_from_file_location("export_answers", _SELF_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)
normalize_env = _mod.normalize_env
normalize_version = _mod.normalize_version

FORBIDDEN = re.compile(r"[\s;~<>]")

ENV_CASES = [
    # (input, expected)
    ("docker", "docker"),
    ("Linux", "linux"),
    ("  node20  ", "node20"),
    ("linux/amd64", "linux/amd64"),
    ("postgresql-16", "postgresql-16"),
    ("", ""),
    (None, ""),
    ("Linux, crier repo ~/crier", "linux"),
    ("linux; go1.26; GNU coreutils", "linux"),
    ("postgres-16, pgx, chi, zerolog/hlog", "postgres-16"),
    ("~/auger; DuckBrain live substrate; GitHub Actions exact implementation SHA passed", "github-actions"),
    ("chimera-v2 integration tests, pytest + httpx, CI on GitHub Actions", "github-actions"),
    ("self-hosted GitHub Actions runner on a shared Linux host", "github-actions"),
    ("linux host, go 1.26.6", "linux"),
    ("n100 wsl2; scheduler dispatch via bash", "linux"),
    ("ubuntu linux host, python 3.11 client", "linux"),
    ("node 22, vitest, TS 7 strict", "node"),
    ("vitest 4 + node 22, repo @ 18c45f7", "node"),
    ("python 3.11, uv, pytest", "python3"),
    ("repo with go 1.26 toolchain and make", "go1.26"),
    ("set -uo pipefail bash hermetic test", "bash"),
    ("GitHub Actions exact implementation SHA passed and Linux, crier repo ~/crier", "github-actions"),
    ("warpfs (hilo) Rust workspace", ""),
    ("~/auger", ""),
    # REVIEW-OB-009: a token carrying a redaction placeholder is not a
    # meaningful filter value — drop to the wildcard.
    ("hilo/<project>", ""),
    ("hilo/<project>, Rust workspace, tracing-subscriber 0.3.23", ""),
]

VERSION_CASES = [
    ("latest", "latest"),
    ("0.14.0", "0.14.0"),
    ("v0.1.1", "v0.1.1"),
    ("go1.26", "go1.26"),
    ("20", "20"),
    ("", ""),
    (None, ""),
    ("gitreins 0.14.0", "0.14.0"),
    ("node 22", "22"),
    ("h3-test v1.0.0", "v1.0.0"),
    ("main e50aea4", "main"),
    ("api.deepseek.com/v1, verified 2026-09-15T02:39Z", ""),
    ("verified on the shared host", ""),
    # REVIEW-OB-009: placeholders are not filter tokens — a real
    # version-shaped field still wins, a bare placeholder drops.
    ("<tool> 0.14.0", "0.14.0"),
    ("hilo/<project>", ""),
]


class TestNormalizeEnv(unittest.TestCase):
    def test_mapping(self):
        for raw, want in ENV_CASES:
            with self.subTest(raw=raw):
                self.assertEqual(want, normalize_env(raw))

    def test_output_contract_no_forbidden_chars(self):
        for raw, _ in ENV_CASES:
            with self.subTest(raw=raw):
                self.assertIsNone(FORBIDDEN.search(normalize_env(raw)))


class TestNormalizeVersion(unittest.TestCase):
    def test_mapping(self):
        for raw, want in VERSION_CASES:
            with self.subTest(raw=raw):
                self.assertEqual(want, normalize_version(raw))

    def test_output_contract_no_forbidden_chars(self):
        for raw, _ in VERSION_CASES:
            with self.subTest(raw=raw):
                self.assertIsNone(FORBIDDEN.search(normalize_version(raw)))


if __name__ == "__main__":
    unittest.main()
