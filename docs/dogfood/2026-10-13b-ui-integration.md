# Dogfood Run 2026-10-13b — Web UI Surface (Submit / Explore / Export / Import / Chat)

**Run:** off-by-one-dogfood tick 2026-10-13T05:37Z (sibling run 2026-10-13 (tools angle) landed mid-tick — its log entry is line 178; this run took the UI angle and cites its INSTALL row).
**Verdict:** PROMISING-BUT-ROUGH — the API underneath is excellent (every UI action's backend call succeeded), but the UI layer has a dead drill-down leaf, a success-rendered-as-failure, and a flapping chat panel.
**Promise tested:** "Agents/users can browse the taxonomy, submit problems, and chat with the AI agent through the web UI." (Surfaces never driven in 18 prior dogfood runs.)

## What I did as a real user (browser, live instance on :8766, 4463 answers, v0.1.1-259-gf851ecbd build)

1. Home view loads, status bar real numbers (Queue 0, Cache 4463, hit rate 99.4%). Theme boot script, 0 long tasks. favicon.ico 404 (cosmetic).
2. **Submit view:** filled the form with a new problem class (df-ui-golang-http-client-timeout-retry-budget-20261013) and clicked Submit. Server responded 409 with the dedup contract body (`submission_id: sub_e3894e, status: deduplicated, position: 1`) — the submission LANDED — but the UI printed **"Submission failed (HTTP 409)"** and kept the form filled. A user reads "failed" for a queued submission. → DF-OFF-BY-ONE-42.
3. **Explore view:** taxonomy renders 1000 classes (see DF-21 for the 1000-cap); drill-down class→env→lang→version works at every level… until the version node: expanding it toggles ▼ and shows **nothing** — the answer leaf never renders. Root cause read from the served JS: `toggleSubNode` calls `renderVersionChildren(children, classTitle, parentValue, answers)` but the function is `renderVersionChildren(container, answers)` — an argument-shift; `answers.forEach` on undefined dies inside the click handler (silently swallowed). The API payload is correct; the render layer kills the advertised path "…version → answer". → DF-OFF-BY-ONE-41.
4. **Export view:** answer list renders with VERIFIED badges and select-all; not driven further (export dir not configured on the live instance; the 501 path was exercised by run 2026-10-10b on scratch nodes).
5. **Import view:** entered a repo URL, Preview Import → 501 not_configured — and the UI renders the server's remediation message clearly ("set --import-dir or OFF_BY_ONE_IMPORT_DIR…"). This surface is honest and good.
6. **AI chat:** 11 "Connected to Off-by-One AI Agent" banners accumulated in ~3 minutes (client reconnect loop; a raw-socket probe completed the server upgrade in 0.00s and got exactly ONE banner, so the flap is client-side). Sending a message left NO transcript entry for my own text and no reply within 25s. → DF-OFF-BY-ONE-43.

## Perf (Step 2b)

Nothing fileable: POST /api/v1/problems/discover on the fresh bunker install ~2.0-2.4ms; view switches ~1s (includes data fetch); WS upgrade 0.0s server-side; 0 long tasks. The UI defects above are correctness/UX, not speed. Discover cold+warm numbers re-confirm the 10-10 runs (1.4-16ms) on a different host.

## Install leg

Sibling run (log line 178, INSTALL-OFF-BY-ONE-2026-10-13) covered today's leg on bunker-mvp (clone+build 84s, seed 23s, smoke ok). This run independently re-proved the same premise on its own agent oby-df1013: clone+build 80s cold, seed ok, health ok, discover found:true (answer #2679) — consistent, different agent, host verified from `bunker status` not from the flag. Agent destroyed (one destroy refusal: my backgrounded smoke server survived the ssh timeout and blocked destroy — killed, retried, list clean after 90s). Residue note: one phantom agent `oby-df2026` was listed-running but `info` said not_found at tick start; after my destroy the list is empty — reconciler swept it, no residue survived.

## Value judgment

- Does it work? API: yes, everything. UI: three real defects (41/42/43).
- Useful? Yes — the import view's error surfacing is exemplary.
- Usable? Explore's dead leaf and the false "Submission failed" are exactly what a first-time user hits.
- Trustworthy? Data intact; error states honest at the API; UI mislabels one success.
- Fast enough? Comfortably (numbers above).
