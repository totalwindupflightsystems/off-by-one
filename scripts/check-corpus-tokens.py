#!/usr/bin/env python3
"""REVIEW-OB-010 corpus check: the answer-level environment/version
discovery-filter fields must be canonical tokens.

env/version are exact-match tuple filters (internal/graph/discovery.go —
empty acts as the wildcard), so a value carrying whitespace, ';' or '~' is
prose: unreachable via tuple-scoped discovery and a host-context leak in
the public catalog.

Scope is deliberately the FILTER fields only: the answer records'
"environment"/"version" keys in data/answers/<class>.json (answers[] entries)
and, when it happens to exist, in the generated bulk file data/answers.jsonl.
Provenance metadata nested inside "signatures" may legitimately carry the raw
prose the solver recorded — it is not a discovery filter and is not flagged.

data/answers.jsonl is a GENERATED local artifact (gitignored, rebuilt with
scripts/build-answers-jsonl.py / `make corpus-jsonl`), so it is absent from a
fresh clone. The per-class corpus is the tracked canonical view and is ALWAYS
scanned — it carries every answer the monolith would, so an absent monolith
cannot hide a violation. A tree with NO answer view at all is a hard failure
instead of an empty green run.

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
    scanned_jsonl = False
    if os.path.exists(jsonl):
        with open(jsonl) as f:
            for i, line in enumerate(f, 1):
                line = line.strip()
                if not line:
                    continue
                rec = json.loads(line)
                check(jsonl, f"line {i} environment", rec.get("environment"))
                check(jsonl, f"line {i} version", rec.get("version"))
        scanned_jsonl = True

    answers_dir = os.path.join(ROOT, "data", "answers")
    scanned_classes = False
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
        scanned_classes = True

    if not scanned_classes and not scanned_jsonl:
        print("corpus hygiene VIOLATION: no answer corpus to scan — neither "
              f"{answers_dir} nor {jsonl} exists, so this check would prove "
              "nothing (a skipped scan is a silent pass).", file=sys.stderr)
        return 1

    if not scanned_jsonl:
        print(f"note: {jsonl} absent (generated artifact, `make corpus-jsonl`) — "
              f"scanned the canonical per-class corpus {answers_dir} instead")

    if offenders:
        print("corpus hygiene VIOLATION: prose in environment/version filter "
              "fields (whitespace, ;, ~ or redaction placeholder) under data/:", file=sys.stderr)
        for o in offenders:
            print(f"  {o}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
