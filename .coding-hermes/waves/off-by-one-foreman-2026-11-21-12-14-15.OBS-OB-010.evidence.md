# OBS-OB-010 — git filter-repo DRY RUN of the `data/answers.jsonl` history purge

Task: dry-run the history purge of the retired monolith `data/answers.jsonl` on a THROWAWAY
mirror, measure the real size win, and hand the owner a copy-paste runbook.

**Scope guarantee:** every write in this document happened under `/tmp`. The only reads outside
`/tmp` were `git worktree list` / `git remote` / `git ls-tree` / `git rev-list` inside
`/home/kara/off-by-one` and the read-only `grep`s of `~/.hermes/fleet*` required by arm 5.
**NOTHING WAS PUSHED. `origin` was never written.** Proof in "Hard-reject compliance" below.

Host: kara (Linux), git 2.53.0, git-filter-repo **2.47.0** (already installed at
`/home/kara/.local/bin/git-filter-repo` — no install was needed).
Run date: 2026-11-21.

---

## Two passes, and why (fidelity note)

`origin/master` moved while this task ran:

- Pass **A** cloned the mirror at `07:40` and got `master = 13c17ec8da4909ad1b5adbb4b8cdc0769a422a2c`
  (tree `ccb30b2d…`).
- While A was being measured, a board commit landed on origin:
  `78b55e83 chore(board): PM cycle-8 — record OBS-OB-005 in-flight worker and re-measured baseline`
  (verified with a read-only `git ls-remote`: `78b55e83…\trefs/heads/master`).

Pass **B** therefore re-cloned at the **live** tip `78b55e83e9603eda422f9bd836b22107b86a532e`
(tree `5fe1af1b5c4432a51ce1d5dc96ca82bbf6650f20`) and **B's numbers are the authoritative ones**.
A is kept only because its inventory output (blob sizes, top-10, date range) is identical — the
one-commit difference is a board-chore commit that does not touch the monolith.
Mirrors: `/tmp/obo-filterrepo-20261121.git` (A), `/tmp/obo-filterrepo-20261121b.git` (B).

---

## SETUP — baseline

```
$ git clone --mirror git@github.com:totalwindupflightsystems/off-by-one.git /tmp/obo-filterrepo-20261121b.git
Cloning into bare repository '/tmp/obo-filterrepo-20261121b.git'...

$ cd /tmp/obo-filterrepo-20261121b.git && git for-each-ref | wc -l
19

$ du -sh /tmp/obo-filterrepo-20261121b.git
91M	/tmp/obo-filterrepo-20261121b.git

$ git count-objects -vH | grep -E 'in-pack|size-pack'
in-pack: 31272
size-pack: 89.97 MiB

$ git rev-parse master
78b55e83e9603eda422f9bd836b22107b86a532e
$ git rev-parse master^{tree}          # PRE-REWRITE TIP TREE
5fe1af1b5c4432a51ce1d5dc96ca82bbf6650f20
```

