#!/usr/bin/env python3
"""Knowledge-freshness gate (OB-GAP-049).

Scans skills/**/*.md and docs/dogfood/*.md (excluding dated 2026-*-integration.md
archives) for lines that cite a board-completed task ID as still ACTIVE.

Active markers, on the same line as the ID or the line immediately following:
open, active, always, still, not fixed, TODO, broken, fails silently, always empty.
A line like "(OB-GAP-047 open)" or "fails silently ... (OB-GAP-048 open)" flags.

Historical records are exempt and must NOT flag: lines inside sections whose
heading marks them as findings/filed/error history/verified-fixed (e.g.
"### New findings (tasks OB-GAP-046..050, filed 2026-08-20)"), and blocks that
carry an uppercase FIXED citation marker ("FIXED (OB-GAP-020, tick 285)").

Exit 0 when clean; exit 1 with file:line matches when drift is found.
"""
import json
import os
import re
import sys

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
BOARD = os.path.join(REPO, ".coding-hermes", "board", "tasks.jsonl")
SKILLS_DIR = os.path.join(REPO, "skills")
DOGFOOD_DIR = os.path.join(REPO, "docs", "dogfood")

ARCHIVE_RE = re.compile(r"^2026-.*-integration\.md$")
ID_RE = re.compile(r"\b(OB-GAP-\d+)\b")

# Active-status citation markers (case-insensitive).
ACTIVE_MARKERS = [
    r"\bopen\b",
    r"\bactive\b",
    r"\balways\b",
    r"\bstill\b",
    r"\bnot fixed\b",
    r"\btodo\b",
    r"\bbroken\b",
    r"\bfails silently\b",
    r"\balways empty\b",
]
MARKER_RE = re.compile("|".join(ACTIVE_MARKERS), re.IGNORECASE)

# Sections whose heading marks them as historical records, not active claims.
HISTORICAL_HEADING_RE = re.compile(
    r"(findings|filed|historical|archive|records|changelog|history|fixed)",
    re.IGNORECASE,
)
HEADING_RE = re.compile(r"^\s{0,3}#{1,6}\s+")
# Uppercase FIXED citation marker, e.g. "FIXED (OB-GAP-020, tick 285)".
FIXED_RE = re.compile(r"\bFIXED\b")


def completed_ids():
    with open(BOARD, encoding="utf-8") as f:
        rows = [json.loads(l) for l in f if l.strip()]
    return {
        str(r.get("id", "")).strip()
        for r in rows
        if str(r.get("status", "")).strip().lower() == "complete"
    }


def target_files():
    files = []
    for root, _dirs, names in os.walk(SKILLS_DIR):
        for n in names:
            if n.endswith(".md"):
                files.append(os.path.join(root, n))
    for n in sorted(os.listdir(DOGFOOD_DIR)):
        if n.endswith(".md") and not ARCHIVE_RE.match(n):
            files.append(os.path.join(DOGFOOD_DIR, n))
    return sorted(files)


def block_fixed_flags(lines):
    """Per-line bool: the blank-line block this line belongs to cites FIXED."""
    flags = [False] * len(lines)
    cur = []
    for idx, line in enumerate(lines):
        if line.strip():
            cur.append(idx)
        elif cur:
            bh = any(FIXED_RE.search(lines[j]) for j in cur)
            for j in cur:
                flags[j] = bh
            cur = []
    if cur:
        bh = any(FIXED_RE.search(lines[j]) for j in cur)
        for j in cur:
            flags[j] = bh
    return flags


def scan_file(path, ids):
    with open(path, encoding="utf-8") as f:
        lines = f.read().splitlines()
    fixed_flags = block_fixed_flags(lines)
    section_historical = False
    hits = []
    for i, line in enumerate(lines):
        if HEADING_RE.match(line):
            section_historical = bool(HISTORICAL_HEADING_RE.search(line))
        if not ID_RE.search(line):
            continue
        if section_historical or fixed_flags[i]:
            continue
        nxt = lines[i + 1] if i + 1 < len(lines) else ""
        if not MARKER_RE.search(line + " " + nxt):
            continue
        for m in ID_RE.finditer(line):
            if m.group(1) in ids:
                hits.append((i + 1, line))
                break
    return hits


def main():
    try:
        ids = completed_ids()
    except FileNotFoundError:
        print(f"knowledge-freshness: ERROR — board not found: {BOARD}", file=sys.stderr)
        return 2
    if not ids:
        print("knowledge-freshness: ERROR — no completed IDs in board", file=sys.stderr)
        return 2

    files = target_files()
    matches = []
    for path in files:
        for ln, line in scan_file(path, ids):
            matches.append((os.path.relpath(path, REPO), ln, line))

    if matches:
        print("knowledge-freshness: DRIFT — completed task IDs cited as still ACTIVE:")
        for rel, ln, line in matches:
            print(f"  {rel}:{ln}: {line.strip()}")
        print(
            f"{len(matches)} drift citation(s) in "
            f"{len({p for p, _, _ in matches})} file(s)"
        )
        return 1

    print(
        f"knowledge-freshness: clean — {len(ids)} completed IDs, "
        f"{len(files)} file(s) scanned, no active cites"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
