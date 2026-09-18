# Public catalog publish transport

Operator guide for the host publish leg of the off-by-one distribution — the
part that ships the lab binary and an answer-DB snapshot to the public catalog
host. Owner: whoever holds the deploy key for that host.

| Piece | Path |
|---|---|
| Cron entrypoint (canonical copy) | `scripts/ob1-distribute.sh` |
| Host publish leg | `scripts/publish-catalog.sh` |
| Transport retry library | `scripts/lib/transport-retry.sh` |
| Regression self-test | `scripts/tests/transport-retry-selftest.sh` (`make transport-retry-selftest`) |

The cron entrypoint is a deployed **untracked** copy at
`~/.hermes/scripts/ob1-distribute.sh`. Re-deploy it after changing the tracked
copy:

```bash
cp scripts/ob1-distribute.sh ~/.hermes/scripts/ob1-distribute.sh
chmod +x ~/.hermes/scripts/ob1-distribute.sh
```

## What the leg does

1. `sqlite3 <db> ".backup …"` — consistent snapshot into a temp file.
2. **Stage** (retried as a unit, because both writes are idempotent and use
   fixed names): `scp` binary → `<remote>/off-by-one.new`, `scp` snapshot →
   `<remote>/off-by-one.db.new`.
3. **Activate** (attempted exactly once): one `ssh … bash -s` body that
   *refuses to run at all* unless **both** staged files exist and are non-empty,
   then `chmod +x`, `mv` both into place, `systemctl restart <service>`.
4. **Health** (read-only, retried on the transport class):
   `ssh … curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:<port><path>`
   must return `200`.

The fixed staging names plus the both-files guard are what make a partial or
stale artifact pair impossible to activate: a truncated transfer can only ever
leave `.new` files behind, and an incomplete pair is refused (exit 6) rather
than promoted.

## Operator configuration

| Variable | Default | Meaning |
|---|---|---|
| `OB1_PUBLISH_MODE` | `host` | `host` = publish to the box; `static` = explicit opt-out (see below) |
| `OB1_PUBLISH_BOX` | `root@78.46.173.180` | `user@host` of the catalog host |
| `OB1_REMOTE_DIR` | `/opt/off-by-one` | absolute remote directory holding the artifacts |
| `OB1_SERVICE` | `off-by-one` | systemd unit restarted after activation |
| `OB1_HEALTH_PORT` | `8766` | port of the lab HTTP API on the host |
| `OB1_HEALTH_PATH` | `/api/v1/stats` | health path (must return HTTP 200) |
| `OB1_HEALTH_SETTLE_SECONDS` | `2` | pause between restart and health probe |
| `OB1_BINARY` / `OB1_DB` | `<repo>/off-by-one`, `<repo>/off-by-one.db` | local artifacts to ship |
| `OB1_SQLITE` | `sqlite3` | sqlite binary used for the snapshot |
| `OB1_PUBLISH_DRY_RUN` | `0` | `1` = print the resolved plan, touch nothing |
| `TRANSPORT_RETRIES` | `3` | retries **after** the first attempt (bound = 4 attempts) |
| `TRANSPORT_BACKOFF_BASE` | `2` | first backoff seconds, doubling |
| `TRANSPORT_BACKOFF_MAX` | `8` | backoff ceiling seconds |

`OB1_PUBLISH_BOX`, `OB1_REMOTE_DIR` and `OB1_SERVICE` are validated (no quotes
or whitespace; remote dir must be absolute) before anything is executed,
because they are interpolated into the remote activation body. A rejected value
exits 5 without contacting the host.

### Check the resolved plan without touching the host

```bash
OB1_PUBLISH_DRY_RUN=1 bash scripts/publish-catalog.sh
```

## Failure semantics

| Exit | Meaning | Retried? |
|---|---|---|
| 0 | published and healthy (or `static` / dry-run) | — |
| 1 | local prep failure, or a non-transport remote failure (e.g. `No space left on device`) | no — one attempt, rc preserved |
| 2 | transport class exhausted the retry budget | yes, bounded, then fails |
| 3 | permanent class (auth denied / host-key mismatch) — **operator action** | no |
| 4 | activated, but the health probe did not return 200 | no (probe itself retries only transport resets) |
| 5 | configuration error | no |
| 6 | activation refused: staged pair incomplete or empty | no |

Retry classification (`scripts/lib/transport-retry.sh`):

