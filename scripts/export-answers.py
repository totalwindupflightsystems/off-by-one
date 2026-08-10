#!/usr/bin/env python3
"""
Export the verified answer corpus from SQLite into flat files for distribution.

Outputs (relative to repo root):
  data/answers.jsonl            — master bulk file, one answer per line (JSON)
  data/answers/<slug>.json      — one file per problem class (PR-friendly)
  data/INDEX.md                 — human-readable catalog of problem classes
  data/COUNTS.md                — live corpus counts (auto-stamped every export)
  data/README.md                — usage guide for consumers

The flat files ARE the distribution layer: teams clone the repo (or fetch a
single file), browse, and contribute new answers via PR — no server required.
SQLite remains the operational store for the live server.

Usage:
  python3 scripts/export-answers.py [path-to-off-by-one.db]
"""
import json
import os
import re
import sqlite3
import sys
from datetime import datetime, timezone

DB_PATH = sys.argv[1] if len(sys.argv) > 1 else os.path.join(
    os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "off-by-one.db"
)
REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
DATA_DIR = os.path.join(REPO_ROOT, "data")
ANSWERS_DIR = os.path.join(DATA_DIR, "answers")

# Self-test / canary / probe classes that must never reach the public export.
# Matched (case-insensitive regex search) against the raw class title,
# lowercased. Keep these specific: real engineering classes whose titles
# merely contain "test" (e.g. "test-mocking-http-requests",
# "test-property-based-shrinking") must NOT be excluded.
EXCLUDED_CLASS_PATTERNS = [
    r"self-test",             # off-by-one-self-test family, bare "self-test"
    r"self[-_]dogfood",       # self-dogfood probes
    r"dogfood",               # test-self-dogfood, dogfood-field-test-*
    r"canary",                # docs-canary-*
    r"field-test",            # dogfood-field-test-*
    r"^test$",                # bare "test" placeholder class
    r"test-gap-sweep",
    r"test-foreman-",
    r"e2e-tick",              # e2e-tickNN probe classes
    r"tick\d+-e2e",           # reversed-form probes: foreman-tick82-e2e, tick89-e2e, tick90-e2e
    r"shell-script-e2e",      # 0001-0017-era probe class
    r"e2e-verification-pipeline",  # foreman-e2e-verification-pipeline probe
    r"foreman-audit",         # tick88-foreman-audit probe
    r"ds-007",                # DS-007 probe family (ds-007, ds-007-tick-106)
    r"shell-say-hello-test",
    r"shell-echo-hello-fix",
    r"tick\d+-self-test",     # tickN-self-test variants of the self-test family
    r"docs-canary",
]
_EXCLUDED_RES = [re.compile(p, re.IGNORECASE) for p in EXCLUDED_CLASS_PATTERNS]


def is_excluded_class(title: str) -> bool:
    """True if a raw (pre-slugify) class title is a self-test/canary/probe."""
    t = title.lower()
    return any(p.search(t) for p in _EXCLUDED_RES)


def slugify(title: str) -> str:
    s = title.strip().lower()
    s = re.sub(r"[^a-z0-9]+", "-", s)
    s = re.sub(r"-{2,}", "-", s).strip("-")
    return s[:80] or "untitled"


