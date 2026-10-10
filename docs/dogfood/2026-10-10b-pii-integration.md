# Dogfood Integration Report — PII Redaction Round-Trip (run #15, 2026-10-10b)

**Angle:** first consumer exercise of the privacy-by-default redaction
(8d3d7924, 2026-10-06). Prior runs proved install, solve, import/export,
Muster-consumer and public-deployment surfaces — none had fed PII through
the sharing engine. The sibling run the same evening (#14,
2026-10-10-integration.md) took the flagship-solve angle; the two are
complementary.

**Promise under test (README/data):** "Answers are scrubbed before
publication … Personal home roots become `~`; email addresses, phone
numbers, and IPv4/IPv6 literals become `<email>`, `<phone>`, and
`<ip-address>`. The same policy is enforced on the read-only public API and
by CI against the generated `data/` and `site/` artifacts."

## The round-trip (3 scratch nodes, real sharing shape)

1. **Peer repo (contaminated):** built a community-answers git repo whose
   solution.md / evidence.md / signatures.json deliberately carry a private
   IPv4 (192.168.1.42), a home path (/home/kara/...), two email addresses,
   a phone (+1 555 867 5309), an IPv6 with zone (fe80::1%eth0), a ULA
   (10.0.0.7), and a **user@host identifier** (kara@builder-machine).
2. **Consumer node import:** `POST /api/v1/import` from the local repo —
   `added:1`, answer stored verbatim (by design: local graph keeps truth).
3. **Export:** `POST /api/v1/export` to a bare target repo — commit
   c936310, `files_changed:3`, pushed. Exported files carry `<ip-address>`,
   `~`, `<email>`, `<phone>`; **`kara@builder-machine` passed through
   verbatim** (→ DF-OFF-BY-ONE-33).
4. **Third-node import + discover:** `added:1`; discover returns the
   scrubbed text; zero raw-PII hits, placeholders present.
5. **Read-only node (`-readonly`):** discover responses pass
   SanitizePublicJSON — same scrub, same user@host leak. Mutating endpoints
   correctly reject with `read_only` error.
6. **Contributor check:** `scripts/check-public-pii.py` on the repo —
   "public PII hygiene OK", rc=0 (it cannot see the dot-less form either;
   it mirrors the email regex).
7. **Published surfaces:** grep over `data/answers.jsonl`, `data/answers/`,
   `site/`: no personal emails / home paths / routable IPs. But the corpus
   DOES contain internal org identifiers (see DF-OFF-BY-ONE-34).

## What held up

- Home-path, email, phone, IPv4, IPv6+zone redaction: correct at every
  surface (export subtree, public API JSON, readonly API, corpus, site).
- Export → import → discover composes cleanly across three independent
  nodes; re-import is idempotent (`skipped:1`, 76ms).
- `-readonly` mode genuinely blocks mutation and sanitizes every response.

## What failed (rows filed)

- **DF-OFF-BY-ONE-33 (P1):** user@host identifiers survive every surface.
  The email regex requires a dotted domain.
- **DF-OFF-BY-ONE-34 (P2):** internal org names (wojons/*, get-h3, dexdat,
  9router, task-router) are present in the published corpus/site while
  sanitizeInternalNames's doctrine says they must never be.
- **DF-OFF-BY-ONE-35 (P3):** export to a non-bare target repo 500s with
  git's raw push error; docs never state the bare-repo requirement.

## Friction log

- `message` is not a field on export requests (`commit_message` is) — my
  mistake, but the 400 correctly named the unknown field.
- The export working dir is a **persistent single clone**: a second export
  to a different `target_repo` reuses the first clone (origin mismatch
  guard returns ErrRepoMismatch — good fail-closed), but the operator must
  know the dir is one-clone-per-server. Docs don't say.

## Perf (Step 2b)

discover (scratch node, hyperfine-grade 10×): 0.9–1.6ms warm; cold
first-call 16ms; idempotent re-import 76ms; seed 2577 files 10s on the
bunker. Nothing a user would feel → **no PERF row** (per coding-hermes-perf
closing rule).

## Install leg (bunker)

las-bunker-03 offline (ssh timeouts; 5+ days per tailscale). las-bunker-02
reachable, bunkerd active. Spawn transiently failed once
(`slice-limits: containment landing did not converge`) — retry per skill
succeeded. Agent 3655aaa9 (TTL 2h): clone HEAD 02471898 60s → `go build`
54s (toolchain go1.26.0 auto-download) → `seed` 10s (2577 classes / 2673
answers / 1531 skipped / 11356 edges) → serve health ok → discover
`found:true`. Agent destroyed, list clean. Zero sudo, zero preinstalled
project deps beyond the Go toolchain the docs already require.
