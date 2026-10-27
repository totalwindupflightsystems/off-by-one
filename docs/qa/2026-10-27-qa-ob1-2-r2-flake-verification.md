# QA-OFF-BY-ONE-2-R2 — flake premise verification at HEAD

- Date: 2026-10-27 (tick off-by-one-foreman-2026-10-27-09-09-48)
- Tested commit: cd5e5ead8ddb253bfcc8fa9ba3951f26f2c62a38 (master, clean working tree)
- Row: QA-OFF-BY-ONE-2-R2 (RECURRENCE of QA-OFF-BY-ONE-2 — fresh-agent ci-pass
  FAILED on BOTH arms for env reasons; non-reproducible by the row's own account)
- Handling: verification-only closeout (no implementation worker; the row's
  acceptance criteria are premise-verification commands, not code changes)

## Acceptance criteria — executed independently, both PASS

### Criterion 1 — 20x repeat run of the named test

Command (from the product git root):

    go test ./internal/web/ -run '^TestChat_PeriodicStatusUntilAgentFrame$' -count=20 -timeout 300s

Exit code: 0
Output tail: `ok  github.com/totalwindupflightsystems/off-by-one/internal/web  10.081s`

### Criterion 2 — clean suite run, no agent-crash lines, test present

Command:

    go test ./internal/web/ -count=1 -timeout 300s

Exit code: 0
Output tail: `ok  github.com/totalwindupflightsystems/off-by-one/internal/web  2.525s`

- `grep -c 'agent runner error: agent crashed'` on the captured suite output: 0
  (criterion: no such lines)
- `grep -c 'TestChat_PeriodicStatusUntilAgentFrame' internal/web/chat_test.go`: 3
  (criterion: >= 1)

## Remote CI state

`gh run list --repo totalwindupflightsystems/off-by-one --limit 3` at capture time:

| run | conclusion |
|-----|------------|
| pages build and deployment (master, 2026-10-27T08:53:35Z) | success |
| chore(board): idle tick 481 (master, exact tested SHA cd5e5ead) | success |
| chore(board): idle tick 480 (master) | success |

Classification: green at the exact tested commit.

## Premise assessment

The original recurrence recorded env-arm failures only (provisioned GOROOT missing
the `testing` package -> 15 packages [setup failed]; act arm's rootless dockerd
never started). Both arms are harness/environment conditions; the repo-side
stability of the named test (the distilled acceptance contract of this row) is
demonstrated at HEAD: 20/20 repeat passes and a crash-line-free suite run.

## Witness and next owner action

- witness=none:verification-only (automated foreman probe; commands and outputs
  above are reproducible from the product git root)
- verdict binding: see the board row's ch:trace / foreman_note (GitReins tier-2
  verdict id recorded at closeout)
- next owner action: none required. If a future fresh-system battery again shows
  `agent runner error: agent crashed` lines or a TestChat_PeriodicStatusUntilAgentFrame
  flake attributable to the repo (not the provisioned GOROOT / act dockerd env),
  open a new recurrence row bound to that battery evidence.
