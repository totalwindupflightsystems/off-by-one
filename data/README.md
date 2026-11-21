# Off-by-One — Answer Distribution

This directory is the **flat-file distribution layer** of the Off-by-One
pre-solve lab. Every file is plain JSON / Markdown in a git repo — **no server,
no SQLite, no setup** needed to consume or contribute.

## What's here

| Path | Content |
|------|---------|
| `answers/` | **Canonical, tracked** — one JSON file per problem class — browse, diff, PR |
| `answers.jsonl` | Generated bulk view — one verified answer per line. **Not tracked**: rebuild locally with `make corpus-jsonl` |
| `INDEX.md` | Catalog of every problem class + language coverage |
| `COUNTS.md` | Live corpus counts — auto-stamped every export |

## Privacy-by-default

Answers are scrubbed before publication because agents can copy real workstation
context into otherwise useful diagnostics. Personal Linux/macOS/Windows home
roots become `~`; email addresses, phone numbers, and IPv4/IPv6 literals become
`<email>`, `<phone>`, and `<ip-address>`. The same policy is enforced on the
read-only public API and by CI against the generated `data/` and `site/` artifacts.
Gitleaks still scans credentials; it is not the PII filter.

## Consume (3 ways)

**1. Grab one file (no clone):**
```bash
curl -O https://raw.githubusercontent.com/totalwindupflightsystems/off-by-one/master/data/answers/0001-unknown.json
```

**2. Clone everything:**
```bash
git clone --depth 1 https://github.com/totalwindupflightsystems/off-by-one
# answers live in data/answers/*.json
make corpus-jsonl   # optional: materialise data/answers.jsonl (bulk, one line per answer)
```

**3. Search locally (no server):**
```bash
grep -l '"title": ".*raft.*"' data/answers/*.json   # find a class
jq '.answers[0].solution' data/answers/0001-unknown.json  # read a solution
```

## Record shape

Each entry in `answers/*.json` (and each line of the generated `answers.jsonl`):

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
(the per-class files are the tracked source of truth) — maintainers verify and
merge.

## Freshness

Exported from the live lab's SQLite store. Regenerate with:
```bash
python3 scripts/export-answers.py
```

`answers.jsonl` is a generated local build artifact (gitignored, not part of the
clone). Rebuild it from the tracked per-class files at any time with
`make corpus-jsonl`; `make corpus-jsonl-check` proves a local copy matches a
fresh rebuild.