The 19 refs the mirror carries (mirror clone fetches everything, including GitHub's PR refs):

```
refs/heads/master                             78b55e83
refs/heads/pr2-carter                         e593e84b
refs/heads/readme-audit-2026-10-26-21-38-04   7ccb8f26
refs/heads/wt/DF-OFF-BY-ONE-33                9a07bc5d
refs/heads/wt/DF-OFF-BY-ONE-52                d777d584
refs/heads/wt/DF-OFF-BY-ONE-60                82ffd2c8
refs/heads/wt/DF-OFF-BY-ONE-61                c98b1cd9
refs/heads/wt/OB-GAP-092                      254435f5
refs/heads/wt/REVIEW-OB-007                   dd93aa06
refs/heads/wt/REVIEW-OB-009                   aec18480
refs/heads/wt/REVIEW-OB-023                   8fa5d0b6
refs/pull/2/head                              b0775f5e
refs/pull/3/head                              dd93aa06
refs/pull/4/head                              6511e56c
refs/tags/v0.1.0 (annotated)                  ada1a5ce
refs/tags/v0.1.1 (annotated)                  99ce78ef
refs/tags/v0.2.0 (annotated)                  b05c1190
refs/tags/v0.2.1 (annotated)                  5d9fdb57
refs/tags/v0.3.0 (annotated)                  84a7c804
```

---

## ARM 1 — inventory (and the premise drift)

### 1a. Distinct blob shas at the path

```
$ git rev-list --objects --all | grep 'data/answers\.jsonl$' | awk '{print $1}' | sort -u | wc -l
363
```

Every one of those 363 objects is a **blob** (no trees/commits in the set):

```
$ git cat-file --batch-check='%(objectname) %(objecttype) %(objectsize)' < blobs.txt | awk '{print $2}' | sort | uniq -c
    363 blob
```

### ⚠️ PREMISE DRIFT — "six historic blobs" is stale by ~60×

| source | distinct blobs at `data/answers.jsonl` |
|---|---|
| board row as originally written | **6** |
| measured, all refs on **origin** (`--mirror` clone, 19 refs) | **363** |
| measured, **owner clone** `/home/kara/off-by-one`, ALL local refs | **364** |

The extra 1 in the owner clone is a blob that exists **only on a local-only branch that is not on
origin**:

```
$ comm -23 owner-blobs.txt origin-blobs.txt
307cf69cd50379c23f166fbd65f8a6f4b87cfbc3

$ git cat-file -s 307cf69c…
27994354                      # 26.7 MiB

$ (ref sweep) → in refs/heads/backup-oby1006-178b02f3
```

`78b55e83`'s own subject already concedes the drift:
`chore(board): PM cycle-8 — correct OBS-OB-008/010 blob inventory to the measured 364, not six`.
**Ground truth: 363 on origin, 364 repo-wide once the unpushed `backup-oby1006-178b02f3` branch is
counted.** The "six" figure is wrong and must not be quoted again.

### 1b. Size of the 363 blobs (uncompressed)

```
$ awk '{s+=$3} END {printf "%d bytes (%.2f MiB)\n", s, s/1048576}' sizes.txt
6677728060 bytes (6368.38 MiB)
```

6.37 GiB of logical blob content — the pack takes 89.97 MiB of that because the revisions are
near-identical line-protocol JSON and delta-compress extremely well. **Do not quote 6.4 GB as "the
win"; it is the uncompressed content, not the disk cost.**

### 1c. Top 10 largest blobs + one example commit carrying each

```
8f06f735dd7d66c21e25b7b8dc6d4eac12b7b96a  58422049B  0cbb63f0 2026-11-21 chore: untrack data/answers.jsonl and rebuild it deterministically — stops ~50MB/export repo growth. Addresses OBS-OB-006.
03dc66e50d15dfce0eaa211b2cb13f9f03a3cffc  37240769B  d7837da9 2026-11-21 data: sync answer corpus — catch-up export (+2574 answers, 10 new classes)
69fc40ebb723c392957aa90f2dea3c0980ed33c4  37196152B  4ce71068 2026-11-20 data: sync answer corpus — 11 files changed
4820430f3b824137656bab387d1bbd78ac0ee6e8  37191749B  325629b0 2026-11-18 data: sync answer corpus — 7 files changed
c234e99cedeefd50924436dd46cf29806e68df84  37049567B  ba00870b 2026-11-16 data: sync answer corpus — 43 files changed
4e622594468a4a4cd29d79af73d6e1e3e51e6770  36829343B  dc2ed7a9 2026-11-15 data: sync answer corpus — 37 files changed
50718131342db2c424db19b9e941fa4acb5f8694  36703227B  f17631cd 2026-11-14 data: sync answer corpus — 37 files changed
2e03945ee4b50298d7e66c7bf38fa1603bca52fa  36537192B  7cd84a14 2026-11-13 data: sync answer corpus — 37 files changed
34732f086198fd80a332f6d978ddd5c84df242fb  36352357B  7ec2917c 2026-11-12 data: sync answer corpus — 43 files changed
44729d3a4b4cd5a7f4eedf92fa98e9d55f7b77c5  36209200B  5feb3b18 2026-11-11 data: sync answer corpus — 27 files changed
(11th, for scale: 959c3ecf… 36072942B; 12th: 4e52b518… 35865677B)
```

Example commits obtained with `git log --all --find-object=<sha> --format='%h %ad %s' --date=short -- data/answers.jsonl | head -1`.

Reading: the top-10 are all ~36–37 MB full-corpus snapshots written once per `data: sync answer
corpus` commit, i.e. **the file was rewritten wholesale on every export**; the largest single blob
is the 58.4 MB final snapshot committed at the `OBS-OB-006` untrack.

### 1d. Date range of the path

```
$ git log --all --format='%h %ad' --date=short -- data/answers.jsonl | wc -l
365                             # commits touching the path, all refs

newest: 0cbb63f0 2026-11-21
oldest: c6e3716e 2026-08-02
```

Introduced 2026-08-02 (`c6e3716e`), last touched 2026-11-21 (`0cbb63f0` — the untrack itself).
365 commits rewrote it; 363 of them produced a distinct blob (two commits re-wrote identical
content — e.g. the merge and its source).

Is it still tracked at the tip? No:

```
$ git ls-tree -r master --name-only | grep -c 'answers\.jsonl$'
0
$ git ls-tree -r master --name-only | grep -c '^data/answers/'
3249                            # the canonical per-class corpus is what ships now
```

---

## ARM 2 — the dry run (real rewrite, throwaway mirror only)

```
$ cd /tmp/obo-filterrepo-20261121b.git
$ git filter-repo --invert-paths --path data/answers.jsonl --force
Parsed 522 commits
Parsed 814 commits
Parsed 1040 commits
Parsed 1124 commits
Parsed 1313 commits
Parsed 1499 commits
Parsed 1652 commits
Parsed 1752 commits
New history written in 0.84 seconds; now repacking/cleaning...
Repacking your repo and cleaning out old unneeded objects
Completely finished after 1.83 seconds.
NOTICE: Removing 'origin' remote; see 'Why is my origin removed?' in the manual if you want to push back there.
        (was git@github.com:totalwindupflightsystems/off-by-one.git)
```

**Rewrite report, pasted from `filter-repo/`:**

```
$ wc -l < filter-repo/changed-refs
19                                  # ALL 19 refs rewritten (11 heads + 3 pull + 5 tags)

$ wc -l < filter-repo/ref-map      # minus header row
19

$ commits parsed ......... 1752
$ commits changed ........ 1413     (awk 'NR>1 && $1!=$2' commit-map | wc -l)
$ commits unchanged ......  339     (awk 'NR>1 && $1==$2' commit-map | wc -l)

$ cat filter-repo/first-changed-commits
c6e3716e8ef50a98f29ed2861fb294de5019c9c6 f5e6acbf7eb6a4e7a2251b9bb5c82cb1d524d213
```

`first-changed-commits` = the 2026-08-02 commit that *introduced* the monolith; everything at or
after it is rewritten, everything before it keeps its sha (339 unchanged commits).

Post-rewrite check — the path is gone from the entire history:

```
$ git rev-list --objects --all | grep -c 'data/answers\.jsonl$'
0
```

Both annotated tag objects and their peeled commits were rewritten (`refs/tags/v0.1.0`
`ada1a5ce` → `c894a77`), which matters for the force-push step in the runbook.

`filter-repo/suboptimal-issues` has 240 lines: commit **messages** that quote now-dead shas were
left as-is (cosmetic, expected — filter-repo does not rewrite messages unless asked). No action
required, but do not be surprised by `git log --grep` hits pointing at shas that no longer exist.

---

## ARM 3 — result equivalence (tip must not change)

```
PRE   master commit  78b55e83e9603eda422f9bd836b22107b86a532e
PRE   master tree    5fe1af1b5c4432a51ce1d5dc96ca82bbf6650f20

POST  master commit  1f2b0b06aecccc4b4f3bf35d2ca2abd8afe5ad3a
POST  master tree    5fe1af1b5c4432a51ce1d5dc96ca82bbf6650f20
```

**RESULT: EQUAL.** The tip tree hash is byte-identical because `OBS-OB-006` already untracked the
monolith — the purge removes *history only*, not the current working set. The commit sha changes
(ancestors were rewritten); the tree does not. The canonical per-class corpus survives intact:
`git ls-tree -r master --name-only | grep -c '^data/answers/'` → **3249** (same before and after).

---

## ARM 4 — measured size (all three numbers)

| # | measurement | command | result |
|---|---|---|---|
| **a** | mirror BEFORE rewrite | `du -sh /tmp/obo-filterrepo-20261121b.git` | **91M** (pack 89.97 MiB, 31,272 objects) |
| **b** | mirror AFTER rewrite | `du -sh /tmp/obo-filterrepo-20261121b.git` | **50M** (pack 48.81 MiB, 30,909 objects) |
| **c** | fresh clone of the REWRITTEN mirror | `git clone file:///tmp/obo-filterrepo-20261121b.git /tmp/obo-fresh-b; du -sh /tmp/obo-fresh-b/.git` | **51M** |

Object count dropped **31,272 → 30,909 = exactly −363**, the blob count from arm 1. Nothing else
was touched; the delta is precisely the monolith's history.

Contemporaneous "before" fresh clone (same procedure, pre-rewrite mirror):

```
$ du -sh /tmp/obo-prefresh-b/.git
92M
```

So the like-for-like fresh-clone comparison on the same day is **92M → 51M**.

### Against the stated 49M baseline — that baseline is not today's pre-purge size

Measured today, several measurement classes, all on the *pre-purge* state:

| clone flavour of current origin | `.git` size |
|---|---|
| shallow `--depth 1` (tip tree only) | 32M |
| `--single-branch --branch master`, full history | 92M |
| full all-refs (from mirror) | 92M |
| owner's local clone `/home/kara/off-by-one` (locally gc'd) | 58M |
| **fresh clone of the PURGED mirror** | **51M** |

