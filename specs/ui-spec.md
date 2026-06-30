# Off-by-One — Web UI Specification

> **Version:** 0.1.0
> **Target:** Single-page application, embedded in Go binary
> **Stack:** Vanilla JS + HTMX + D3.js, served via `embed.FS`

---

## 1. Shell Layout

```
┌──────────────────────────────────────────────────────────────────┐
│  Off-by-One — Pre-Solve Lab                    [⚙ Settings]     │
├──────────────────────────────────────────────────────────────────┤
│  [🔍 Search] [📤 Submit] [🌳 Explore] [📦 Export] [📥 Import]   │
├────────────────────────────────────────────────┬─────────────────┤
│                                                │  💬 AI Agent    │
│              Main Content                      │  ─────────────  │
│              (view-dependent)                  │                 │
│                                                │  [chat history] │
│                                                │                 │
│                                                │                 │
│                                                │                 │
│                                                │  ┌───────────┐  │
│                                                │  │ type...    │  │
│                                                │  └───────────┘  │
│                                                │  [Send]         │
├────────────────────────────────────────────────┴─────────────────┤
│  Queue: 3 pending | Cache: 12 answers | Hit rate: 23% | v0.1.0  │
└──────────────────────────────────────────────────────────────────┘
```

### 1.1 Persistent Elements

| Element | Position | Behavior |
|---------|----------|----------|
| Header bar | Top, full width | Title left, settings gear right |
| Nav tabs | Below header | Active tab highlighted, others muted |
| Main content | Left 70% | Swaps per active tab |
| Chat sidebar | Right 30% | Collapsible via `«` / `»` toggle |
| Status bar | Bottom, full width | Live stats, refreshes every 30s |

### 1.2 Responsive Breakpoints

| Width | Behavior |
|-------|----------|
| >1024px | Side-by-side: main + chat |
| 768–1024px | Chat collapses to bottom panel |
| <768px | Single column, chat hidden, tabs scroll horizontally |

### 1.3 States

| State | Visual |
|-------|--------|
| **Loading** | Skeleton placeholders (grey pulsing bars) in main content area |
| **Empty** | Illustration + "No results yet" message with CTA button |
| **Error** | Red banner at top with error message + retry button |
| **Offline** | Yellow banner: "Server unreachable — retrying in 10s..." |

---

## 2. Search View

### 2.1 Layout

```
┌──────────────────────────────────────────────────┐
│  🔍 [________________________________________]  │
│  Filters: [env: all ▾] [lang: all ▾] [status: all ▾] │
├──────────────────────────────────────────────────┤
│                                                  │
│  ┌──────────────────────────────────────────────┐│
│  │ ✅ file-ownership-after-container-transfer   ││
│  │    Files owned by root after Docker volume    ││
│  │    transfer. 1 answer · 47 hits · last: 2h   ││
│  │    Tags: docker, permissions, volume          ││
│  └──────────────────────────────────────────────┘│
│                                                  │
│  ┌──────────────────────────────────────────────┐│
│  │ ⏳ docker-cp-permission-denied                ││
│  │    Permission denied after docker cp to       ││
│  │    running container. 0 answers · queued      ││
│  │    Tags: docker, cp, permissions              ││
│  └──────────────────────────────────────────────┘│
│                                                  │
│  ┌──────────────────────────────────────────────┐│
│  │ ❌ go-mod-tidy-network-failure                ││
│  │    go mod tidy fails behind corporate proxy.  ││
│  │    Attempted 3x · 0 answers · failed          ││
│  │    Tags: go, proxy, network                   ││
│  └──────────────────────────────────────────────┘│
│                                                  │
│  Showing 1–3 of 12          [◀ 1 2 3 ▶]        │
└──────────────────────────────────────────────────┘
```

### 2.2 Result Card States

| Status | Badge | Color | Meaning |
|--------|-------|-------|---------|
| `verified` | ✅ | Green | Pre-verified answer exists, 2+ validator sigs |
| `pending` | 🔄 | Blue | Answer exists but unverified (pre-validator-ring MVP) |
| `queued` | ⏳ | Yellow | In queue, no answer yet |
| `in_progress` | 🔧 | Orange | Being solved right now |
| `failed` | ❌ | Red | Attempted, couldn't solve |
| `imported` | 📥 | Purple | Pulled from community repo |