def main() -> None:
    db = sqlite3.connect(DB_PATH)
    db.row_factory = sqlite3.Row
    c = db.cursor()

    c.execute("""
        SELECT pc.id AS class_id, pc.title, pc.description, pc.created_at AS class_created,
               an.id AS answer_id, an.lang, an.env, an.version, an.solution,
               an.evidence, an.signatures, an.status, an.created_at AS answer_created
        FROM problem_classes pc
        JOIN answer_nodes an ON an.class_id = pc.id
        WHERE an.status = 'verified'
        ORDER BY pc.id, an.id
    """)
    rows = c.fetchall()

    # Group by class
    classes: dict[int, dict] = {}
    for r in rows:
        cls = classes.setdefault(r["class_id"], {
            "class_id": r["class_id"],
            "title": r["title"],
            "description": r["description"],
            "created_at": r["class_created"],
            "answers": [],
        })
        cls["answers"].append({
            "answer_id": r["answer_id"],
            "language": r["lang"],
            "environment": r["env"],
            "version": r["version"],
            "solution": r["solution"],
            "evidence": r["evidence"],
            "signatures": json.loads(r["signatures"]) if r["signatures"] else None,
            "status": r["status"],
            "created_at": r["answer_created"],
        })

    # Filter out self-test/canary/probe classes — lab plumbing, not verified
    # engineering answers. They must not appear in any export output.
    excluded = [cls for cls in classes.values() if is_excluded_class(cls["title"])]
    for cls in excluded:
        del classes[cls["class_id"]]
    if excluded:
        print(f"Excluded {len(excluded)} self-test/canary/probe classes:")
        for cls in sorted(excluded, key=lambda x: x["class_id"]):
            print(f"  - [{cls['class_id']}] {cls['title']} "
                  f"({len(cls['answers'])} answers)")

    os.makedirs(ANSWERS_DIR, exist_ok=True)

    # 1. Master JSONL (one answer per line, class metadata embedded)
    jsonl_path = os.path.join(DATA_DIR, "answers.jsonl")
    n_answers = 0
    with open(jsonl_path, "w") as f:
        for cls in classes.values():
            for ans in cls["answers"]:
                record = {
                    "class_id": cls["class_id"],
                    "class": slugify(cls["title"]),
                    "title": cls["title"],
                    "description": cls["description"],
                    **ans,
                }
                f.write(json.dumps(record, ensure_ascii=False) + "\n")
                n_answers += 1

    # 2. One file per class
    for cls in classes.values():
        slug = slugify(cls["title"])
        path = os.path.join(ANSWERS_DIR, f"{cls['class_id']:04d}-{slug}.json")
        with open(path, "w") as f:
            json.dump(cls, f, ensure_ascii=False, indent=2)
            f.write("\n")

    # 2b. Remove stale per-class files so data/answers/ stays in sync with
    # the export (excluded classes, renamed slugs, deleted classes).
    expected_files = {
        f"{cls['class_id']:04d}-{slugify(cls['title'])}.json"
        for cls in classes.values()
    }
    removed = 0
    for fname in sorted(os.listdir(ANSWERS_DIR)):
        if re.match(r"^\d{4}-.+\.json$", fname) and fname not in expected_files:
            os.remove(os.path.join(ANSWERS_DIR, fname))
            removed += 1
            print(f"  removed stale {fname}")
    if removed:
        print(f"Removed {removed} stale per-class files")

    # 3. INDEX.md
    index_path = os.path.join(DATA_DIR, "INDEX.md")
    now = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M UTC")
    with open(index_path, "w") as f:
        f.write(f"# Off-by-One Answer Index\n\n")
        f.write(f"**{len(classes)} problem classes · {n_answers} verified answers** · "
                f"exported {now}\n\n")
        f.write("Browse per-class files in [`data/answers/`](answers/), or use the "
                "master [`answers.jsonl`](answers.jsonl).\n\n")
        f.write("| Class ID | Problem | Answers | Languages |\n")
        f.write("|----------|---------|---------|-----------|\n")
        for cls in sorted(classes.values(), key=lambda x: -len(x["answers"])):
            langs = sorted({a["language"] for a in cls["answers"] if a["language"]})
            f.write(f"| {cls['class_id']} | {cls['title']} | {len(cls['answers'])} "
                    f"| {', '.join(langs)} |\n")

    # 3b. COUNTS.md — small auto-stamped counts file. README and docs link to
    # this instead of hardcoding corpus counts that drift within days.
    counts_path = os.path.join(DATA_DIR, "COUNTS.md")
    with open(counts_path, "w") as f:
        f.write("# Corpus counts\n\n")
        f.write(f"**{len(classes)} problem classes · {n_answers} verified answers** · "
                f"exported {now}\n\n")
        f.write("Source of truth: data/INDEX.md (regenerated every sync).\n")

    # 4. README.md (consumer guide)
    readme_path = os.path.join(DATA_DIR, "README.md")
    with open(readme_path, "w") as f:
        f.write("""# Off-by-One — Answer Distribution

This directory is the **flat-file distribution layer** of the Off-by-One
pre-solve lab. Every file is plain JSON / Markdown in a git repo — **no server,
no SQLite, no setup** needed to consume or contribute.

## What's here

| Path | Content |
|------|---------|
| `answers.jsonl` | Master file — one verified answer per line (JSON) |
| `answers/` | One JSON file per problem class — browse, diff, PR |
| `INDEX.md` | Catalog of every problem class + language coverage |
| `COUNTS.md` | Live corpus counts — auto-stamped every export |

## Consume (3 ways)

**1. Grab one file (no clone):**
```bash
curl -O https://raw.githubusercontent.com/totalwindupflightsystems/off-by-one/main/data/answers/0001-hello-world.json
```

**2. Clone everything:**
```bash
git clone --depth 1 https://github.com/totalwindupflightsystems/off-by-one
# answers in data/answers.jsonl
```

**3. Search locally (no server):**
```bash
grep -l '"title": ".*raft.*"' data/answers/*.json   # find a class
jq '.answers[0].solution' data/answers/0001-hello-world.json  # read a solution
```

## Record shape

Each line of `answers.jsonl` (and each entry in `answers/*.json`):

```json
{
  "class_id": 1,
  "class": "hello-world",
  "title": "Hello World",
  "description": "...",
  "answer_id": 1,
  "language": "go",
  "environment": "docker",
  "version": "latest",
  "solution": "...",
  "evidence": "...",
  "signatures": {"model": "bash", "result": "passed", "tests": 1},
  "status": "verified",
  "created_at": "2026-07-08 01:34:24"
}
```

## Contribute

Add or fix an answer? Open a PR that adds/updates a file under `answers/`
(or appends to `answers.jsonl`) — maintainers verify and merge.

## Freshness

Exported from the live lab's SQLite store. Regenerate with:
```bash
python3 scripts/export-answers.py
```
""")

    print(f"Exported {len(classes)} classes / {n_answers} answers → {DATA_DIR}")
    print(f"  {jsonl_path}")
    print(f"  {ANSWERS_DIR}/ (per-class files)")
    print(f"  {index_path}")
    print(f"  {counts_path}")
    print(f"  {readme_path}")


if __name__ == "__main__":
    main()