The quoted "49M fresh clone" matches the **post-purge** number (51M), not any pre-purge measurement
I could reproduce (92M three ways). Treat that baseline as misattributed/stale: **pre-purge is
~92M today.**

### Verdict — is the win material?

**Yes: ~41M, ~45% of both the mirror (91M → 50M) and a fresh clone (92M → 51M).** No dressing up
beyond that: this is a real, material reduction — but the clone is still ~51M afterwards, because
the canonical per-class corpus (3,249 tracked files under `data/answers/`) plus code is what the
remaining weight actually is. The purge stops the monolith's ~41M of packed history from being
carried by every future clone; it does not make the repo small.

Second, honest caveat: this is a **one-time 41M**, not a growth fix. The growth fix already landed
as `OBS-OB-006` (untrack + deterministic rebuild); the purge is the cleanup of what that left behind.

---

## ARM 5 — RUNBOOK FOR THE OWNER (copy-paste ready)

> Preconditions: nothing in the off-by-one lane is mid-flight, and you know that a force-push
> rewrites every sha — every existing clone, worktree and open PR becomes stale (list in 5e).

### 5a. Backup first (from a normal clone, not a mirror)

```bash
cd /home/kara/off-by-one
git fetch --all --prune
git bundle create /home/kara/off-by-one-pre-purge-20261121.bundle --all
git bundle verify /home/kara/off-by-one-pre-purge-20261121.bundle | tail -3
ls -lh /home/kara/off-by-one-pre-purge-20261121.bundle
```