### 2.3 Interactions

| Trigger | Action |
|---------|--------|
| Type in search bar | Debounced 300ms → `GET /api/v1/problems?q=<input>` |
| Change filter | Immediate → refresh results with filter params |
| Click result card | Expand inline to show full answer (see §2.4) |
| Click tag | Add to search bar as filter |
| Keyboard: `/` | Focus search bar |
| Keyboard: `Esc` | Clear search, collapse expanded result |
| Keyboard: `↓`/`↑` | Navigate results |
| Keyboard: `Enter` | Expand selected result |

### 2.4 Expanded Result

```
┌──────────────────────────────────────────────────┐
│  ✅ file-ownership-after-container-transfer  [✕] │
│                                                  │
│  Problem: When transferring files from build     │
│  container to runtime container via Docker       │
│  volume, file ownership defaults to root:root.   │
│                                                  │
│  ┌─ Solution ──────────────────────────────────┐ │
│  │ Use `COPY --chown=appuser:appuser` in        │ │
│  │ Dockerfile instead of relying on volume      │ │
│  │ mount ownership propagation.                 │ │
│  │                                              │ │
│  │ ```dockerfile                                │ │
│  │ COPY --chown=appuser:appuser \               │ │
│  │   --from=builder /build/output /app          │ │
│  │ ```                                          │ │
│  │ [📋 Copy]                                    │ │
│  └──────────────────────────────────────────────┘ │
│                                                  │
│  ⚠ Version warning: Docker 26.0+ changed         │
│  default ownership. Tested on 24.x–25.x.         │
│                                                  │
│  📊 Evidence: 2/2 validators passed · 6 tests    │
│  🔗 Related: docker-volume-permissions (95%)     │
│             chown-in-dockerfile (85%)            │
│                                                  │
│  [📤 Export] [📋 Copy solution] [🔗 Copy link]   │
└──────────────────────────────────────────────────┘
```

### 2.5 Empty State

```
┌──────────────────────────────────────────────────┐
│                                                  │
│              🔍 No results found                 │
│                                                  │
│     No pre-solved answers match your search.     │
│                                                  │
│     ┌─────────────────────────────────────┐      │
│     │  Try:                                │      │
│     │  • Different keywords                │      │
│     │  • Remove filters                    │      │
│     │  • Browse the taxonomy               │      │
│     └─────────────────────────────────────┘      │
│                                                  │
│              [📤 Submit this problem]            │
│              [🌳 Browse taxonomy]                │
│                                                  │
└──────────────────────────────────────────────────┘
```

---

## 3. Submit View

### 3.1 Layout

```
┌──────────────────────────────────────────────────┐
│  Submit a Problem                                │
│                                                  │
│  Cadence:  ○ Pre-phase  ○ End-of-day  ● Post-debug│
│                                                  │
│  Problem Class *                                 │
│  ┌──────────────────────────────────────────────┐│
│  │ file-ownership-after-container-tra…          ││
│  └──────────────────────────────────────────────┘│
│  ↳ auto-suggest: file-ownership-after-container-transfer │
│    docker-volume-permissions                     │
│    chown-in-dockerfile                           │
│                                                  │
│  Environment *         Language *    Version *   │
│  [docker       ▾]     [go    ▾]     [go-1.26 ▾] │
│                                                  │
│  Description *                                   │
│  ┌──────────────────────────────────────────────┐│
│  │ When transferring files from a build         ││
│  │ container to a runtime container via Docker  ││
│  │ volume, file ownership defaults to root:root ││
│  │ instead of the target user. This causes      ││
│  │ 'permission denied' when the runtime         ││
│  │ container tries to execute or read files.    ││
│  │                                              ││
│  └──────────────────────────────────────────────┘│
│                                                  │
│  Error Message                                   │
│  ┌──────────────────────────────────────────────┐│
│  │ fork/exec /app/binary: permission denied     ││
│  └──────────────────────────────────────────────┘│
│                                                  │
│  Stack Trace (optional)                          │
│  ┌──────────────────────────────────────────────┐│
│  │ ...                                          ││
│  └──────────────────────────────────────────────┘│
│                                                  │
│  Context (optional)                              │
│  Project: [muster        ]                       │
│  Files:   [Dockerfile, docker-compose.yml]       │
│                                                  │
│  ┌──────────────┐                                │
│  │  📤 Submit    │                                │
│  └──────────────┘                                │
└──────────────────────────────────────────────────┘
```

