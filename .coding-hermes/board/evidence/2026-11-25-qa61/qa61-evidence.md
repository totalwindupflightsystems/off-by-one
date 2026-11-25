# QA-OFF-BY-ONE-61 evidence — bunker-qa harness/classifier fork reconciliation

Commit: d80d419 on coding-hermes-tools master; push verified: origin/master..HEAD count = 0
Commit subject byte-verified against the acceptance string; trailer `Co-authored-by: Alexis Okuwa <wojonstech@gmail.com>` present in `git log -1 --format=%B` (appended manually; hooksPath unset on this host).

## Final sha256 (first 12 chars) — deployed MUST equal repo

| surface | sha256[0:12] |
|---|---|
| repo-harness | d3a4432d80ea |
| dep-harness | d3a4432d80ea |
| repo-cls | 4aae22a4b40f |
| dep-cls | 4aae22a4b40f |

pre-reconcile deployed copies (backed up in /tmp/qa61-backup): bunker-qa.sh b8fe33f4840c (6405 lines), classifier 8f8ec71bbe31 (496 lines).

## Copy parity

- `diff /home/kara/coding-hermes-tools/bunker-qa.sh /home/kara/.hermes/scripts/bunker-qa.sh` -> `rc=0` (empty output + rc=0 = byte-identical)
- `diff /home/kara/coding-hermes-tools/test_bunker_qa_classifier.sh /home/kara/.hermes/scripts/test_bunker_qa_classifier.sh` -> `rc=0` (empty output + rc=0 = byte-identical)

## Suite runs

### Repo suite (cd /home/kara/coding-hermes-tools && bash test_bunker_qa_classifier.sh)

```
ok   cap applied + rc=0 -> PASS
  ok   cap NOT applied + rc=0 would be vacuous PASS before -> UNVERIFIED
  ok   chaos-resource vacuous gate present (before PASS grading) -> yes
  ok   mirror: 3-of-5 setup-step race matches -> match
  ok   mirror: genuine product failure does NOT match -> refuse
  ok   mirror: checkout-token gap does NOT match -> refuse
  ok   mirror: signature without ❌ does NOT match -> refuse
  ok   mirror: empty log does NOT match -> refuse
  ok   behavioral: rendered predicate grades race/product/token shapes out of the REAL harness -> yes
  ok   generated arm: ci-pass AND ci-act both name the setup-go toolcache race -> yes
  ok   generated arm: ci-pass UNVERIFIED naming cause + hosted-CI pointer -> yes
  ok   generated arm: setup-race elif sits before the FAIL fallthrough -> yes
PASS test_bunker_qa_classifier.sh (harness=/home/kara/coding-hermes-tools/bunker-qa.sh)
RESULT: 80 passed, 0 failed
```
exit 0; result: 80 passed, 0 failed; zero FAIL lines; all 9 toolcache-race arms green (listed above).

### Deployed suite (cd /home/kara/.hermes/scripts && bash test_bunker_qa_classifier.sh), run AFTER deploy

```
ok   cap applied + rc=0 -> PASS
  ok   cap NOT applied + rc=0 would be vacuous PASS before -> UNVERIFIED
  ok   chaos-resource vacuous gate present (before PASS grading) -> yes
  ok   mirror: 3-of-5 setup-step race matches -> match
  ok   mirror: genuine product failure does NOT match -> refuse
  ok   mirror: checkout-token gap does NOT match -> refuse
  ok   mirror: signature without ❌ does NOT match -> refuse
  ok   mirror: empty log does NOT match -> refuse
  ok   behavioral: rendered predicate grades race/product/token shapes out of the REAL harness -> yes
  ok   generated arm: ci-pass AND ci-act both name the setup-go toolcache race -> yes
  ok   generated arm: ci-pass UNVERIFIED naming cause + hosted-CI pointer -> yes
  ok   generated arm: setup-race elif sits before the FAIL fallthrough -> yes
PASS test_bunker_qa_classifier.sh (harness=/home/kara/.hermes/scripts/bunker-qa.sh)
RESULT: 80 passed, 0 failed
```
exit 0; result: 80 passed, 0 failed; zero FAIL lines; all 9 toolcache-race arms green.
Full tails of both runs are in /tmp/qa61-work/repo-suite-2.txt and /tmp/qa61-work/deployed-suite.txt.

