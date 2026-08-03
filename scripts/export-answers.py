#!/usr/bin/env python3
"""
Export the verified answer corpus from SQLite into flat files for distribution.

Outputs (relative to repo root):
  data/answers.jsonl            — master bulk file, one answer per line (JSON)
  data/answers/<slug>.json      — one file per problem class (PR-friendly)
  data/INDEX.md                 — human-readable catalog of problem classes
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
    print(f"  {readme_path}")


if __name__ == "__main__":
    main()