### 3.2 Problem Class Auto-Suggest

As user types, query `GET /api/v1/problems?q=<prefix>&limit=5`. Show dropdown with existing problem classes + short descriptions. Selecting an existing class pre-fills env/lang/version from the most recent submission.

### 3.3 Validation Rules

| Field | Required | Rules |
|-------|----------|-------|
| Problem class | Yes | 3–100 chars, kebab-case validated |
| Environment | Yes | From list: docker, k8s, bare-metal, aws, gcp, azure, other |
| Language | Yes | From list: go, python, typescript, rust, java, ruby, other |
| Version | Yes | Free text, suggested format: `go-1.26` |
| Description | Yes | 10–5000 chars |
| Error message | No | 0–2000 chars |
| Stack trace | No | 0–10000 chars |

### 3.4 Submit States

| State | Visual |
|-------|--------|
| **Idle** | Form ready, Submit button active |
| **Validating** | Fields with errors highlighted red, messages below |
| **Submitting** | Submit button disabled, spinner, "Submitting..." |
| **Success** | Green checkmark + "Problem queued! Position #3 · ~2m estimated" + [View queue] + [Submit another] |
| **Error** | Red banner: error message + retry button |

### 3.5 Submit via Chat Shortcut

If user types a problem in chat and Pi Agent suggests submitting, the chat panel has a "Submit to Queue" button that pre-fills the Submit form with the problem details extracted by the agent.

---

## 4. Explore View (Taxonomy Browser)

### 4.1 Layout

```
┌────────────────────────────┬─────────────────────┐
│  Taxonomy                  │  Details             │
│                            │                     │
│  ▼ docker                  │  file-ownership-    │
│    ▼ go                    │  after-container-   │
│      ▼ go-1.25             │  transfer           │
│        📄 answer (Mar 12)  │                     │
│      ▼ go-1.26             │  Files owned by     │
│        📄 answer (Jun 28) ✅│  root after Docker  │
│    ▼ python                │  volume transfer.   │
│      ▼ python-3.11         │                     │
│        📄 answer (May 3)   │  1 answer · 47 hits │
│  ▼ k8s                     │  Status: ✅ Verified│
│    ▼ go                    │                     │
│      ...                   │  [🔗 Related graph] │
│                            │  [📤 Export]        │
│                            │                     │
└────────────────────────────┴─────────────────────┘
```

### 4.2 Tree Nodes

| Icon | Type | Click behavior |
|------|------|----------------|
| ▶ / ▼ | Expandable category (problem class, env, lang, version) | Toggle expand/collapse |
| 📄 | Answer node | Select → show details in right panel |
| ✅ | Verified answer | Green badge overlay |
| ⏳ | Queued problem | Yellow badge overlay |
| ❌ | Failed attempt | Red badge overlay |

### 4.3 Right Panel — Selected Answer

Same as Search > Expanded Result (§2.4), with added "Related Graph" button.

### 4.4 Related Graph (D3.js)

Clicking "Related Graph" opens a modal with a D3.js force-directed graph:

```
┌──────────────────────────────────────────────────┐
│  Related Problems — file-ownership-after-...  [✕]│
│                                                  │
│     ● chown-in-dockerfile                        │
│     │                                            │
│     │ prerequisite (0.85)                        │
│     │                                            │
│     ● file-ownership-after-container-transfer ← selected │
│     │                                            │
│     │ same_root_cause (0.95)                     │
│     │                                            │
│     ● docker-volume-permissions                  │
│                                                  │
│  Click any node to navigate to that problem.     │
└──────────────────────────────────────────────────┘
```

Graph rendering:
- Nodes: problem class names, 12px font
- Selected node: highlighted border, larger radius
- Edges: labeled with relationship type + weight
- Force layout: charge -300, link distance 100
- Drag nodes to reposition
- Click node → close modal, navigate to that problem

---

## 5. Export View

### 5.1 Layout