## Criterion-4 grep proofs (final harness — repo copy == deployed copy)

```
+ setup_toolcache_race
4

+ DOCKER_HOST export
2041:        export DOCKER_HOST="unix://\$qa_cand"
3408:    export DOCKER_HOST="unix:///run/bunker/$agent/docker.sock"
3631:[ -x ~/bin/docker ] && export DOCKER_HOST="unix:///run/bunker/$agent/docker.sock"

+ QUARANTINE root preflight
1147:    echo "QUARANTINE-DIR-UNWRITABLE root=$root rc=$rc — refusing pre-spawn: orphan evidence would be destroyed at reap time (QA-HERMES-CANOPY-56)" >&2
1176:  # QUARANTINE-DIR-UNWRITABLE line lands in the FAIL row and the stderr echo.

+ QA-62 comment block
2

+ fork reconcile 311dd8f / ssh-port d69a46b / root-not-Dockerfile-dir d54a732 / manifest digest c219542
0

+ vacuous install/disconnect rejection 9e04051 (commit ref)
0

+ go toolchain floor logic
1

+ tests_fail_detail ported from deployed
2

```

## Disposition of the six failing deployed assertions (criterion 4)

Baseline (pre-reconcile deployed suite): 68 passed, 6 failed — the six verbatim FAILs below.

| # | deployed assertion | disposition | how resolved |
|---|---|---|---|
| 1 | chaos-disconnect vacuous/0-byte gate present | (b) | grep re-spelled to the merged harness's actual gate: `[ ! -s "\$LOGD/disc.log" ]; then` + `disconnect_log_valid` predicate (the deployed `vacuous_native_suite \$LOGD/disc.log` spelling never existed in the merged generation); `# QA-OFF-BY-ONE-61` comment above; now passes on the rendered harness greps |
| 2 | chaos-disconnect 0-byte UNVERIFIED branch present | (b) | grep re-spelled to the merged arm text `cell chaos-disconnect UNVERIFIED "disc.log is EMPTY` (0-byte arm exists, different wording than deployed `(0 bytes)`); owning comment above |
| 3 | behavioral: could extract the disc chain | (b)+(c) | block re-anchored to the merged grading ladder: end marker re-spelled to the merged `OK "chaos-disconnect tests: fails fast` arm, the REAL `disconnect_suite_counts`/`disconnect_log_valid` predicates extracted from bunker-qa.sh and driven with empty/marker-only/populated fixtures (5 behavioral arms; marker-only arm proves the lockstep gate); equivalent rendered-chain coverage also lives in the merged tracked battery (grade_disc arms); owning comment above |
| 4 | streamer wired at both call sites | (b) | grep re-pinned to the merged call-site spelling `ship_prev_tag_tree "$repo" "$QA_PROJ_SLUG"` (2 hit sites, same streamer function); owning comment above |
| 5 | discovery prefers long-running commands | (b) | merged harness discovers BIN on the OBSERVABLE ladder (QA-9ROUTER-22 / QA-OFF-BY-ONE-15-R2) — real long-running start commands beat a bare index.js guess, module-named product main wins the cmd/ ladder; the deployed `(serve|server|daemon|start)` probe preference never landed in this generation; assertion pins the merged shape (kept the original check name) + added the module-main check; the rendered BIN-ladder arms drive it behaviorally; owning comment above |
| 6 | generated battery: run is a second-pass fallback | (b) | the deployed generation's EP_SUB second-pass fallback never landed in this generation; the merged render picks BIN via the observable ladder and the errorpath cell probes exactly that pinned command — case pattern re-pinned to `ep_probe "$BIN" "$EP_DIR" "$LOGD"` in the rendered battery; renamed `generated battery: probe runs the pinned BIN directly` with owning comment |

Check-name census: every deployed check name preserved at equal-or-greater multiplicity except the resolved ones above (documented in /tmp/qa61-work/census.py output); no lockstep assertion deleted silently; RESULT pass gate unchanged.

