#!/usr/bin/env python3
"""One-shot corpus sweep for REVIEW-OB-010: rewrite the environment/version
fields of the committed corpus (data/answers/*.json) through the same
normalize_env()/normalize_version() the export pipeline now applies
(scripts/export-answers.py), so historical prose values become canonical
discovery-filter tokens.

Pure data transform — no server, no SQLite, no network. Every other field
is preserved byte-identically: per-class files are re-dumped with the same
settings the exporter uses (ensure_ascii=False, indent=2, trailing
newline) and only written when their content actually changed (file mtime
is preserved either way so the site sitemap's lastmod derivation is not
disturbed).

data/answers.jsonl is a GENERATED, untracked bulk artifact since OBS-OB-006
(see .gitignore). When a local copy is present it is rebuilt from the
just-normalized per-class files with scripts/build-answers-jsonl.py so the
two surfaces cannot disagree; when it is absent (a fresh clone) the sweep
prints one line naming it and leaves the per-class rewrite standing — it
never crashes with ENOENT.

Idempotent: a second run reports 0 rewrites.

Usage:
  python3 scripts/normalize-corpus-tokens.py [--dry-run] [ROOT]

  ROOT (optional positional) scans that tree instead of the repo this script
  lives in — the same override convention as scripts/check-corpus-tokens.py,
  and what scripts/tests/build_answers_jsonl_test.py uses for temp fixtures.
"""
import importlib.util
import json
import os
import subprocess
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))


def _root_override():
    """Positional root override, same convention as the check-*.py guards, so a
    self-test can drive a temp fixture tree instead of the live corpus."""
    for arg in sys.argv[1:]:
        if not arg.startswith("--"):
            return os.path.abspath(arg)
    return ROOT


ROOT = _root_override()
DATA_DIR = os.path.join(ROOT, "data")
ANSWERS_DIR = os.path.join(DATA_DIR, "answers")
JSONL_PATH = os.path.join(DATA_DIR, "answers.jsonl")
# The rebuild helper ships next to this script, so a copied fixture tree
# rebuilds its own corpus rather than reaching back into the live repo.
BUILDER = os.path.join(os.path.dirname(os.path.abspath(__file__)),
                       "build-answers-jsonl.py")

_spec = importlib.util.spec_from_file_location(
    "export_answers", os.path.join(ROOT, "scripts", "export-answers.py"))
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)
normalize_env = _mod.normalize_env
normalize_version = _mod.normalize_version

DRY_RUN = "--dry-run" in sys.argv


def normalize_answer(ans, stats):
    """Normalize one answer dict in place; bump stats on change."""
    env_old = ans.get("environment")
    ver_old = ans.get("version")
    env_new = normalize_env(env_old)
    ver_new = normalize_version(ver_old)
    if env_new != (env_old or ""):
        stats["env_rewritten"] += 1
        ans["environment"] = env_new
    if ver_new != (ver_old or ""):
        stats["ver_rewritten"] += 1
        ans["version"] = ver_new
    return (env_old, env_new, ver_old, ver_new)


def main():
    stats = {"env_rewritten": 0, "ver_rewritten": 0, "files_rewritten": 0}
    samples = []

    # 1. Per-class files: rewrite only when content changes, keep mtime.
    for fname in sorted(os.listdir(ANSWERS_DIR)):
        if not fname.endswith(".json"):
            continue
        path = os.path.join(ANSWERS_DIR, fname)
        with open(path) as f:
            original_text = f.read()
        cls = json.loads(original_text)
        for ans in cls.get("answers", []):
            change = normalize_answer(ans, stats)
            if (change[0] or "") != change[1] or (change[2] or "") != change[3]:
                if len(samples) < 8 and ((change[0] or "") != change[1]):
                    samples.append((fname, change[0], change[1]))
        new_text = json.dumps(cls, ensure_ascii=False, indent=2) + "\n"
        if new_text != original_text:
            stats["files_rewritten"] += 1
            if not DRY_RUN:
                st = os.stat(path)
                with open(path, "w") as f:
                    f.write(new_text)
                os.utime(path, (st.st_atime, st.st_mtime))

    # 2. The bulk JSONL artifact is GENERATED (untracked + gitignored since
    # OBS-OB-006) and its content is derived entirely from the files rewritten
    # above, so it is rebuilt rather than hand-edited. Absent copy = fresh
    # clone: say so once and keep the per-class rewrite as the result.
    jsonl_notes = []
    if not os.path.exists(JSONL_PATH):
        jsonl_notes.append("data/answers.jsonl: not present (generated artifact, "
                           "untracked) — skipped; rebuild it with `make corpus-jsonl`")
        jsonl_status = "not present (skipped)"
    elif DRY_RUN:
        jsonl_notes.append("data/answers.jsonl: present — would be rebuilt from "
                           "the per-class files (scripts/build-answers-jsonl.py)")
        jsonl_status = "present (rebuild skipped: dry run)"
    else:
        proc = subprocess.run(
            [sys.executable, BUILDER, "--out", JSONL_PATH, "--quiet"],
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        if proc.returncode != 0:
            print("data/answers.jsonl: REBUILD FAILED — "
                  "scripts/build-answers-jsonl.py exited "
                  f"{proc.returncode}; per-class files are already rewritten",
                  file=sys.stderr)
            if proc.stderr.strip():
                print(proc.stderr.strip(), file=sys.stderr)
            return 1
        if proc.stdout.strip():
            jsonl_notes.append(proc.stdout.strip())
        jsonl_status = "rebuilt from the per-class files"

    mode = "DRY RUN — no files written" if DRY_RUN else "applied"
    print(f"normalize-corpus-tokens ({mode}):")
    for note in jsonl_notes:
        print(f"  {note}")
    print(f"  env values rewritten:    {stats['env_rewritten']}")
    print(f"  version values rewritten: {stats['ver_rewritten']}")
    print(f"  per-class files changed: {stats['files_rewritten']}")
    print(f"  answers.jsonl:           {jsonl_status}")
    if samples:
        print("  samples (env):")
        for fname, old, new in samples:
            old_s = (old or "")[:70]
            print(f"    [{fname}] {old_s!r} -> {new!r}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