| Class | Trigger | Retryable |
|---|---|---|
| `TRANSPORT_RESET` | `ssh`/`scp` rc **255** plus a transient diagnostic (`Connection reset by peer`, `scp: Connection closed`, `Connection timed out`, `Connection refused`, `kex_exchange_identification`, `Broken pipe`, …), or a bare rc 255 with no recognizable diagnostic | **yes** — the whole idempotent staged pair is re-run |
| `TRANSPORT_PERMANENT` | rc 255 plus a fatal diagnostic (`Permission denied`, `Host key verification failed`, `REMOTE HOST IDENTIFICATION HAS CHANGED`, `Too many authentication failures`) | no — repeating it just delays the same answer |
| `NON_TRANSPORT` | any rc **other than** 255 (the remote command's own status: 1, 6, …) | no |

Both the exhausted-budget and the non-transport failures report the class, the
attempt count and the budget in a single verdict line
(`TRANSPORT_RETRY_VERDICT`), and the script always returns the **last** rc — no
false success. Activation is deliberately **not** wrapped in the retry helper:
`mv` + `systemctl restart` is not idempotent, and the remote body's both-files
guard is what protects the live pair instead.

## The static / GitHub half

`OB1_PUBLISH_MODE=static` is an explicit operator declaration that the host
publish is retired or unavailable. It performs **no** ssh/scp, exits 0, and
prints exactly which artifacts are **not** published — it never redirects a
binary/DB publish to another host or domain. In that mode the catalog is served
only by the git half (`data/` corpus → GitHub + `site/` static tree refreshed by
`scripts/sync-answers.sh`).

Note that the answer-corpus half of `scripts/ob1-distribute.sh` (PART 1) is
unchanged and independent: it regenerates the corpus, commits it locally and
pushes to GitHub, and keeps working even when the host leg fails.

## State of the legacy deploy host (as of 2026-09-18, verified live)

This change does **not** repair the legacy deploy host; it makes the leg
truthful and retry-safe around it. Measured live from this host:

| Probe | Result | Class | Behaviour now |
|---|---|---|---|
| `scp` of a scratch artifact to `root@78.46.173.180:/opt/off-by-one/*.new` | `scp: Connection closed`, rc 255 | `TRANSPORT_RESET` | retried within the bound (3 retries / 4 attempts, doubling backoff), then exit **2** with the class and attempt count; the staged pair is left untouched and activation is skipped |
| `ssh -o BatchMode=yes -o ConnectTimeout=8 root@78.46.173.180` | `Permission denied (publickey,password)`, rc 255 | `TRANSPORT_PERMANENT` | exit **3**, one attempt, no pointless retries, operator recipe printed |

The six consecutive cron failures (2026-09-16T23:00 → 2026-09-18T05:01 local,
cron `0741a5de4cba`) all died on the first signature — which is now retried — so
if the close was a transient hop the next fire recovers on its own, and if it is
not, the run fails with attribution instead of a bare `❌ scp binary failed`.

Related public-surface facts (also measured live):

* `ob1.it` no longer fronts the lab — `178.23.13.163` redirects to
  `https://www.nets-sr.com/` (HTTP 200).
* `ob1.it.com` (Cloudflare) serves the static `site/` tree, whose
  `sitemap.xml` `lastmod` is still `2026-08-18` — the public catalog content is
  stale until the host publish works again or the static tree is refreshed.
* GitHub Pages (`master:/docs`, status `built`) is a separate landing surface;
  it is not the catalog host.

**Owner action required:** confirm the real deploy target (is the box rebuilt or
renamed? does this host hold a deploy key?) and then either

* point the leg at it: `OB1_PUBLISH_BOX=<user@host>` (plus `OB1_REMOTE_DIR` /
  `OB1_SERVICE` if they differ), or
* retire the host publish explicitly: `OB1_PUBLISH_MODE=static`.

Neither DNS nor the legacy host can be fixed from inside this repository.

## Verification

```bash
make transport-retry-selftest     # 108 checks, PATH shims + temp dirs, no network
```

The self-test covers: one reset recovers; a persistent reset fails after the
bound with class + attempt accounting; rc 1 and rc 6 are attempted exactly once
and keep their rc; a clean run is one attempt; the classification table; a
**NEUTER negative control** (the single-line lever in a copy of the library is
flipped and the retry stops happening, proving the retry is conditional); the
publish flow's staged-pair guard and single activation; static mode performing
zero ssh/scp calls; and configuration errors failing before any ssh.
