# Verdict: OB-GAP-073

**Task:** Optimize corpus linking during seed
**Evaluated:** 2026-09-18T14:59:30.823758
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin
  ✓ secrets: [90m9:53AM[0m [32mINF[0m [1mscanned ~7175027 bytes (7.18 MB) in 1.05s[0m
[90m9:53AM[0m [32m
- ✓ **tier2**
  - COMPLETE
  ✓ Seed preserves the current similar-edge count and idempotent rerun while materially reducing link-phase work; tests and Go quality gates pass: Edge count preserved: seeded the real 1828-file corpus with the new binary (HEAD ac56559) and the pre-optimization binary (bf25f1a) — both reported 'edges=7210 created', and a Python dump of both problem_edges tables (source_id,target_id,relationship,weight ORDER BY) was IDENTICAL (7210 rows, same weights/directions). Idempotent rerun: both binaries reported 'classes=0 created / 1828 existing; answers=0 created / 1916 skipped; edges=0 created'; internal/seed TestSeedIdempotentRerun and TestSeedCreatesRelatedEdges PASS (asserts 2nd run EdgesCreated==0 and unchanged row count). Material link-phase reduction: isolated benchmark of LinkAllSimilar on the live 1828-class DB gave OLD 4.19s/op (4.10/4.17/4.23s) vs NEW 0.111s/op (111/116/108ms) — ~38x, from replacing O(classes^2) title tokenizations with one tokenization per class (titleIndex + inverted byToken map) and ~7800 autocommit INSERTs with one transaction (writeSimilarEdges). Go gates run fresh: go build ./... exit 0; go vet ./... exit 0; gofmt -l cmd/ internal/ pkg/ sql/ empty (exit 0); go test ./... -short -p 1 -count=1 -timeout 300s exit 0 with all 13 packages 'ok' (graph 1.039s, seed 1.445s) and 0 failures. New tests in internal/graph/linker_test.go all PASS: TestLinkAllSimilarMatchesPreOptimizationRanking (exact edge-set equality vs an independent pre-optimization oracle + idempotent 2nd pass), TestLinkSimilarClassesMatchesPreOptimizationRanking (per-call deltas vs oracle), TestLinkAllSimilarTokenizesEachTitleOnce (12 classes => exactly 12 tokenizer calls, not 144), TestWriteSimilarEdgesBatchesAtomically (failed batch rolls back to 0 rows, control batch=2, rerun=0).
Seed produces a byte-identical 7210-edge similar graph with a 0-edge idempotent rerun while the link phase drops from 4.19s to 0.111s (~38x), and all Go build/vet/gofmt/test gates pass.

## Summary

Judge Result: OB-GAP-073

Stage tier1: PASS
    ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin
  ✓ secrets: [90m9:53AM[0m [32mINF[0m [1mscanned ~7175027 bytes (7.18 MB) in 1.05s[0m
[90m9:53AM[0m [32m

Stage tier2: PASS
  COMPLETE
  ✓ Seed preserves the current similar-edge count and idempotent rerun while materially reducing link-phase work; tests and Go quality gates pass: Edge count preserved: seeded the real 1828-file corpus with the new binary (HEAD ac56559) and the pre-optimization binary (bf25f1a) — both reported 'edges=7210 created', and a Python dump of both problem_edges tables (source_id,target_id,relationship,weight ORDER BY) was IDENTICAL (7210 rows, same weights/directions). Idempotent rerun: both binaries reported 'classes=0 created / 1828 existing; answers=0 created / 1916 skipped; edges=0 created'; internal/seed TestSeedIdempotentRerun and TestSeedCreatesRelatedEdges PASS (asserts 2nd run EdgesCreated==0 and unchanged row count). Material link-phase reduction: isolated benchmark of LinkAllSimilar on the live 1828-class DB gave OLD 4.19s/op (4.10/4.17/4.23s) vs NEW 0.111s/op (111/116/108ms) — ~38x, from replacing O(classes^2) title tokenizations with one tokenization per class (titleIndex + inverted byToken map) and ~7800 autocommit INSERTs with one transaction (writeSimilarEdges). Go gates run fresh: go build ./... exit 0; go vet ./... exit 0; gofmt -l cmd/ internal/ pkg/ sql/ empty (exit 0); go test ./... -short -p 1 -count=1 -timeout 300s exit 0 with all 13 packages 'ok' (graph 1.039s, seed 1.445s) and 0 failures. New tests in internal/graph/linker_test.go all PASS: TestLinkAllSimilarMatchesPreOptimizationRanking (exact edge-set equality vs an independent pre-optimization oracle + idempotent 2nd pass), TestLinkSimilarClassesMatchesPreOptimizationRanking (per-call deltas vs oracle), TestLinkAllSimilarTokenizesEachTitleOnce (12 classes => exactly 12 tokenizer calls, not 144), TestWriteSimilarEdgesBatchesAtomically (failed batch rolls back to 0 rows, control batch=2, rerun=0).
Seed produces a byte-identical 7210-edge similar graph with a 0-edge idempotent rerun while the link phase drops from 4.19s to 0.111s (~38x), and all Go build/vet/gofmt/test gates pass.

Overall: PASS ✓
