# Off-by-One — Answer Distribution

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
