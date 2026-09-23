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
