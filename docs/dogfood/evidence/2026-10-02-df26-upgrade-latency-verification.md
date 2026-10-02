# DF-OFF-BY-ONE-26 — verification: 25s "WebSocket upgrade latency" is NOT a defect

Tick: off-by-one-foreman-2026-10-02-21-10-30 · Verification-only close (disproven premise)

## Claim under test (board row)

"WebSocket upgrade takes 25s (user-noticeable latency)" — dogfood 2026-10-02 measured
`curl` to `/ws/chat` with upgrade headers at 25.0s before HTTP 101, hypothesizing a
handshake-path delay in `internal/web/chat.go`.

## Reproduction (live :8766, tick time)

3/3 attempts: HTTP 101 at elapsed = 25.03s / 25.02s / 25.04s — deterministic fixed
timer, not load.

Decomposition via curl timing on one attempt:

    ttfb=0.000444 total=25.010539 code=101

The 101 Switching Protocols response (the upgrade itself) arrives in 0.44ms.
The remaining 25s is the client-held connection lifetime, i.e. when the SERVER
closes the probe connection.

## Root cause of the 25s constant

`internal/web/chat.go`: ping loop interval = `readTimeout/2` = 15s; ping wait
deadline = `readTimeout/3` = 10s; 15 + 10 = 25s. The probe (`curl` with upgrade
headers) never completes the RFC 6455 handshake, so it never answers the server's
keepalive ping. The server correctly tears the dead probe down after the pong
deadline. Journal confirms, timestamps match the probe attempts:

    chat: client not responding to ping: failed to ping:
      failed to wait for pong: context deadline exceeded

## Verdict

The dogfood probe misread connection CLOSE as slow upgrade. An RFC 6455-compliant
client (any browser; the web SPA) auto-pongs, and upgrade latency is sub-millisecond
(TTFB 0.44ms measured). No code change. The ping-timeout close of non-compliant
probe clients is correct liveness behavior (DF-OFF-BY-ONE-2 fix, e1f8c18).

ch:trace row=DF-OFF-BY-ONE-26 evidence=docs/dogfood/evidence/2026-10-02-df26-upgrade-latency-verification.md witness=live:localhost:8766/ws/chat verdict=not-a-defect (verification-only close; guard full-mode PASS 2026-10-02T21:33Z, CI 3/3 success)
