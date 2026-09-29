#!/usr/bin/env python3
"""One-shot corpus sweep for REVIEW-OB-010: rewrite the environment/version
fields of the committed corpus (data/answers/*.json + data/answers.jsonl)
through the same normalize_env()/normalize_version() the export pipeline
now applies (scripts/export-answers.py), so historical prose values become
canonical discovery-filter tokens.

Pure data transform — no server, no SQLite, no network. Every other field
is preserved byte-identically: per-class files are re-dumped with the same
settings the exporter uses (ensure_ascii=False, indent=2, trailing
newline) and only written when their content actually changed (file mtime
is preserved either way so the site sitemap's lastmod derivation is not
disturbed); answers.jsonl is rewritten line-wise with the exporter's
json.dumps settings.

Idempotent: a second run reports 0 rewrites.

Usage:
  python3 scripts/normalize-corpus-tokens.py [--dry-run]
"""
import importlib.util
import json
import os
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
DATA_DIR = os.path.join(ROOT, "data")
ANSWERS_DIR = os.path.join(DATA_DIR, "answers")
JSONL_PATH = os.path.join(DATA_DIR, "answers.jsonl")

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
    stats = {"env_rewritten": 0, "ver_rewritten": 0, "files_rewritten": 0,
             "jsonl_rewritten": 0}
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

    # 2. Master JSONL, line-wise (key order + separators preserved).
    with open(JSONL_PATH) as f:
        lines = f.read().splitlines(keepends=True)
    out_lines = []
    for line in lines:
        body = line.rstrip("\n")
        if not body.strip():
            out_lines.append(line)
            continue
        rec = json.loads(body)
        change = normalize_answer(rec, stats)
        if (change[0] or "") != change[1] and len(samples) < 8:
            samples.append(("answers.jsonl", change[0], change[1]))
        new_body = json.dumps(rec, ensure_ascii=False)
        if new_body != body:
            stats["jsonl_rewritten"] += 1
        out_lines.append(new_body + "\n")
    if not DRY_RUN:
        with open(JSONL_PATH, "w") as f:
            f.writelines(out_lines)

    mode = "DRY RUN — no files written" if DRY_RUN else "applied"
    print(f"normalize-corpus-tokens ({mode}):")
    print(f"  env values rewritten:    {stats['env_rewritten']}")
    print(f"  version values rewritten: {stats['ver_rewritten']}")
    print(f"  per-class files changed: {stats['files_rewritten']}")
    print(f"  answers.jsonl lines changed: {stats['jsonl_rewritten']}")
    if samples:
        print("  samples (env):")
        for fname, old, new in samples:
            old_s = (old or "")[:70]
            print(f"    [{fname}] {old_s!r} -> {new!r}")


if __name__ == "__main__":
    main()
