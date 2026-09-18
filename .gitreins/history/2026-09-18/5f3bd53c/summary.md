# Verdict: OB-GAP-073

**Task:** Optimize corpus linking during seed
**Evaluated:** 2026-09-18T14:53:31.821538
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin
  ✓ secrets: [90m9:48AM[0m [32mINF[0m [1mscanned ~7153594 bytes (7.15 MB) in 1.18s[0m
[90m9:48AM[0m [32m
- ✓ **tier2**
  - COMPLETE
  ✓ Seed preserves the current similar-edge count and idempotent rerun while materially reducing link-phase work; tests and Go quality gates pass: Commit 953e2de rewrites internal/graph/linker.go: LinkAllSimilar now calls loadTitleIndex() (each title tokenized once + inverted token->positions map) and candidatesFor() instead of the old per-class LinkSimilarClasses table scan (O(n^2) tokenizations), and writeSimilarEdges() batches all edge INSERTs in ONE transaction (was one autocommit INSERT per edge). Semantics shared via similarWeight/rankCandidates/planSimilarEdges. EMPIRICAL (built both binaries, live 1828-file corpus): pre-opt (953e2de^) fresh seed 116.8s -> 7210 edges, idempotent re-run 9.63s -> 0 created; post-opt fresh seed 29.0s -> 7210 edges, idempotent re-run 0.28s -> 0 created. Edge tables byte-identical (both 7210 rows, set diff only-new=0/only-old=0, reverse-edge symmetry intact) => ~4x faster seed, ~34x faster rerun, identical edge count, idempotency preserved. TESTS: go build ./... exit 0; go vet ./... exit 0; gofmt -l cmd/ internal/ pkg/ sql/ empty; go test ./... -short -p 1 -count=1 -timeout 300s => 13 pkgs ok, 0 failures (graph 1.367s); new tests TestLinkAllSimilarMatchesPreOptimizationRanking, TestLinkSimilarClassesMatchesPreOptimizationRanking, TestLinkAllSimilarTokenizesEachTitleOnce, TestWriteSimilarEdgesBatchesAtomically all PASS; gitreins guard Tier 1 PASS 4/4 (secrets, go_build, go_lint, go_tests); LSP diagnostics 0. Parity oracle verified non-tautological: mutating only the optimized candidatesFor minShared filter made TestLinkAllSimilarMatchesPreOptimizationRanking FAIL with 8 edge diffs (file restored, tree clean).
Seed linking is materially faster (fresh seed 116.8s->29.0s, idempotent rerun 9.63s->0.28s) with a byte-identical 7210-edge table and preserved idempotency, and all Go quality gates plus the new parity/tokenization/atomicity tests pass.

## Summary

Judge Result: OB-GAP-073

Stage tier1: PASS
    ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin
  ✓ secrets: [90m9:48AM[0m [32mINF[0m [1mscanned ~7153594 bytes (7.15 MB) in 1.18s[0m
[90m9:48AM[0m [32m

Stage tier2: PASS
  COMPLETE
  ✓ Seed preserves the current similar-edge count and idempotent rerun while materially reducing link-phase work; tests and Go quality gates pass: Commit 953e2de rewrites internal/graph/linker.go: LinkAllSimilar now calls loadTitleIndex() (each title tokenized once + inverted token->positions map) and candidatesFor() instead of the old per-class LinkSimilarClasses table scan (O(n^2) tokenizations), and writeSimilarEdges() batches all edge INSERTs in ONE transaction (was one autocommit INSERT per edge). Semantics shared via similarWeight/rankCandidates/planSimilarEdges. EMPIRICAL (built both binaries, live 1828-file corpus): pre-opt (953e2de^) fresh seed 116.8s -> 7210 edges, idempotent re-run 9.63s -> 0 created; post-opt fresh seed 29.0s -> 7210 edges, idempotent re-run 0.28s -> 0 created. Edge tables byte-identical (both 7210 rows, set diff only-new=0/only-old=0, reverse-edge symmetry intact) => ~4x faster seed, ~34x faster rerun, identical edge count, idempotency preserved. TESTS: go build ./... exit 0; go vet ./... exit 0; gofmt -l cmd/ internal/ pkg/ sql/ empty; go test ./... -short -p 1 -count=1 -timeout 300s => 13 pkgs ok, 0 failures (graph 1.367s); new tests TestLinkAllSimilarMatchesPreOptimizationRanking, TestLinkSimilarClassesMatchesPreOptimizationRanking, TestLinkAllSimilarTokenizesEachTitleOnce, TestWriteSimilarEdgesBatchesAtomically all PASS; gitreins guard Tier 1 PASS 4/4 (secrets, go_build, go_lint, go_tests); LSP diagnostics 0. Parity oracle verified non-tautological: mutating only the optimized candidatesFor minShared filter made TestLinkAllSimilarMatchesPreOptimizationRanking FAIL with 8 edge diffs (file restored, tree clean).
Seed linking is materially faster (fresh seed 116.8s->29.0s, idempotent rerun 9.63s->0.28s) with a byte-identical 7210-edge table and preserved idempotency, and all Go quality gates plus the new parity/tokenization/atomicity tests pass.

Overall: PASS ✓
