#!/usr/bin/env python3
"""scripts/tests/export-answers-exclusions-selftest.sh's Python core.

DF-OFF-BY-ONE-15 regression test for EXCLUDED_CLASS_PATTERNS in
scripts/export-answers.py: the anchored probe patterns must still quarantine
every probe family, and must NOT exclude a real engineering class whose
title merely CONTAINS a protected word mid-slug.
"""
import importlib.util
import os
import sys
import unittest

_SELF_PATH = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "export-answers.py")
_spec = importlib.util.spec_from_file_location("export_answers", _SELF_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)
is_excluded_class = _mod.is_excluded_class


class ExcludedClassPatternsTest(unittest.TestCase):
    def test_probe_families_still_excluded(self):
        probes = [
            # self-test family (separator-delimited token)
            "off-by-one-self-test",
            "off-by-one-self-test-2026-07-29-tick199",
            "self-test",
            "tick12-self-test",
            "foreman-tick-132-self-test",
            # dogfood probe families
            "self-dogfood-tick23",
            "test-self-dogfood",
            "dogfood-field-test-2",
            "dogfood",
            # canary probe families
            "docs-canary-3",
            "docs-canary-x",
            "canary",
            # bare / prefix placeholders
            "test",
            "test-gap-sweep-x",
            "test-foreman-t50",
            # e2e probe families
            "e2e-tick7",
            "foreman-tick82-e2e",
            "tick89-e2e",
            "shell-script-e2e",
            "foreman-e2e-verification-pipeline",
            # one-off probe titles
            "tick88-foreman-audit",
            "ds-007",
            "ds-007-tick-106",
            "shell-say-hello-test",
            "shell-echo-hello-fix",
        ]
        for title in probes:
            self.assertTrue(
                is_excluded_class(title),
                f"is_excluded_class({title!r}) = False, want True (probe family)",
            )

    def test_contains_word_title_not_excluded(self):
        not_probes = [
            # the measured 404 victims from the defect report
            "ob1-dogfood-fib-off-by-one-index",
            "ob1-ctx-canary-deploy-oom",
            # real classes carrying the protected words mid-slug
            "dogfood-stale-premise-filing",
            "canary-deliver-failure-recurrence",
            "python-canary-staleness-probe",
            "test-mocking-http-requests",
            "test-property-based-shrinking",
            "how-to-test-a-canary-deployment-strategy",
            # fused "selftest" (no separator) is a real engineering word
            "bash-gate-selftest-asserts-repo-state-nonhermetic",
        ]
        for title in not_probes:
            self.assertFalse(
                is_excluded_class(title),
                f"is_excluded_class({title!r}) = True, want False (contains-word, not a probe)",
            )


if __name__ == "__main__":
    unittest.main()