```
┌──────────────────────────────────────────────────┐
│  Export Answers to Git                           │
│                                                  │
│  Step 1: Select answers                          │
│  ┌──────────────────────────────────────────────┐│
│  │ ☑ file-ownership-after-container-transfer    ││
│  │   docker / go-1.26 · verified · 47 hits      ││
│  │ ☑ docker-volume-permissions                  ││
│  │   docker / python-3.11 · verified · 23 hits  ││
│  │ ☐ chown-in-dockerfile                        ││
│  │   docker / go-1.26 · verified · 12 hits      ││
│  │ ☐ k8s-pod-volume-mount-race                  ││
│  │   k8s / go-1.25 · pending · 0 hits           ││
│  └──────────────────────────────────────────────┘│
│  [Select all verified] [Select all] [Clear]      │
│                                                  │
│  Step 2: Target repository                       │
│  ┌──────────────────────────────────────────────┐│
│  │ git@github.com:wojons/off-by-one-answers.git ││
│  └──────────────────────────────────────────────┘│
│  Branch: [main           ▾]                     │
│                                                  │
│  Step 3: Preview                                 │
│  ┌──────────────────────────────────────────────┐│
│  │ + pre-solve-answers/file-ownership-after-    ││
│  │   container-transfer/docker/go-1.26/         ││
│  │   ├── solution.md          (1.2 KB)          ││
│  │   ├── evidence.md          (0.8 KB)          ││
│  │   └── signatures.json      (0.2 KB)          ││
│  │ + pre-solve-answers/docker-volume-            ││
│  │   permissions/docker/python-3.11/             ││
│  │   ├── solution.md          (0.9 KB)          ││
│  │   ├── evidence.md          (0.6 KB)          ││
│  │   └── signatures.json      (0.2 KB)          ││
│  │                                              ││
│  │ Commit message: export: 2 verified answers   ││
│  └──────────────────────────────────────────────┘│
│                                                  │
│  ┌──────────────┐                                │
│  │  📤 Push      │                                │
│  └──────────────┘                                │
└──────────────────────────────────────────────────┘
```

### 5.2 Export States

| State | Visual |
|-------|--------|
| **Selecting** | Checkboxes active, preview updates on change |
| **Validating** | Check git repo reachable, branch exists |
| **Preview** | Diff view of what will be committed |
| **Pushing** | Spinner + "Pushing 2 answers to github.com:..." |
| **Success** | Green banner: "Exported 2 answers · commit a1b2c3d" + copyable link |
| **Error** | Red banner: error message + retry |

### 5.3 Edge Cases

| Case | Behavior |
|------|----------|
| No verified answers | "No verified answers to export. Only verified answers can be exported." |
| Repo unreachable | Error state with "Check repository URL and SSH key configuration" |
| Branch protected | Error state with "Branch is protected. Push to a different branch or configure access." |
| Conflict on push | Error state with "Remote has diverged. Pull first." |
| Large export (>50 answers) | Confirmation dialog: "You're exporting 87 answers. This will create a large commit. Continue?" |

---

## 6. Import View

### 6.1 Layout

```
┌──────────────────────────────────────────────────┐
│  Import Answers from Git                         │
│                                                  │
│  Source repository                               │
│  ┌──────────────────────────────────────────────┐│
│  │ git@github.com:wojons/off-by-one-answers.git ││
│  └──────────────────────────────────────────────┘│
│  ┌──────────────┐                                │
│  │  🔍 Preview   │                                │
│  └──────────────┘                                │
│                                                  │
│  ── Results ──────────────────────────────────── │
│                                                  │
│  New (3) ─ will be added to local graph          │
│  ┌──────────────────────────────────────────────┐│
│  │ 📥 file-ownership-after-container-transfer   ││
│  │    docker / go-1.26 · 2 signatures           ││
│  │ ☑ Import                                     ││
│  │ 📥 docker-volume-permissions                  ││
│  │    docker / python-3.11 · 2 signatures       ││
│  │ ☑ Import                                     ││
│  │ 📥 chown-in-dockerfile                        ││
│  │    docker / go-1.26 · 2 signatures           ││
│  │ ☑ Import                                     ││
│  └──────────────────────────────────────────────┘│
│                                                  │
│  Updated (1) — newer version available            │
│  ┌──────────────────────────────────────────────┐│
│  │ 📥 k8s-pod-volume-mount-race                  ││
│  │    Remote: go-1.26 (Jun 29)                  ││
│  │    Local:  go-1.25 (May 12)                  ││
│  │ ☐ Import (will replace local)                ││
│  └──────────────────────────────────────────────┘│
│                                                  │
│  Conflict (1) — same version, different answer    │
│  ┌──────────────────────────────────────────────┐│
│  │ ⚠ go-mod-tidy-network-failure                ││
│  │    docker / go-1.26                          ││
│  │    [View diff]                                ││
│  │    ○ Keep local  ● Import remote              ││
│  └──────────────────────────────────────────────┘│
│                                                  │
│  Skipped (2) — already in local graph             │
│  ┌──────────────────────────────────────────────┐│
│  │ ✓ npm-install-node-gyp-failure (identical)   ││
│  │ ✓ pip-install-cuda-toolkit-mismatch (identical)││
│  └──────────────────────────────────────────────┘│
│                                                  │
│  ┌──────────────┐                                │
│  │  📥 Import 3   │                                │
│  └──────────────┘                                │
└──────────────────────────────────────────────────┘
```