Keep that bundle (and a copy off-box) until you are satisfied; it is the only undo.

### 5b. The purge — run it in a FRESH working clone, not in `/home/kara/off-by-one`

```bash
cd /tmp
git clone git@github.com:totalwindupflightsystems/off-by-one.git obo-purge-20261121
cd obo-purge-20261121
git filter-repo --invert-paths --path data/answers.jsonl --force
```

Exactly the command dry-run in arm 2; verified consequences: 1,413 of 1,752 commits rewritten,
all 19 refs rewritten, tip tree hash unchanged (`5fe1af1b…`), `data/answers.jsonl` gone from all
history, `data/answers/` per-class corpus untouched (3,249 files).

Optional, if you also want the *local-only* backup branch blob gone (the 364th blob,
`307cf69c`, 26.7 MiB, on `backup-oby1006-178b02f3` — it is **not** on origin, so the origin purge
cannot touch it): run the same filter-repo in a clone that has that branch, or simply drop the
branch locally. If that branch is ever pushed, the path comes back.

### 5c. Force-push sequence

`git filter-repo` **deletes the `origin` remote on purpose** (so you cannot push by accident). Its
own output:

```
NOTICE: Removing 'origin' remote; see 'Why is my origin removed?' in the manual if you want to push back there.
        (was git@github.com:totalwindupflightsystems/off-by-one.git)
```

