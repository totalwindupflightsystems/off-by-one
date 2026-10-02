# Verdict: OB-GAP-092

**Task:** Fix ob1-distribute publish leg after bunker-mvp key rotation
**Evaluated:** 2026-10-01T11:11:50.602034
**Result:** ✗ FAIL

## Pipeline Stages

- ✗ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✗ tests: Command timed out
- ✓ **tier2**
  - COMPLETE
  ✓ The ob1-distribute publish leg to bunker-mvp (root@78.46.173.180) succeeds again: ssh/scp authenticate and a real publish run completes with rc=0 and the catalog service healthy, using the rotated-key path; the ssh config and any scripts updated accordingly.: Live-verified end-to-end. (1) ssh auth: `ssh -o BatchMode=yes root@78.46.173.180 'echo SSH_OK; hostname'` -> 'SSH_OK / bunker-mvp', rc=0. (2) ssh config updated for the rotated key: ~/.ssh/config:154-157 Host bunker-mvp -> IdentityFile ~/.ssh/id_ed25519_bunker_mvp; ~/.ssh/config:217-220 Host 78.46.173.180 (the bare-IP form the publish transport actually connects to) -> IdentityFile ~/.ssh/id_ed25519_bunker_mvp + IdentitiesOnly yes, with an explicit 'OB-GAP-092 2026-10-01' comment at :212-216 recording that id_ed25519_bunker was regenerated 2026-09-30 and is no longer accepted. Keys on disk confirm the rotation (id_ed25519_bunker 09-30 22:29 vs id_ed25519_bunker_mvp 09-30 19:34). (3) Real publish run: `bash scripts/publish-catalog.sh` -> 'staged pair -> root@78.46.173.180:/opt/off-by-one', 'activated on root@78.46.173.180 (systemctl restart off-by-one)', 'public catalog healthy on root@78.46.173.180 (HTTP 200)', PUBLISH_RC=0. (4) Full task subject `bash scripts/ob1-distribute.sh` -> same publish-leg output plus DISTRIBUTE_RC=0. (5) Catalog service healthy: remote `systemctl is-active off-by-one` = active; remote `curl http://127.0.0.1:8766/api/v1/stats` = HTTP 200 with body {"total_problems":2490,"total_answers":4219,"verified_answers":4191,...,"readonly":true}; /opt/off-by-one/off-by-one and off-by-one.db freshly activated. (6) Scripts/docs updated accordingly: docs/publish-transport.md:135 'Deploy-key rotation 2026-09-30 -> fixed 2026-10-01 (OB-GAP-092)' (commit e7ade40d), board rows 332618a6/7e176cfd; scripts/publish-catalog.sh already carried correct transport classification from OB-GAP-065, so the fix was correctly scoped to the local credential rather than transport code. Note: one earlier ob1-distribute.sh run returned rc=141 (SIGPIPE) from the pre-existing `git show --stat --oneline HEAD | head -8` line (introduced in f94a8c66, Sep 18, unrelated to OB-GAP-092) when PART 1 had corpus changes; on a clean tree PART 1 is skipped and the run returns rc=0.
The ob1-distribute publish leg to root@78.46.173.180 is fixed and verified live: ssh/scp authenticate via the rotated id_ed25519_bunker_mvp key path, both publish-catalog.sh and the full ob1-distribute.sh complete with rc=0, and the catalog service is active and serving HTTP 200.

## Summary

Judge Result: OB-GAP-092

Stage tier1: FAIL
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✗ tests: Command timed out

Stage tier2: PASS
  COMPLETE
  ✓ The ob1-distribute publish leg to bunker-mvp (root@78.46.173.180) succeeds again: ssh/scp authenticate and a real publish run completes with rc=0 and the catalog service healthy, using the rotated-key path; the ssh config and any scripts updated accordingly.: Live-verified end-to-end. (1) ssh auth: `ssh -o BatchMode=yes root@78.46.173.180 'echo SSH_OK; hostname'` -> 'SSH_OK / bunker-mvp', rc=0. (2) ssh config updated for the rotated key: ~/.ssh/config:154-157 Host bunker-mvp -> IdentityFile ~/.ssh/id_ed25519_bunker_mvp; ~/.ssh/config:217-220 Host 78.46.173.180 (the bare-IP form the publish transport actually connects to) -> IdentityFile ~/.ssh/id_ed25519_bunker_mvp + IdentitiesOnly yes, with an explicit 'OB-GAP-092 2026-10-01' comment at :212-216 recording that id_ed25519_bunker was regenerated 2026-09-30 and is no longer accepted. Keys on disk confirm the rotation (id_ed25519_bunker 09-30 22:29 vs id_ed25519_bunker_mvp 09-30 19:34). (3) Real publish run: `bash scripts/publish-catalog.sh` -> 'staged pair -> root@78.46.173.180:/opt/off-by-one', 'activated on root@78.46.173.180 (systemctl restart off-by-one)', 'public catalog healthy on root@78.46.173.180 (HTTP 200)', PUBLISH_RC=0. (4) Full task subject `bash scripts/ob1-distribute.sh` -> same publish-leg output plus DISTRIBUTE_RC=0. (5) Catalog service healthy: remote `systemctl is-active off-by-one` = active; remote `curl http://127.0.0.1:8766/api/v1/stats` = HTTP 200 with body {"total_problems":2490,"total_answers":4219,"verified_answers":4191,...,"readonly":true}; /opt/off-by-one/off-by-one and off-by-one.db freshly activated. (6) Scripts/docs updated accordingly: docs/publish-transport.md:135 'Deploy-key rotation 2026-09-30 -> fixed 2026-10-01 (OB-GAP-092)' (commit e7ade40d), board rows 332618a6/7e176cfd; scripts/publish-catalog.sh already carried correct transport classification from OB-GAP-065, so the fix was correctly scoped to the local credential rather than transport code. Note: one earlier ob1-distribute.sh run returned rc=141 (SIGPIPE) from the pre-existing `git show --stat --oneline HEAD | head -8` line (introduced in f94a8c66, Sep 18, unrelated to OB-GAP-092) when PART 1 had corpus changes; on a clean tree PART 1 is skipped and the run returns rc=0.
The ob1-distribute publish leg to root@78.46.173.180 is fixed and verified live: ssh/scp authenticate via the rotated id_ed25519_bunker_mvp key path, both publish-catalog.sh and the full ob1-distribute.sh complete with rc=0, and the catalog service is active and serving HTTP 200.

Overall: FAIL ✗
