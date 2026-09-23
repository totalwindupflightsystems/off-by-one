# Corrected GitReins guard recipe (AGENTS.md text pending approval)

`AGENTS.md` is an approval-gated protected file in this fleet: a write to it
times out by design. This file carries the exact replacement text for its
"GitReins Quality Harness" section so the owning tick (or Bane) can apply it
verbatim. Filed by review #16 finding R16-02 / board row REVIEW-OB-001.

## Replace in AGENTS.md (lines 29-31, the fenced block)

Current (dead — the directory was renamed away):

```bash
PATH="$HOME/gitreins-poc/.venv/bin:$PATH" gitreins guard
```

Corrected:

```bash
gitreins guard
```

with this note added under the block:

> `gitreins` resolves through the pipx shim at `~/.local/bin/gitreins`. Do NOT
> put a repo venv first on PATH for this: `$HOME/gitreins-poc` no longer exists
> (renamed to `~/gitreins`), and where a similar stale venv does exist, its
> tools shadow the repo interpreter and can falsely FAIL the tests lane on a
> clean tree. The tests lane is interpreter-pinned in `.gitreins/config.yaml`
> (`test_command`), so it is independent of PATH ordering.

Precedent: `terminal-jail/AGENTS.md` carries exactly this corrected form
(observed 2026-09-22).

## Status as of 2026-09-23 (review-programme re-check)

**Still not applied — this is the one remaining half of REVIEW-OB-001.**

Verified by EXECUTION on 2026-09-23, not by reading:

- The corrected recipe runs as written: `gitreins guard` → `Tier 1 Guards: PASS (test mode: full)`
  (secrets ✓, go_build ✓, go_lint ✓, go_tests ✓), exit **0**.
- `README.md` and `CONTRIBUTING.md` both already carry the corrected form and both label the old
  path as an older recipe naming a directory that no longer exists — so the row's first two docs
  are done.
- `AGENTS.md` line 30 still prescribes the deleted `$HOME/gitreins-poc/.venv` form, with no
  historical label, inside the section that calls the harness MANDATORY.
- Why it still appears to work: `~/gitreins-poc` does not exist, so the bogus PATH entry falls
  through and `gitreins` resolves to the same pipx shim. It breaks exactly in the condition README
  describes — a stale venv of that shape present, shadowing the repo interpreter and falsely
  failing the tests lane on a clean tree.

A second attempt at the gated edit was made on 2026-09-23 and the approval prompt **timed out without
a response**; silence is not consent, so it was abandoned and not retried by any other path
(no terminal, no execute_code). Applying the text above is the remaining action: either a human
approval of an `AGENTS.md` write, or the project's own foreman, which owns its instruction file.
