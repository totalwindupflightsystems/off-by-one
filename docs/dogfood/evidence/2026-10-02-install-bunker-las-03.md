# Install-proof evidence: bunker-las-03 fresh-agent v0.1.1 release-binary install (2026-10-02)

Tick: off-by-one-foreman-2026-10-02-10-44-23 · closes SKIPPED-install-bunker-OB-2026-09-25.

Method: local `bunker` CLI → spawn a fresh agent on server bunker-las-03, run the GitHub
Releases install path inside it, capture raw outputs, then destroy the agent. This is a
live remote-host proof — the verification is the transcript below, captured verbatim from
the session commands (no code change accompanies this tick; there is no test to run).

Note: the 2026-10-02 dogfood report's "Install Leg — SKIPPED" line is the dogfood
session's own leg (it reused the already-running local service). THIS document is a
separate, independent install proof by the foreman tick the same day.

## 1. Host reachable (the 09-25 row's blocker)

    $ ssh -o IdentitiesOnly=yes bunker3 'whoami; uname -m'
    kara
    x86_64

(bunker3 = 100.69.3.13, las-bunker-03; kara pubkey authorized — the 10-01 PM note
"auth fails only" was a stale read.)

## 2. bunkerd server daemon healthy

    $ ssh bunker3 '/opt/bunker/bunkerd version'
    bunkerd 0.1.4
      commit:     6a6ad20
      built:      2026-09-20T07:17:29Z

## 3. Fresh agent spawn

    $ bunker spawn oby-install-test --server bunker-las-03 --ttl 2h
    Agent created: oby-install-test
      SSH Key:      /home/kara/.bunker/keys/oby-install-test
      Port Range:   30500-30599
      Expires:      2026-10-02T05:49:27-07:00

## 4. Agent alive

    $ bunker exec oby-install-test --server bunker-las-03 -- 'echo alive; uname -m; curl --version | head -1'
    alive
    x86_64
    curl 8.14.1 (x86_64-pc-linux-gnu) libcurl/8.14.1 OpenSSL/3.5.7 zlib/8.1.1 ...

## 5. Release download + SHA256SUMS verify + version stamp

    $ bunker exec oby-install-test -- 'sh -c "mkdir -p ~/off-by-one && cd ~/off-by-one \
        && curl -sfLO https://github.com/totalwindupflightsystems/off-by-one/releases/download/v0.1.1/off-by-one-v0.1.1-linux-amd64 \
        && curl -sfLO https://github.com/totalwindupflightsystems/off-by-one/releases/download/v0.1.1/SHA256SUMS \
        && chmod +x off-by-one-v0.1.1-linux-amd64 && sha256sum -c --ignore-missing SHA256SUMS \
        && ./off-by-one-v0.1.1-linux-amd64 --version"'
    off-by-one-v0.1.1-linux-amd64: OK
    off-by-one v0.1.1

## 6. Serve + /health

    $ bunker exec oby-install-test -- 'sh -c "cd ~/off-by-one \
        && nohup ./off-by-one-v0.1.1-linux-amd64 --port 8766 -load-threshold -1 > serve.log 2>&1 & \
        sleep 2; curl -s http://localhost:8766/health"'
    {"status":"ok","uptime":"1s"}

## 7. Empty-node stats + discover (pre-seed)

    {"total_problems":0,"total_answers":0,"verified_answers":0,"queue_depth":0,"hit_rate":0,...}
    POST /api/v1/problems/discover {"problem_class":"so-nil-pointer-deref"}
    → {"error":"not_found","message":"problem class not found"}   (empty node: correct)

## 8. Corpus seed (Quick Start extension — data/ must be deployed; bare release asset has no answers/)

`bunker deploy` (scp -r) of data/ failed `scp -r: exit status 1` on this agent (58 MB dir);
tar + `bunker cp` works:

    $ bunker exec oby-install-test -- 'sh -c "cd ~/off-by-one && tar xzf /tmp/oby-data.tgz \
        && ./off-by-one-v0.1.1-linux-amd64 seed ... "'
    2026/10/02 03:52:20 seed complete: files=2433; classes=2433 created / 0 existing; answers=2528 created / 1531 skipped; edges=10602 created (db=./off-by-one.db)

    curl /api/v1/stats →
    {"total_problems":2433,"total_answers":2528,"verified_answers":2528,"queue_depth":0,"hit_rate":1,"coverage":1.0390464447184546,...}
    POST /api/v1/problems/discover {"problem_class":"so-nil-pointer-deref"}
    → {"found":true,"answer":{"id":57,"problem_class":"so-nil-pointer-deref","env":"linux","lang":"go",...,"solution":"The solution is complete and fully verified..."}}

## 9. Teardown

    $ bunker destroy oby-install-test --server bunker-las-03
    Agent oby-install-test destroyed.
    Removed local SSH key /home/kara/.bunker/keys/oby-install-test

## Verdict

All criterion steps verified live: download → sha256sum -c OK → --version v0.1.1 →
/health ok → discover 200 (found:true after seed). Installability on bunker-las-03
is PROVEN. Known operational note for future install legs: the release asset ships no
corpus (seed needs data/ deployed separately; `bunker deploy` scp -r failed, tar+cp
route proven).
