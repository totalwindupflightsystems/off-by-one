# Traceability Doctrine — Off-by-One

Status: authoritative for this repository. Fleet-level operational details live in the
(internal) `coding-hermes-traceability` skill; this document is the public contract for
what traceability means in off-by-one and where the evidence lives.

## Purpose

Traceability in off-by-one means every code change can be walked backwards from the
running system to the reason it exists: a board row, a worker commit, a guard verdict,
a test, and (when applicable) a deployment. If a change cannot be traced to a row or
an owner request, it should not exist on `main`.

## Scope

Tracked:

- **Commits** — every commit lands via the GitReins harness and carries a board task
  id in its message (`Addresses <task-id>.`).
- **Board rows** — the work ledger in `.coding-hermes/board/tasks.jsonl` (canonical)
  and the append-only history in `.coding-hermes/board/events.jsonl`.
- **Guard and judge verdicts** — `.gitreins/history/` per task attempt.
- **CI runs** — GitHub Actions runs on the pushed commit.
- **Deployments / live state** — install and dogfood evidence under `docs/dogfood/`
  when a change touches a deployed surface.

Not tracked as rows: one-line typos folded into an in-flight row, and harness state
(`.gitreins/`, board caches) itself.

## Artifacts

| Artifact | Path | Role |
|----------|------|------|
| Task board (canonical) | `.coding-hermes/board/tasks.jsonl` | One row per unit of work; `reasoning` + `acceptance_criteria` are the contract |
| Board events | `.coding-hermes/board/events.jsonl` | Append-only audit of dispatches/closures |
| Guard/judge history | `.gitreins/history/<date>/<shortid>/` | Guard output and tier-2 verdict per task |
| CI logs | GitHub Actions | Per-commit build/test proof on the pushed tip |
| Dogfood evidence | `docs/dogfood/` | Raw logs proving a change works against a live install |

## Workflow

1. **Row**: a finding becomes a board row with verbatim acceptance criteria (usually a
   shell command whose exact output must be quoted in the closeout).
2. **Dispatch**: the foreman briefs a worker; the commit message references the row id.
3. **Guard**: the commit-time GitReins guard runs secrets/build/lint/tests against the
   staged diff; a blocked commit does not exist.
4. **Judge**: tier-2 judge reads the diff against the row's criteria and writes a
   verdict under `.gitreins/history/`.
5. **Closeout**: the foreman verifies the committed tree satisfies the acceptance
   command — not the commit message's claim — then flips the row and pushes.
6. **Deploy** (when applicable): live evidence under `docs/dogfood/` ties the commit
   to observed behavior.

## Exceptions

- **Docs-only and board-only commits** may bypass the code guard lane when the hook
  exempts them; the reason goes in the commit body.
- **Emergency fixes** still land as commits with a row id; if the row is filed after
  the fix, the closeout notes the inversion explicitly.
- Traceability is never relaxed to "no commit message id" — an undocumented change is
  a finding, not a shortcut.
