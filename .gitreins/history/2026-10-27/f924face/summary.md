# Verdict: DOC-6

**Task:** Document solver key fallback and PI_MODEL override
**Evaluated:** 2026-10-27T16:30:16.171091
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
- ✓ **tier2**
  - COMPLETE
  ✓ docs/integration.md documents DeepSeek preferred key, OpenRouter fallback plus embeddings note, and PI_MODEL blank/provider-qualified override; help exits 0 and grep criteria pass: All four documentation elements present and accurate. docs/integration.md:39 (prose) states: '`DEEPSEEK_API_KEY` is preferred for solves'; 'If `DEEPSEEK_API_KEY` is absent or holds a placeholder value, the solver falls back to `OPENROUTER_API_KEY` and logs which path is used (`DEEPSEEK_API_KEY empty — using OPENROUTER_API_KEY for solves`)'; '`OPENROUTER_API_KEY` is also read by the internal DS-003 embeddings package, which is experimental and unwired — it has no production callers and no API route'; '`PI_MODEL` is an optional provider-qualified model override for the solver: a non-blank value is forwarded verbatim, while an empty or whitespace-only value is treated as unset'. Config table repeats them: docs/integration.md:444 (DEEPSEEK preferred), :445 (OpenRouter fallback + embeddings), :446 (PI_MODEL provider-qualified, blank=unset). Docs match real code, not fiction: cmd/off-by-one/main.go:154-163 implements the exact fallback chain (DEEPSEEK_API_KEY -> looksPlaceholderAPIKey -> OPENROUTER_API_KEY, logging 'DEEPSEEK_API_KEY empty — using OPENROUTER_API_KEY for solves', else WARNING ... 401); cmd/off-by-one/main.go:451-456 resolveSolverModel uses strings.TrimSpace(PI_MODEL) so blank/whitespace = unset, else solver.DefaultModel (internal/solver/piagent.go:29); cmd/off-by-one/main.go:465-470 solverExtraEnv forwards PI_MODEL only when non-blank; scripts/pi-agent:108-110 applies the same verbatim rule. Embeddings 'unwired' claim verified: grep for Embedding/embeddings across internal/ and cmd/ (excluding internal/graph/embeddings.go) returns only a comment at cmd/off-by-one/main.go:20 — no production callers. help exits 0: built ./cmd/off-by-one (build_exit=0), `/tmp/obo --help` -> help_exit=0 and `/tmp/obo -h` -> h_exit=0, printing 'Usage of off-by-one:' with Commands/Flags. grep criteria pass: grep -c PI_MODEL docs/integration.md = 2; 'provider-qualified' at lines 39,446; 'placeholder' at lines 39,445; 'embeddings' at lines 39,445. Tests: `go test ./... -short -p 1 -count=1 -timeout 180s` exit_code=0, all packages ok (cmd/off-by-one 1.741s, internal/api 0.631s, internal/solver 2.879s, internal/graph 4.507s, internal/sandbox 1.307s), no FAIL lines. [resolution 0.26; docs/integration.md]
docs/integration.md documents the DeepSeek-preferred/OpenRouter-fallback key chain with the embeddings caveat and the PI_MODEL blank-vs-provider-qualified override, all matching the actual code in cmd/off-by-one/main.go and scripts/pi-agent; --help exits 0 and the full short test suite passes.

## Summary

Judge Result: DOC-6

Stage tier1: PASS
    ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)

Stage tier2: PASS
  COMPLETE
  ✓ docs/integration.md documents DeepSeek preferred key, OpenRouter fallback plus embeddings note, and PI_MODEL blank/provider-qualified override; help exits 0 and grep criteria pass: All four documentation elements present and accurate. docs/integration.md:39 (prose) states: '`DEEPSEEK_API_KEY` is preferred for solves'; 'If `DEEPSEEK_API_KEY` is absent or holds a placeholder value, the solver falls back to `OPENROUTER_API_KEY` and logs which path is used (`DEEPSEEK_API_KEY empty — using OPENROUTER_API_KEY for solves`)'; '`OPENROUTER_API_KEY` is also read by the internal DS-003 embeddings package, which is experimental and unwired — it has no production callers and no API route'; '`PI_MODEL` is an optional provider-qualified model override for the solver: a non-blank value is forwarded verbatim, while an empty or whitespace-only value is treated as unset'. Config table repeats them: docs/integration.md:444 (DEEPSEEK preferred), :445 (OpenRouter fallback + embeddings), :446 (PI_MODEL provider-qualified, blank=unset). Docs match real code, not fiction: cmd/off-by-one/main.go:154-163 implements the exact fallback chain (DEEPSEEK_API_KEY -> looksPlaceholderAPIKey -> OPENROUTER_API_KEY, logging 'DEEPSEEK_API_KEY empty — using OPENROUTER_API_KEY for solves', else WARNING ... 401); cmd/off-by-one/main.go:451-456 resolveSolverModel uses strings.TrimSpace(PI_MODEL) so blank/whitespace = unset, else solver.DefaultModel (internal/solver/piagent.go:29); cmd/off-by-one/main.go:465-470 solverExtraEnv forwards PI_MODEL only when non-blank; scripts/pi-agent:108-110 applies the same verbatim rule. Embeddings 'unwired' claim verified: grep for Embedding/embeddings across internal/ and cmd/ (excluding internal/graph/embeddings.go) returns only a comment at cmd/off-by-one/main.go:20 — no production callers. help exits 0: built ./cmd/off-by-one (build_exit=0), `/tmp/obo --help` -> help_exit=0 and `/tmp/obo -h` -> h_exit=0, printing 'Usage of off-by-one:' with Commands/Flags. grep criteria pass: grep -c PI_MODEL docs/integration.md = 2; 'provider-qualified' at lines 39,446; 'placeholder' at lines 39,445; 'embeddings' at lines 39,445. Tests: `go test ./... -short -p 1 -count=1 -timeout 180s` exit_code=0, all packages ok (cmd/off-by-one 1.741s, internal/api 0.631s, internal/solver 2.879s, internal/graph 4.507s, internal/sandbox 1.307s), no FAIL lines. [resolution 0.26; docs/integration.md]
docs/integration.md documents the DeepSeek-preferred/OpenRouter-fallback key chain with the embeddings caveat and the PI_MODEL blank-vs-provider-qualified override, all matching the actual code in cmd/off-by-one/main.go and scripts/pi-agent; --help exits 0 and the full short test suite passes.

Overall: PASS ✓