### 6.2 Import Categories

| Category | Icon | Action |
|----------|------|--------|
| New | 📥 | Select to import → added to local graph |
| Updated | 📥 | Select to import → replaces local version |
| Conflict | ⚠ | Must choose: keep local or import remote |
| Skipped | ✓ | Already identical in local graph — no action needed |

### 6.3 Conflict Resolution Modal

```
┌──────────────────────────────────────────────────┐
│  Conflict: go-mod-tidy-network-failure        [✕]│
│                                                  │
│  ┌─ Local (current) ─┐  ┌─ Remote (import) ────┐│
│  │ Solution:          │  │ Solution:            ││
│  │ Set GOPROXY=direct │  │ Use go env -w        ││
│  │ and GONOSUMDB=*    │  │ GOPROXY=https://...  ││
│  │                    │  │                      ││
│  │ Evidence:          │  │ Evidence:            ││
│  │ 1/2 validators     │  │ 2/2 validators       ││
│  │ 3 tests            │  │ 7 tests              ││
│  │ Hits: 12           │  │ Hits: 34             ││
│  │ Created: May 3     │  │ Created: Jun 28      ││
│  └────────────────────┘  └──────────────────────┘│
│                                                  │
│  ○ Keep local     ● Import remote (recommended)  │
│                                                  │
│  Remote has more validator signatures, more      │
│  tests, and higher hit count.                    │
│                                                  │
│  [Confirm]                                       │
└──────────────────────────────────────────────────┘
```

### 6.4 Import States

| State | Visual |
|-------|--------|
| **Entering URL** | Input field + Preview button |
| **Fetching** | Spinner + "Fetching answers from github.com:..." |
| **Preview** | Categorized results with checkboxes |
| **Importing** | Spinner + "Importing 3 answers..." with progress bar |
| **Success** | Green banner: "Imported 3 new, 1 updated, 1 conflict resolved, 2 skipped" |
| **Error** | Red banner: error message + retry |
| **Empty repo** | "No pre-solve-answers/ directory found in this repository." |

---

## 7. AI Agent Chat Panel

### 7.1 Layout

```
┌─────────────────────────────────┐
│  💬 AI Agent              [− ✕]│
│  ───────────────────────────── │
│                                 │
│  [Agent] 👋 I'm the Off-by-One │
│  assistant. I can help you:    │
│  • Search for pre-solved       │
│    answers                     │
│  • Submit new problems         │
│  • Explain solutions           │
│  • Suggest related problems    │
│                                 │
│  [You] I'm getting "permission │
│  denied" after docker cp       │
│                                 │
│  [Agent] 🔍 Searching...       │
│                                 │
│  [Agent] I found a pre-solved  │
│  answer:                       │
│  ┌───────────────────────────┐ │
│  │ ✅ file-ownership-after-  │ │
│  │    container-transfer     │ │
│  │ 47 hits · 2/2 validators │ │
│  │ [View full answer]        │ │
│  └───────────────────────────┘ │
│  This is likely your issue.   │
│  The fix is to use COPY       │
│  --chown in your Dockerfile.  │
│                                 │
│  [You] That worked, thanks!    │
│  Can you submit it anyway so   │
│  others find it faster?        │
│                                 │
│  [Agent] It already has 47     │
│  hits — it's well-discovered!  │
│  No need to re-submit. 😊      │
│                                 │
│  ┌─────────────────────────┐   │
│  │ type a message...        │   │
│  └─────────────────────────┘   │
│  [Send]                        │
└─────────────────────────────────┘
```

