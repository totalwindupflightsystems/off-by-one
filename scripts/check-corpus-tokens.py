#!/usr/bin/env python3
"""REVIEW-OB-010 corpus check: the answer-level environment/version
discovery-filter fields must be canonical tokens.

env/version are exact-match tuple filters (internal/graph/discovery.go —
empty acts as the wildcard), so a value carrying whitespace, ';' or '~' is
prose: unreachable via tuple-scoped discovery and a host-context leak in
the public catalog.

Scope is deliberately the FILTER fields only: the answer records'
"environment"/"version" keys in data/answers.jsonl and
data/answers/<class>.json (answers[] entries). Provenance metadata nested
inside "signatures" may legitimately carry the raw prose the solver
recorded — it is not a discovery filter and is not flagged.

Exit 1 (listing offenders) when a prose value is found; exit 0 when clean.
Pure data read — no network, no DB.

Usage: python3 scripts/check-corpus-tokens.py [corpus-root]   (default: repo root)
"""
import json
import os
import re
import sys

FORBIDDEN = re.compile(r"[\s;~<>]")

ROOT = sys.argv[1] if len(sys.argv) > 1 else os.path.dirname(
    os.path.dirname(os.path.abspath(__file__)))


def main():
    offenders = []

    def check(path, where, value):
        if isinstance(value, str) and FORBIDDEN.search(value):
            offenders.append(f"{path} ({where}: {value[:80]!r})")

    jsonl = os.path.join(ROOT, "data", "answers.jsonl")
    if os.path.exists(jsonl):
        with open(jsonl) as f:
            for i, line in enumerate(f, 1):
                line = line.strip()
                if not line:
                    continue
                rec = json.loads(line)
                check(jsonl, f"line {i} environment", rec.get("environment"))
                check(jsonl, f"line {i} version", rec.get("version"))

    answers_dir = os.path.join(ROOT, "data", "answers")
    if os.path.isdir(answers_dir):
        for fname in sorted(os.listdir(answers_dir)):
            if not fname.endswith(".json"):
                continue
            path = os.path.join(answers_dir, fname)
            with open(path) as f:
                cls = json.load(f)
            for j, ans in enumerate(cls.get("answers", [])):
                check(path, f"answers[{j}] environment", ans.get("environment"))
                check(path, f"answers[{j}] version", ans.get("version"))

    if offenders:
        print("corpus hygiene VIOLATION: prose in environment/version filter "
              "fields (whitespace, ;, ~ or redaction placeholder) under data/:", file=sys.stderr)
        for o in offenders:
            print(f"  {o}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