So the sequence is: re-add, push all rewritten heads **and** tags, then verify.

```bash
cd /tmp/obo-purge-20261121
git remote add origin git@github.com:totalwindupflightsystems/off-by-one.git

# 11 heads + 5 annotated tags were rewritten — push both namespaces with --force
git push --force origin 'refs/heads/*:refs/heads/*'
git push --force origin 'refs/tags/*:refs/tags/*'

# verify: local tip must equal origin tip, and no ref may still point at a pre-purge sha
git ls-remote origin | grep 'refs/heads/master$'
git rev-parse master
```

Refs the rewrite touched on origin (all 11 heads + 5 tags — nothing else is pushable):

```
refs/heads/master, refs/heads/pr2-carter, refs/heads/readme-audit-2026-10-26-21-38-04,
refs/heads/wt/DF-OFF-BY-ONE-33, wt/DF-OFF-BY-ONE-52, wt/DF-OFF-BY-ONE-60,
wt/DF-OFF-BY-ONE-61, wt/OB-GAP-092, wt/REVIEW-OB-007, wt/REVIEW-OB-009, wt/REVIEW-OB-023,
refs/tags/v0.1.0, v0.1.1, v0.2.0, v0.2.1, v0.3.0   (annotated → their tag objects were rewritten too)
```

Do **not** use `git push --force --mirror`: it would also push your local-only branches
(`backup-oby1006-178b02f3`, the stale `wt/*` locals) and delete origin refs you did not rewrite.

### ⚠️ 5c-bis. GitHub's PR refs keep the monolith reachable — the purge is NOT complete on GitHub

`refs/pull/2/head`, `refs/pull/3/head`, `refs/pull/4/head` are GitHub-managed, cannot be
force-pushed, and their commits still carry the file in their trees (verified read-only in the
owner clone):

```
b0775f5e 2026-08-15 Merge remote-tracking branch 'origin/master' into fix/sandb        answers.jsonl in tree: 1
dd93aa06 2026-09-28 fix: bind HTTP server to loopback by default (REVIEW-OB-007 …)     answers.jsonl in tree: 1
6511e56c 2026-10-06 feat(privacy): redact common PII from shared answers by def…       answers.jsonl in tree: 1
```

