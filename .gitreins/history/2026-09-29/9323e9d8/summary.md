# Verdict: REVIEW-OB-011

**Task:** Docs/config honesty for inert embeddings package
**Evaluated:** 2026-09-29T18:12:42.176780
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
- ✓ **tier2**
  - COMPLETE
  ✓ README and docs/integration.md no longer imply a working embeddings serving path, or a semantic fallback is wired and tested: Fix is on branch wt/REVIEW-OB-011 (commit 19ac6995, worktree /home/kara/worktrees/off-by-one-REVIEW-OB-011; master/HEAD 980a17e0 does not contain it). README.md:71-75 adds '> **Experimental, not wired:** the semantic-embeddings layer in `internal/graph` (DS-003 — `OpenRouterEmbedder`, `SimilaritySearch`, the `problem_class_embeddings` schema) exists and is unit-tested, but as of v0.1.x it has **no production callers and no API route**; no serving path uses it.' README.md:248 OPENROUTER_API_KEY row rewritten to '...experimental and unwired — it has no production callers and no API route as of v0.1.x, so this key has no runtime effect on the served API'. docs/integration.md:39 and :407 both rewritten ('used only by the internal DS-003 embeddings package, which is experimental and unwired — it has no production callers and no API route, so the key has no runtime effect on the served API' / 'Optional; experimental — read only by the unwired DS-003 embeddings package, no runtime effect on the served API'). .env.example:4-6 comment corrected. The disclaimer is factually accurate: grep -rn 'DiscoveryWithSimilar|OpenRouterEmbedder|graph.Embedder|InitEmbeddings|SimilarClasses' --include=*.go (non-test) returns only internal/graph/embeddings.go itself, and no embeddings route exists in internal/api/handlers.go or pkg/api/openapi.yaml. grep -i 'embedding|semantic' on README.md and docs/integration.md now returns only the new disclaimers plus unrelated 'semantics' prose — no remaining implication of a working embeddings serving path. (Residual, outside the criterion's named scope: specs/system-spec.md:599 still lists 'Semantic similarity (OpenRouter embeddings) — score 0.3–0.7' as a ranking step.) Criterion is docs-only, so no test/build/lint run was required. [resolution 0.24; docs/integration.md]
README.md and docs/integration.md on the task branch (19ac6995) explicitly mark the DS-003 embeddings layer as experimental/unwired with no production callers or API route, and that claim matches the code (no non-test callers, no route).

## Summary

Judge Result: REVIEW-OB-011

Stage tier1: PASS
    ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)

Stage tier2: PASS
  COMPLETE
  ✓ README and docs/integration.md no longer imply a working embeddings serving path, or a semantic fallback is wired and tested: Fix is on branch wt/REVIEW-OB-011 (commit 19ac6995, worktree /home/kara/worktrees/off-by-one-REVIEW-OB-011; master/HEAD 980a17e0 does not contain it). README.md:71-75 adds '> **Experimental, not wired:** the semantic-embeddings layer in `internal/graph` (DS-003 — `OpenRouterEmbedder`, `SimilaritySearch`, the `problem_class_embeddings` schema) exists and is unit-tested, but as of v0.1.x it has **no production callers and no API route**; no serving path uses it.' README.md:248 OPENROUTER_API_KEY row rewritten to '...experimental and unwired — it has no production callers and no API route as of v0.1.x, so this key has no runtime effect on the served API'. docs/integration.md:39 and :407 both rewritten ('used only by the internal DS-003 embeddings package, which is experimental and unwired — it has no production callers and no API route, so the key has no runtime effect on the served API' / 'Optional; experimental — read only by the unwired DS-003 embeddings package, no runtime effect on the served API'). .env.example:4-6 comment corrected. The disclaimer is factually accurate: grep -rn 'DiscoveryWithSimilar|OpenRouterEmbedder|graph.Embedder|InitEmbeddings|SimilarClasses' --include=*.go (non-test) returns only internal/graph/embeddings.go itself, and no embeddings route exists in internal/api/handlers.go or pkg/api/openapi.yaml. grep -i 'embedding|semantic' on README.md and docs/integration.md now returns only the new disclaimers plus unrelated 'semantics' prose — no remaining implication of a working embeddings serving path. (Residual, outside the criterion's named scope: specs/system-spec.md:599 still lists 'Semantic similarity (OpenRouter embeddings) — score 0.3–0.7' as a ranking step.) Criterion is docs-only, so no test/build/lint run was required. [resolution 0.24; docs/integration.md]
README.md and docs/integration.md on the task branch (19ac6995) explicitly mark the DS-003 embeddings layer as experimental/unwired with no production callers or API route, and that claim matches the code (no non-test callers, no route).

Overall: PASS ✓