### 7.2 Agent Capabilities

The chat agent (Pi Agent via WebSocket) can:

| Action | Trigger phrase examples | What happens |
|--------|------------------------|--------------|
| **Search** | "find", "look up", "search for", any error message | Searches local graph, returns top match |
| **Explain** | "explain", "what does this mean", "why" | Returns solution in plain language |
| **Submit** | "submit", "add to queue", "this isn't solved yet" | Pre-fills Submit form |
| **Suggest** | "what else", "related", "similar problems" | Returns lateral edges from graph |
| **Diagnose** | "debug", "what's wrong with", error paste | Reads error, searches graph, suggests fix |
| **Compare** | "which is better", "difference between" | Side-by-side of two solutions |

### 7.3 Message Types

| Type | Rendering |
|------|-----------|
| `text` | Plain text, markdown rendering |
| `search_result` | Rich card with problem class, status badge, hit count, expand button |
| `submit_confirm` | "Submit to queue?" card with pre-filled fields + Confirm/Cancel buttons |
| `error` | Red-tinted message with error details |
| `thinking` | Animated "..." dots while agent processes |
| `related` | Horizontal scroll of related problem cards |

### 7.4 Chat States

| State | Visual |
|-------|--------|
| **Connected** | Green dot next to "AI Agent", input enabled |
| **Connecting** | Yellow dot, "Connecting...", input disabled |
| **Disconnected** | Red dot, "Disconnected — retrying...", input disabled |
| **Agent thinking** | Animated dots in chat, no response yet |
| **Agent error** | Red message: "Sorry, I couldn't process that. Try rephrasing." |
| **Empty history** | Welcome message with suggested queries |

### 7.5 Suggested Queries (Empty State)

```
┌─────────────────────────────────┐
│  💬 AI Agent                    │
│  ───────────────────────────── │
│                                 │
│  👋 I'm your Off-by-One        │
│  assistant. Try asking:        │
│                                 │
│  ┌───────────────────────────┐ │
│  │ "I'm getting permission   │ │
│  │  denied in Docker"        │ │
│  └───────────────────────────┘ │
│  ┌───────────────────────────┐ │
│  │ "How do I fix go mod tidy │ │
│  │  behind a proxy?"         │ │
│  └───────────────────────────┘ │
│  ┌───────────────────────────┐ │
│  │ "What problems are queued │ │
│  │  right now?"              │ │
│  └───────────────────────────┘ │
│                                 │
│  ┌─────────────────────────┐   │
│  │ type a message...        │   │
│  └─────────────────────────┘   │
└─────────────────────────────────┘
```

---

## 8. Settings Panel

### 8.1 Layout

```
┌──────────────────────────────────────────────────┐
│  Settings                                     [✕]│
│                                                  │
│  Git Configuration                               │
│  ┌──────────────────────────────────────────────┐│
│  │ Default export repo:                         ││
│  │ git@github.com:wojons/off-by-one-answers.git ││
│  │ Default export branch: [main           ▾]    ││
│  │ SSH key path: [~/.ssh/id_ed25519]            ││
│  └──────────────────────────────────────────────┘│
│                                                  │
│  Community Repos                                 │
│  ┌──────────────────────────────────────────────┐│
│  │ git@github.com:wojons/off-by-one-answers.git ││
│  │ [Remove]                                     ││
│  │ + [Add community repo]                       ││
│  └──────────────────────────────────────────────┘│
│                                                  │
│  Sandbox                                         │
│  ┌──────────────────────────────────────────────┐│
│  │ Timeout: [5 minutes          ▾]              ││
│  │ Max concurrent solves: [1]                   ││
│  │ Idle threshold (load avg): [2.0]             ││
│  └──────────────────────────────────────────────┘│
│                                                  │
│  API Keys                                        │
│  ┌──────────────────────────────────────────────┐│
│  │ DeepSeek: sk-8a7d...8ae8   [Show] [Change]   ││
│  │ OpenRouter: sk-or-v1-dfd...b38 [Show] [Change]││
│  └──────────────────────────────────────────────┘│
│                                                  │
│  ┌──────────────┐                                │
│  │  💾 Save       │                                │
│  └──────────────┘                                │
└──────────────────────────────────────────────────┘
```

