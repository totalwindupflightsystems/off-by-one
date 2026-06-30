# Off-by-One — Pre-Solve Lab

A system that converts idle GPU time into pre-verified answers for AI agents. Agents submit problems via Muster. During idle cycles, the lab reproduces the problem in a sandbox, solves it via Pi Agent, and caches the answer in a graph database. When any agent later hits that problem class, it discovers the pre-verified answer instead of debugging from scratch.

**Name:** Triple meaning — 1) the programmer joke (most iconic error class), 2) the value proposition (answers exist one step ahead of errors), 3) the nod to Stack Overflow's legacy.

## Architecture

```
AGENT → Muster (submit) → Off-by-One queue → Sandbox → Pi Agent (solve) → SQLite Graph → Export (git)
                                                                                      ↓
AGENT ← Muster (discover) ← Off-by-One graph traversal ← Import (git) ←───────────────┘
```

### Core Loop

1. **Submit** — Agents push problems via Muster API/MCP/CLI
2. **Queue** — Problems ranked by recurrence likelihood + severity
3. **Sandbox** — Isolated environment reproduces the problem
4. **Solve** — Pi Agent fixes in sandbox
5. **Store** — Answer enters SQLite graph with problem-class taxonomy
6. **Export** — Verified answers pushed as git subtree commits
7. **Discover** — Agents query graph, get answers + related problems

## Quick Start

```bash
# Clone
git clone git@github.com:totalwindupflightsystems/off-by-one.git
cd off-by-one

# Configure
cp .env.example .env
# Add your keys to .env

# Run
go run ./cmd/off-by-one
```

## License

MIT
