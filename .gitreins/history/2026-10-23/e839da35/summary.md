# Verdict: PERF-OFF-BY-ONE-1

**Task:** Batch seed writes to remove per-statement fsync
**Evaluated:** 2026-10-23T18:59:27.848069
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ Fresh ext4 seed completes in <=10s, fsync count <100, output and idempotency unchanged, and internal/seed plus internal/graph tests pass.: Measured live on ext4 (/dev/nvme0n1p2, `df -T` confirms ext4): `strace -f -c -e trace=fsync,fdatasync env OFF_BY_ONE_DB=/tmp/fresh1.db /tmp/ob1 seed` => WALL=7.28 s (<=10s) and fsync calls=29 (<100), exit 0. Output unchanged: 'files=2860; classes=2860 created / 0 existing; answers=2959 created / 1530 skipped; edges=12890 created'. Idempotent re-run: 'classes=0 created / 2860 existing; answers=0 created / 4489 skipped; edges=0 created' (0.67s), DB counts problem_classes=2860, answer_nodes=2959, problem_edges=12890. Batching implemented in internal/graph/batch.go (Store.BeginTx/Tx.Commit single transaction) and internal/seed/seed.go:107-156 (phase 1 parse outside tx, phase 2 all writes in one graph.Tx, deferred Rollback, linker after commit). Tests: `go build ./...` exit 0; `go test ./internal/seed/... ./internal/graph/... -count=1 -p 1 -timeout 300s` => 'ok internal/seed 7.749s' and 'ok internal/graph 4.497s' (exit 0); verbose run shows all TestSeed*/TestTx* PASS including TestSeedIdempotentRerun, TestSeedDeterministicAcrossDatabases, TestSeedAtomicRollbackOnWriteError, TestTxCommitPersistsAndSeesOwnWrites.
Batched seed verified live on ext4: 7.28s wall and 29 fsyncs (was ~45-70s / ~9,100), identical output and idempotent re-run, with internal/seed and internal/graph tests passing.

## Summary

Judge Result: PERF-OFF-BY-ONE-1

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ Fresh ext4 seed completes in <=10s, fsync count <100, output and idempotency unchanged, and internal/seed plus internal/graph tests pass.: Measured live on ext4 (/dev/nvme0n1p2, `df -T` confirms ext4): `strace -f -c -e trace=fsync,fdatasync env OFF_BY_ONE_DB=/tmp/fresh1.db /tmp/ob1 seed` => WALL=7.28 s (<=10s) and fsync calls=29 (<100), exit 0. Output unchanged: 'files=2860; classes=2860 created / 0 existing; answers=2959 created / 1530 skipped; edges=12890 created'. Idempotent re-run: 'classes=0 created / 2860 existing; answers=0 created / 4489 skipped; edges=0 created' (0.67s), DB counts problem_classes=2860, answer_nodes=2959, problem_edges=12890. Batching implemented in internal/graph/batch.go (Store.BeginTx/Tx.Commit single transaction) and internal/seed/seed.go:107-156 (phase 1 parse outside tx, phase 2 all writes in one graph.Tx, deferred Rollback, linker after commit). Tests: `go build ./...` exit 0; `go test ./internal/seed/... ./internal/graph/... -count=1 -p 1 -timeout 300s` => 'ok internal/seed 7.749s' and 'ok internal/graph 4.497s' (exit 0); verbose run shows all TestSeed*/TestTx* PASS including TestSeedIdempotentRerun, TestSeedDeterministicAcrossDatabases, TestSeedAtomicRollbackOnWriteError, TestTxCommitPersistsAndSeesOwnWrites.
Batched seed verified live on ext4: 7.28s wall and 29 fsyncs (was ~45-70s / ~9,100), identical output and idempotent re-run, with internal/seed and internal/graph tests passing.

Overall: PASS ✓