---

## 9. Status Bar

### 9.1 Content

```
Queue: 3 pending | Cache: 12 answers | Hit rate: 23% | Uptime: 4h 12m | v0.1.0
```

### 9.2 Refresh

- Auto-refresh every 30 seconds via `GET /api/v1/stats`
- Click "Queue: 3 pending" → jump to queue view (or filter search by queued)
- Click "Cache: 12 answers" → jump to taxonomy
- Click "Hit rate: 23%" → show mini trend chart in tooltip (last 7 days)

### 9.3 Hit Rate Tooltip

```
┌──────────────────────────┐
│  Hit Rate — Last 7 Days  │
│                          │
│  30% │        ▄▄█▄      │
│  20% │    ▄▄██▄██▄▄    │
│  10% │ ▄▄██████████▄   │
│      └────────────────  │
│      M T W T F S S      │
│                          │
│  Today: 23% (7/30)      │
│  Week avg: 22%          │
│  Target: >20% ✅        │
└──────────────────────────┘
```

---

## 10. Color System

```
┌─ Semantic ───────────────────────────────────────┐
│  Green  #22c55e  Success, verified, healthy      │
│  Blue   #3b82f6  Info, pending, links            │
│  Yellow #eab308  Warning, queued, version alerts  │
│  Orange #f97316  In progress, attention           │
│  Red    #ef4444  Error, failed, danger            │
│  Purple #a855f7  Imported, community              │
├─ Neutral ────────────────────────────────────────┤
│  bg     #0f172a  Page background (dark)          │
│  surf   #1e293b  Card/surface background          │
│  bord   #334155  Borders, dividers                │
│  text   #f1f5f9  Primary text                     │
│  mute   #64748b  Secondary text, placeholders     │
├─ Code ───────────────────────────────────────────┤
│  code   #0d1117  Code block background            │
│  syn    #c9d1d9  Code syntax (GitHub-dark theme)  │
└──────────────────────────────────────────────────┘
```

---

## 11. Keyboard Shortcuts

| Key | Scope | Action |
|-----|-------|--------|
| `/` | Global | Focus search bar |
| `Esc` | Global | Clear search, close modals, collapse expanded result |
| `1`–`5` | Global | Switch to tab 1–5 (Search, Submit, Explore, Export, Import) |
| `↓` / `↑` | Search results | Navigate between results |
| `Enter` | Search results | Expand selected result |
| `Ctrl+Enter` | Submit form, chat input | Submit |
| `Ctrl+E` | Global | Toggle export panel |
| `Ctrl+I` | Global | Toggle import panel |
| `Ctrl+Shift+C` | Global | Toggle chat sidebar |

---

## 12. Component Tree (for implementation)

```
index.html
├── #shell
│   ├── #header
│   │   ├── .logo ("Off-by-One")
│   │   └── #settings-btn (⚙)
│   ├── #nav
│   │   ├── .tab[data-tab="search"]  (🔍 Search)
│   │   ├── .tab[data-tab="submit"]  (📤 Submit)
│   │   ├── .tab[data-tab="explore"] (🌳 Explore)
│   │   ├── .tab[data-tab="export"]  (📦 Export)
│   │   └── .tab[data-tab="import"]  (📥 Import)
│   ├── #main
│   │   ├── #view-search   (display: none unless active)
│   │   ├── #view-submit   (display: none unless active)
│   │   ├── #view-explore  (display: none unless active)
│   │   ├── #view-export   (display: none unless active)
│   │   └── #view-import   (display: none unless active)
│   ├── #chat-sidebar
│   │   ├── #chat-toggle (« / »)
│   │   ├── #chat-messages
│   │   ├── #chat-suggestions (empty state)
│   │   └── #chat-input
│   │       ├── input[type="text"]
│   │       └── button[type="submit"]
│   ├── #status-bar
│   └── #modals (portal)
│       ├── #modal-related-graph
│       ├── #modal-conflict-resolution
│       └── #modal-hit-rate-chart
├── style.css
├── app.js (router, tab switching, keyboard shortcuts)
├── search.js
├── submit.js
├── explore.js
├── export.js
├── import.js
├── chat.js
└── d3.min.js (loaded from CDN or bundled)
```