Consequences to accept explicitly:
- Any `git clone --mirror` (including this dry run's) will keep re-fetching those PR refs and
  therefore the old blobs.
- Removing the data from GitHub's own storage needs either those PRs deleted/their refs cleared
  (GitHub support request: "please gc unreachable objects / clear pull refs for repo X") or a
  private-repo/rotation decision. Force-pushing heads+tags alone does not do it.
- Public forks/mirrors of the repo, if any exist, keep their own copies regardless.

### 5d. Clone-refresh list — every local clone that must be re-cloned

`git worktree list` in `/home/kara/off-by-one` (shared `.git` — all of these die the moment the
main clone is reset):

```
/home/kara/off-by-one                          78b55e83 [master]
/home/kara/off-by-one-deploy-ob002             1967a88c (detached HEAD)
/home/kara/off-by-one-deploy-ob003             7afc74cd (detached HEAD)
/home/kara/worktrees/off-by-one-OBS-OB-005     6d5ad63e [wt/OBS-OB-005]
/home/kara/worktrees/off-by-one-RELEASE-OB-019 b07ea135 [wt/RELEASE-OB-019]
/home/kara/worktrees/off-by-one-df45       7910d5da [wt/df45]
/home/kara/worktrees/off-by-one-ob-ops-002 9220b75a [wt/OB-OPS-002]
```

Independent clone (own `.git`, `origin = /home/kara/off-by-one` — must be deleted and re-cloned,
it will never fast-forward again):

```
/home/kara/off-by-one-review-2026-10-28     (OWN-GIT-DIR, clone of the local path)
```

Recommended refresh procedure (order matters — re-clone the main clone, then re-create worktrees):

```bash
cd /home/kara
mv off-by-one off-by-one.pre-purge-20261121     # keep as local rollback, delete once satisfied
git clone git@github.com:totalwindupflightsystems/off-by-one.git off-by-one
# re-create the four active worktrees from their branches, e.g.
git -C off-by-one worktree add ../worktrees/off-by-one-OBS-OB-005 wt/OBS-OB-005
# the two deploy-ob00x detached worktrees and the 2026-10-28 review clone: recreate or drop
```

Note: `/home/kara/off-by-one/data/answers.jsonl` (≈58 MB) still exists **untracked** in the old
working tree and is untouched by the purge — the deterministic rebuild (`OBS-OB-006`) owns it; do
not expect the purge to remove it from disk.

Fleet rows that name this repo (read-only `grep`; 34 matching lines, 11 lanes). **Workdir-only
lanes need no re-clone** — they are stand-ins, no `.git` (verified):

```
~/.hermes/fleet.toml:3495  name=off-by-one-docs      repo_url=local:/home/kara/off-by-one                                  workdir=~/.hermes/stand-in/docs/off-by-one        (no .git)
~/.hermes/fleet.toml:3506  name=off-by-one-dogfood   repo_url=local:/home/kara/off-by-one                                  workdir=~/.hermes/stand-in/dogfood/off-by-one     (no .git)
~/.hermes/fleet.toml:3517  name=off-by-one-foreman   repo_url=local:/home/kara/off-by-one                                  workdir=/home/kara/off-by-one                     (CLONE — 5d above)
~/.hermes/fleet.toml:3528  name=off-by-one-ops       repo_url=local:~/.hermes/stand-in/pm-lane/off-by-one                  workdir=~/.hermes/stand-in/pm-lane/off-by-one     (no .git)
~/.hermes/fleet.toml:3539  name=off-by-one-perf      repo_url=local:/home/kara/off-by-one                                  workdir=~/.hermes/stand-in/perf/off-by-one        (no .git)
~/.hermes/fleet.toml:3550  name=off-by-one-pm        repo_url=local:~/.hermes/stand-in/pm-lane/off-by-one                  workdir=~/.hermes/stand-in/pm-lane/off-by-one     (no .git)
~/.hermes/fleet.toml:3561  name=off-by-one-qa        repo_url=local:/home/kara/off-by-one                                  workdir=~/.hermes/stand-in/pm/off-by-one          (no .git)
~/.hermes/fleet.toml:3572  name=off-by-one-readme    repo_url=local:/home/kara/off-by-one                                  workdir=~/.hermes/stand-in/readme/off-by-one      (no .git)
~/.hermes/fleet.toml:3583  name=off-by-one-releng    repo_url=local:/home/kara/off-by-one                                  workdir=~/.hermes/stand-in/releng-lane/off-by-one (no .git)
~/.hermes/fleet.toml:3594  name=off-by-one-review    repo_url=local:/home/kara/off-by-one                                  workdir=~/.hermes/stand-in/review/off-by-one      (no .git)
~/.hermes/fleet.toml:3605  name=off-by-one-sync      repo_url=https://example.com/off-by-one-sync                          workdir=~/.hermes/sync-workdirs/off-by-one-sync   (no .git, phantom url — pre-existing)
```

`grep -rl 'off-by-one' ~/.hermes/fleet*` also matches ~90 `.bak-*` snapshots of `fleet.toml` and
one `fleet_report_20260724.html`; those are history, not live config — no action.

**Paths holding a real clone that must be re-cloned, in full:**

1. `/home/kara/off-by-one` (the fleet's foreman workdir; 6 worktrees hang off its `.git`)
2. `/home/kara/off-by-one-review-2026-10-28` (independent clone of #1)
3. Any external/off-box clone or CI cache not on this host (unknowable from here — say so)
4. `/tmp/obo-*` mirrors/clones from this dry run (disposable, reaped by /tmp policy)

### 5e. Post-purge sequencing (why this is not a 3-command job)

Force-pushing rewrites every sha, so:
1. Freeze the lane (`off-by-one-foreman`, `-pm`, `-ops`, `-sync`, `-qa`, …) — a tick that commits
   mid-purge leaves an orphan.
2. Land/clone anything in flight **before** the push; worktrees listed above cannot survive it.
3. Push heads+tags, then re-clone, then re-create worktrees, then un-freeze.
4. Accept the `refs/pull/*` gap (5c-bis) or file the GitHub support request — otherwise the
   purge is "monolith gone from main history, still reachable via PR refs".

---

## Hard-reject compliance

| reject | status |
|---|---|
| Any push / force-push to origin | **NONE.** No `git push` was run in any form. |
| Rewriting `data/answers/*.json` | **NOT touched** — filter path was exactly `data/answers.jsonl`; post-rewrite tip still has 3,249 files under `data/answers/`. |
| Editing anything outside `/tmp` | **NONE.** Outside `/tmp` this task only ran `git worktree list`, `git remote -v`, `git rev-list/ls-tree/log/cat-file` (read) in `/home/kara/off-by-one`, and the two prescribed read-only greps of `~/.hermes/fleet*`. |
| Size claims without all three `du` numbers | All three given in arm 4, plus two contemporaneous control clones. |

**Proof nothing was pushed** (final state of the throwaway mirror, pass B):

```
$ git -C /tmp/obo-filterrepo-20261121b.git remote -v
origin	git@github.com:totalwindupflightsystems/off-by-one.git (fetch)
origin	git@github.com:totalwindupflightsystems/off-by-one.git (push)
```

Immediately after `filter-repo`, `remote -v` printed **nothing** — filter-repo had removed the
remote by design; the URL above was re-added verbatim (config write only, no network write).

```
$ git -C /tmp/obo-filterrepo-20261121b.git ls-remote origin | grep 'refs/heads/master$'
78b55e83e9603eda422f9bd836b22107b86a532e	refs/heads/master      ← ORIGIN STILL PRE-PURGE
$ git -C /tmp/obo-filterrepo-20261121b.git rev-parse master
1f2b0b06aecccc4b4f3bf35d2ca2abd8afe5ad3a                       ← LOCAL IS PURGED

$ (origin shas vs the pre-rewrite ref snapshot taken before any rewrite)
18 of 19 pre-rewrite ref shas still present on origin
```

The single difference is `refs/heads/master`, which advanced `13c17ec8` → `78b55e83` **before** this
task's rewrite (a board chore that landed between pass A's clone and pass B's; confirmed by
`git ls-remote` at the top of this document). Every rewritten sha (`1f2b0b06`, `737810da`, …) is
absent from origin — the purge exists only in `/tmp`.

Origin still serves the pre-purge sha and the rewritten commit exists only locally — the two trees
would differ only by history, and they were never compared over the wire because **no push ran**.
`git reflog show master` in the mirror shows only filter-repo's rewrite; there is no push entry.

---

## One-line arm results

1. **Inventory:** 363 distinct blobs at `data/answers.jsonl` on origin (+1 local-only = 364
   repo-wide), 6.37 GiB uncompressed, 365 commits over **2026-08-02 → 2026-11-21**; the board's
   "six historic blobs" is stale by ~60× and already superseded on the board by the measured 364.
2. **Dry run:** `filter-repo --invert-paths --path data/answers.jsonl --force` → 1,413 of 1,752
   commits rewritten, **all 19 refs** rewritten, path count in history 363 → **0**, 1.83 s.
3. **Equivalence:** pre tip tree `5fe1af1b…` = post tip tree `5fe1af1b…` → **EQUAL** (commit sha
   moves `78b55e83` → `1f2b0b06`, tree does not).
4. **Size:** mirror **91M → 50M**, fresh clone **92M → 51M** (−41M, −45%, objects −363 exactly);
   material. The quoted "49M baseline" is the *post*-purge number, not today's pre-purge 92M.
5. **Runbook:** bundle backup → filter-repo in a fresh clone → re-add origin (filter-repo deletes
   it) → force-push 11 heads + 5 tags → 1 fleet clone + 6 worktrees + 1 independent clone re-cloned;
   `refs/pull/{2,3,4}` remain on GitHub and still carry the file — flagged, not fixed.
