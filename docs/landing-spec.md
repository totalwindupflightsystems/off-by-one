# Off-by-One — Landing Page Specification

> **Version:** 1.0.0  
> **Target file:** `docs/index.html`  
> **Constraint:** Single HTML file, embedded CSS, zero JavaScript dependencies  
> **Status:** Specification — ready for implementation  

---

## Table of Contents

1. [Design Principles](#1-design-principles)
2. [Color Palette & Theming](#2-color-palette--theming)
3. [Typography](#3-typography)
4. [Layout Grid](#4-layout-grid)
5. [Responsive Breakpoints](#5-responsive-breakpoints)
6. [Animation & Motion](#6-animation--motion)
7. [Section Specifications](#7-section-specifications)
   - [7.1 Navbar](#71-navbar)
   - [7.2 Hero Section](#72-hero-section)
   - [7.3 Stats Bar](#73-stats-bar)
   - [7.4 How It Works](#74-how-it-works)
   - [7.5 Supported Problem Types](#75-supported-problem-types)
   - [7.6 Tech Stack](#76-tech-stack)
   - [7.7 Quick Start](#77-quick-start)
   - [7.8 Footer](#78-footer)
8. [Component Specifications](#8-component-specifications)
9. [Dark/Light Mode Toggle](#9-darklight-mode-toggle)
10. [HTML Document Structure](#10-html-document-structure)
11. [Implementation Checklist](#11-implementation-checklist)

---

## 1. Design Principles

| Principle | Description |
|-----------|-------------|
| **Zero-dependency** | No JS frameworks, no CSS libraries, no external fonts, no CDN links. Everything embedded. |
| **Progressive enhancement** | Core content and styling work without JS. The theme toggle is the only JS — and it degrades gracefully (page defaults to dark mode, `prefers-color-scheme` respected when available). |
| **Dark-first, light-capable** | Dark mode is the default and primary design target. Light mode is a polished alternate — not an afterthought. |
| **Content-driven** | Information hierarchy guides the user from "what is this?" → "how does it work?" → "can it solve my problem?" → "how do I start?". |
| **GitHub-aesthetic with personality** | Dark mode draws from GitHub's primer design system (#0d1117, #161b22, #30363d) but the colored wordmark and status indicators add distinct identity. |
| **Single-page, single-file** | All sections live on one page. No routing, no partials, no build step. One HTML file with a `<style>` block. |

---

## 2. Color Palette & Theming

### 2.1 Dark Mode (default)

```
┌─────────────────────────────────────────────────────────────┐
│ Role           │ Hex       │ CSS Variable      │ Usage      │
├─────────────────────────────────────────────────────────────┤
│ Background     │ #0d1117   │ --bg-primary      │ Page bg    │
│ Card bg        │ #161b22   │ --bg-card         │ Cards, stats│
│ Card hover     │ #1c2129   │ --bg-card-hover   │ Card hover │
│ Border         │ #30363d   │ --border          │ Card borders│
│ Border subtle  │ #21262d   │ --border-subtle   │ Divider hr │
│ Text primary   │ #e6edf3   │ --text-primary    │ Body text  │
│ Text secondary │ #8b949e   │ --text-secondary  │ Descriptions│
│ Text tertiary  │ #6e7681   │ --text-tertiary   │ Meta, labels│
│ Accent         │ #58a6ff   │ --accent          │ Links, CTAs│
│ Accent hover   │ #79c0ff   │ --accent-hover    │ Link hover │
│ Green/success  │ #3fb950   │ --green           │ Stats, hits│
│ Green bg       │ #033a16   │ --green-bg        │ Stat bg    │
│ Orange/warn    │ #d2991d   │ --orange          │ "By" in logo│
│ Orange bg      │ #3d2300   │ --orange-bg       │ Accent bg  │
│ Red/error      │ #f85149   │ --red             │ Errors     │
│ Purple         │ #a371f7   │ --purple          │ Accent var │
│ Code bg        │ #0d1117   │ --code-bg         │ Code blocks│
│ Code text      │ #c9d1d9   │ --code-text       │ Code       │
│ Code accent    │ #58a6ff   │ --code-accent     │ Code hl    │
│ Shadow         │ rgba(0,0,0,0.4) │ --shadow    │ Card shadow│
└─────────────────────────────────────────────────────────────┘
```

### 2.2 Light Mode

```
┌─────────────────────────────────────────────────────────────┐
│ Role           │ Hex       │ CSS Variable      │ Usage      │
├─────────────────────────────────────────────────────────────┤
│ Background     │ #ffffff   │ --bg-primary      │ Page bg    │
│ Card bg        │ #f6f8fa   │ --bg-card         │ Cards, stats│
│ Card hover     │ #eaeef2   │ --bg-card-hover   │ Card hover │
│ Border         │ #d0d7de   │ --border          │ Card borders│
│ Border subtle  │ #d8dee4   │ --border-subtle   │ Divider hr │
│ Text primary   │ #1f2328   │ --text-primary    │ Body text  │
│ Text secondary │ #656d76   │ --text-secondary  │ Descriptions│
│ Text tertiary  │ #57606a   │ --text-tertiary   │ Meta, labels│
│ Accent         │ #0969da   │ --accent          │ Links, CTAs│
│ Accent hover   │ #0550ae   │ --accent-hover    │ Link hover │
│ Green/success  │ #1a7f37   │ --green           │ Stats, hits│
│ Green bg       │ #dafbe1   │ --green-bg        │ Stat bg    │
│ Orange/warn    │ #9a6700   │ --orange          │ "By" in logo│
│ Orange bg      │ #fff8c5   │ --orange-bg       │ Accent bg  │
│ Red/error      │ #cf222e   │ --red             │ Errors     │
│ Purple         │ #8250df   │ --purple          │ Accent var │
│ Code bg        │ #f6f8fa   │ --code-bg         │ Code blocks│
│ Code text      │ #1f2328   │ --code-text       │ Code       │
│ Code accent    │ #0550ae   │ --code-accent     │ Code hl    │
│ Shadow         │ rgba(31,35,40,0.08) │ --shadow│ Card shadow│
└─────────────────────────────────────────────────────────────┘
```

### 2.3 CSS Custom Properties Strategy

Define all colors as custom properties on `:root`. Light mode overrides go in a `[data-theme="light"]` selector or a `.light` class on `<html>`. This way a single `<style>` block handles both themes without duplication.

```css
:root {
  /* Dark mode defaults */
  --bg-primary: #0d1117;
  --bg-card: #161b22;
  /* ... all vars ... */
}

[data-theme="light"] {
  --bg-primary: #ffffff;
  --bg-card: #f6f8fa;
  /* ... overrides ... */
}
```

The `<html>` element gets `data-theme="dark"` by default. A minimal JS snippet (see §9) toggles the attribute and persists to `localStorage`.

---

## 3. Typography

### 3.1 Font Stack

```css
font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Noto Sans',
             Helvetica, Arial, sans-serif, 'Apple Color Emoji',
             'Segoe UI Emoji';
```

This is the system font stack — no web font loading, zero latency, native rendering on every platform. Matches GitHub's approach.

### 3.2 Type Scale

| Token | Size | Weight | Line Height | Usage |
|-------|------|--------|-------------|-------|
| `--fs-hero` | `clamp(2.5rem, 6vw, 4.5rem)` | 700 | 1.1 | Hero H1 |
| `--fs-h2` | `clamp(1.75rem, 3.5vw, 2.5rem)` | 600 | 1.3 | Section titles |
| `--fs-h3` | `1.125rem` | 600 | 1.4 | Card titles |
| `--fs-body` | `1rem` | 400 | 1.6 | Body text |
| `--fs-body-lg` | `1.125rem` | 400 | 1.6 | Hero subtitle |
| `--fs-stat` | `clamp(2rem, 4vw, 3rem)` | 700 | 1.0 | Stat numbers |
| `--fs-stat-label` | `0.8125rem` | 500 | 1.3 | Stat labels |
| `--fs-code` | `0.875rem` | 400 | 1.5 | Code blocks |
| `--fs-small` | `0.8125rem` | 400 | 1.5 | Footer, meta |
| `--fs-caption` | `0.75rem` | 400 | 1.4 | Tags, badges |

### 3.3 Typography Rules

- **No italic** for body text — only for code comments.
- **Letter spacing:** `-0.02em` on hero H1, `0.02em` on stat labels (uppercase).
- **Stat labels** are uppercase (`text-transform: uppercase; letter-spacing: 0.08em`).
- **Monospace for code:** `'SF Mono', 'Fira Code', 'Fira Mono', Menlo, Consolas, monospace`.
- **No `font-weight: 300` or `200`** — too thin on non-Retina screens.

---

## 4. Layout Grid

### 4.1 Container

```css
.container {
  max-width: 1100px;
  margin: 0 auto;
  padding: 0 24px;
}
```

A single container class for all sections. No nested containers. Sections that need full-bleed backgrounds (hero, footer) get their own padding inside.

### 4.2 Grid Systems

**Two-column (How It Works, Supported Problems):**
```css
grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
gap: 24px;
```
- At 1100px container: 3 columns (260px × 3 + 48px gaps = 828px)
- At 768px: 2 columns
- At 480px: 1 column

**Four-column (Stats Bar):**
```css
grid-template-columns: repeat(4, 1fr);
gap: 20px;
```
- At 768px: 2 columns
- At 480px: 2 columns (tighter)

**Three-column (Tech Stack):**
```css
grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
gap: 20px;
```

### 4.3 Section Spacing

| Section | Padding (top/bottom) | Notes |
|---------|---------------------|-------|
| Hero | `100px 24px 80px` | Generous top to push content below any fixed nav |
| Stats Bar | `0` | Sits between hero and content, visually bridges the gap |
| Content section | `80px 0` | Consistent rhythm |
| Divider between sections | `1px solid var(--border-subtle)` | Optional, only between major content groupings |
| Quick Start | `80px 0` | Standard |
| Footer | `48px 24px` | Compact, content-light |

---

## 5. Responsive Breakpoints

| Breakpoint | Width | Layout Changes |
|------------|-------|----------------|
| **XL** | ≥ 1100px | Full container width, 3-column cards, 4-column stats |
| **LG** | 900–1099px | 3-column cards, 4-column stats |
| **MD** | 600–899px | 2-column cards, 2-column stats, smaller hero text |
| **SM** | 400–599px | 1-column cards, 2-column stats, tighter padding, stacked CTA buttons |
| **XS** | < 400px | Single column everything, reduced padding, code blocks horizontal-scroll |

### 5.1 Mobile-Specific Rules

- **Hero H1** shrinks via `clamp()` — no manual override needed.
- **CTA buttons** stack vertically below 480px (`flex-direction: column`).
- **Stats grid** goes 2×2 then stays 2×2 even at smallest sizes (never 1×1 — each stat is narrow enough).
- **Code blocks** get `overflow-x: auto` and `-webkit-overflow-scrolling: touch`.
- **Cards** lose `box-shadow` on mobile (performance, plus card borders already define edges).
- **Container padding** drops to `16px` below 480px.

---

## 6. Animation & Motion

### 6.1 Principles

- **CSS-only.** No JavaScript animations. Everything uses `@keyframes`, `transition`, and `animation`.
- **Subtle and purposeful.** No bouncing, no spinning, no parallax.
- **Respects `prefers-reduced-motion`.** All animations disabled when the user's OS says so.
- **Performance-first.** Only animate `opacity` and `transform` (GPU-composited properties).

### 6.2 Named Animations

#### a) Section Reveal (fade-up)

Cards and section content fade in as they scroll into view. Implemented via **scroll-driven animations** if browser support is acceptable, otherwise just a simple fade-up on page load with staggered delays.

```css
@keyframes fade-up {
  from {
    opacity: 0;
    transform: translateY(24px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.card {
  animation: fade-up 0.5s ease-out both;
}
.card:nth-child(1) { animation-delay: 0.05s; }
.card:nth-child(2) { animation-delay: 0.1s; }
.card:nth-child(3) { animation-delay: 0.15s; }
/* ... etc */
```

**Note:** This is the one animation that runs on page load. If removed, the page still looks complete — it just appears instantly instead of fading in. This is intentional: the animation is an enhancement, not a requirement.

#### b) Card Hover Lift

```css
.card {
  transition: transform 0.15s ease, box-shadow 0.15s ease, border-color 0.15s ease;
}
.card:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 24px var(--shadow);
  border-color: var(--accent);
}
```

The hover lift is 2px — enough to feel responsive, not enough to feel gimmicky.

#### c) Stat Number "Count Up" (Static)

No actual counting animation (that would require JS). Instead, stat numbers get a subtle color pulse once on page load:

```css
@keyframes stat-glow {
  0%, 100% { color: var(--green); }
  50% { color: var(--accent); }
}
.stat .number {
  animation: stat-glow 2s ease-in-out 1;
}
```

#### d) Pipeline Flow (How It Works visual)

If a visual pipeline strip is included (arrow-connected steps), use a subtle shimmer animation on the arrows:

```css
@keyframes flow-shimmer {
  0% { opacity: 0.3; }
  50% { opacity: 0.7; }
  100% { opacity: 0.3; }
}
.pipeline-arrow {
  animation: flow-shimmer 3s ease-in-out infinite;
}
```

### 6.3 Transition Defaults

```css
a, button, .card, .btn-primary, .btn-secondary, .stat {
  transition: all 0.15s ease;
}
```

### 6.4 Reduced Motion

```css
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.01ms !important;
  }
}
```

---

## 7. Section Specifications

### 7.1 Navbar

**Purpose:** Minimal top bar with logo/name, theme toggle, and GitHub link. Fixed position.

```
┌──────────────────────────────────────────────────────────────────┐
│  Off-by-One                              [☀/🌙]  [☆ GitHub]    │
└──────────────────────────────────────────────────────────────────┘
```

- **Position:** `position: sticky; top: 0; z-index: 100;`
- **Height:** 56px
- **Background:** `var(--bg-primary)` with `backdrop-filter: blur(12px)` and `border-bottom: 1px solid var(--border-subtle)`
- **Left:** Project name "Off-by-One" in the colored wordmark style (Off=accent, By=orange, One=green), smaller than hero (1.25rem, weight 600)
- **Right:** Theme toggle icon button (☀ in dark mode, 🌙 in light mode) + GitHub link with star icon
- **Mobile (< 600px):** GitHub link text hides, only icon shows
- **No hamburger menu** — single-page, no navigation items to hide

### 7.2 Hero Section

**Purpose:** First impression. Communicate what Off-by-One is in 5 seconds.

```
┌──────────────────────────────────────────────────────────────────┐
│                                                                   │
│                      Off-By-One                                  │
│              Convert idle GPU time into                          │
│              pre-verified answers for AI agents                   │
│                                                                   │
│    AI agents submit problems. During idle cycles, the lab         │
│    reproduces, sandbox-solves, and caches answers. Any agent      │
│    hitting the same problem later discovers the solution          │
│    instantly — no debugging from scratch.                         │
│                                                                   │
│          [▸ Get Started]    [☆ Star on GitHub]                    │
│                                                                   │
│              ┌──────────┐     ┌──────────┐     ┌──────────┐      │
│              │ 60+      │     │ 66+      │     │ 100%     │      │
│              │ Problems │     │ Answers  │     │ Hit Rate │      │
│              └──────────┘     └──────────┘     └──────────┘      │
│                                                                   │
└──────────────────────────────────────────────────────────────────┘
```

#### Wordmark (H1)

The project name "Off-by-One" is rendered as three colored spans:

```html
<h1>
  <span class="wordmark-off">Off</span>-<span class="wordmark-by">By</span>-<span class="wordmark-one">One</span>
</h1>
```

| Part | Color (dark) | Color (light) | Weight |
|------|-------------|---------------|--------|
| Off | `var(--accent)` | `var(--accent)` | 700 |
| — | `var(--text-secondary)` | `var(--text-secondary)` | 400 |
| By | `var(--orange)` | `var(--orange)` | 700 |
| — | `var(--text-secondary)` | `var(--text-secondary)` | 400 |
| One | `var(--green)` | `var(--green)` | 700 |

The colored wordmark is the primary brand identity. Each part has a distinct semantic color that reinforces the triple meaning (Off = cool/technical, By = caution/transition, One = success/arrival).

#### Subtitle (H2-style)

The value proposition line(s). Two-line treatment:
- Line 1: "Convert idle GPU time into" (weight 400, `var(--text-secondary)`)
- Line 2: "pre-verified answers for AI agents" (weight 600, `var(--text-primary)`)

Combined using `<br>` or two `<span>` elements inside a single `<p class="hero-subtitle">`.

#### Description

A single `<p>` paragraph, max-width 640px, centered. Explains the core loop in plain language. `var(--text-secondary)`.

#### CTA Buttons

Two buttons, side by side (stack on mobile below 480px):

| Button | Style | Link |
|--------|-------|------|
| Get Started | `.btn-primary` | Scrolls to `#quick-start` |
| Star on GitHub | `.btn-secondary` | `https://github.com/totalwindupflightsystems/off-by-one` |

- Button hover: `transform: translateY(-1px); box-shadow: 0 4px 12px var(--shadow);`
- Button active: `transform: translateY(0);`
- Both have `border-radius: 8px; padding: 14px 32px; font-weight: 600;`

#### Inline Stats (Hero Bottom)

A compact stats strip in the hero. These are a teaser — the full stats bar (§7.3) provides the prominent treatment.

```
┌──────────┐  ┌──────────┐  ┌──────────┐
│   60+    │  │   66+    │  │  100%    │
│ Problems │  │ Answers  │  │ Hit Rate │
└──────────┘  └──────────┘  └──────────┘
```

- Inline, flex row, centered, `gap: 32px`
- Number: `var(--fs-stat)`, `var(--green)`, weight 700
- Label: `var(--fs-caption)`, `var(--text-tertiary)`, uppercase
- Separator between stats: subtle vertical pipe `|` in `var(--border-subtle)`
- On mobile < 600px: wrap to 2 rows or reduce gap

### 7.3 Stats Bar

**Purpose:** Prominent, visually impactful stats display immediately after the hero. This is designed to grab attention and convey credibility through numbers.

```
┌──────────────────────────────────────────────────────────────────┐
│                                                                   │
│    ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│    │              │  │              │  │              │  │              │
│    │     60+      │  │     66+      │  │     100%     │  │      4       │
│    │   PROBLEMS   │  │   ANSWERS    │  │   HIT RATE   │  │  LANGUAGES   │
│    │   SOLVED     │  │   VERIFIED   │  │              │  │              │
│    │              │  │              │  │              │  │              │
│    └──────────────┘  └──────────────┘  └──────────────┘  └──────────────┘
│                                                                   │
└──────────────────────────────────────────────────────────────────┘
```

#### Visual Treatment

- **Background:** A subtle gradient or solid background that sits between content sections. Options:
  - A: `background: linear-gradient(180deg, var(--bg-primary) 0%, var(--bg-card) 50%, var(--bg-primary) 100%);`
  - B: `background: var(--bg-card); border-top: 1px solid var(--border); border-bottom: 1px solid var(--border);`
  
  **Recommended: B** — cleaner, more contained, works in both themes.

- **Stat cards:** No card boxes — the numbers float in the full-bleed bar. Each stat is a centered column.
- **Number color:** `var(--green)` — this is the brand's "proof" color. Success, verified, trustworthy.
- **Number size:** `var(--fs-stat)` — the largest type on the page after the hero H1.
- **Label treatment:** Uppercase, `var(--fs-stat-label)`, `var(--text-tertiary)`, `letter-spacing: 0.08em`.
- **Divider between stats:** A subtle 1px vertical line or just gap-based separation. Gap preferred (simpler, cleaner).

#### Layout

```css
.stats-bar {
  display: flex;
  justify-content: center;
  gap: 64px;
  padding: 48px 24px;
  background: var(--bg-card);
  border-top: 1px solid var(--border);
  border-bottom: 1px solid var(--border);
}

.stat-item {
  text-align: center;
  min-width: 120px;
}
```

- On mobile < 600px: `flex-wrap: wrap; gap: 32px;` — stats go 2×2
- On mobile < 400px: `gap: 24px;`

#### Animation

Each stat gets a staggered `fade-up` animation on load:
```css
.stat-item:nth-child(1) { animation-delay: 0.1s; }
.stat-item:nth-child(2) { animation-delay: 0.2s; }
.stat-item:nth-child(3) { animation-delay: 0.3s; }
.stat-item:nth-child(4) { animation-delay: 0.4s; }
```

### 7.4 How It Works

**Purpose:** Explain the 4-step pipeline. Each step is a card. Together they form a narrative flow: Submit → Solve → Cache → Discover.

```
┌──────────────────────────────────────────────────────────────────┐
│                     How It Works                                  │
│                                                                   │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐         │
│  │  📥      │  │  🏖️      │  │  💾      │  │  🔍      │         │
│  │  Submit  │  │  Solve   │  │  Cache   │  │ Discover │         │
│  │          │  │  (Idle)  │  │          │  │          │         │
│  │ AI agents│  │ GPU      │  │ SQLite   │  │ Any agent│         │
│  │ submit   │  │ solves in│  │ graph +  │  │ queries & │         │
│  │ problems │  │ sandbox  │  │ FTS5     │  │ retrieves │         │
│  │ via REST │  │ via Pi   │  │ storage  │  │ instantly │         │
│  │ API/MCP  │  │ Agent    │  │          │  │           │         │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘         │
│                                                                   │
│  ┌──────────────────────────────────────────────────────────────┐ │
│  │  Submit ──→ Queue ──→ Sandbox ──→ Pi Agent ──→ Graph ──→ Hit │ │
│  └──────────────────────────────────────────────────────────────┘ │
│                                                                   │
└──────────────────────────────────────────────────────────────────┘
```

#### Card Layout (4 columns)

Each card:
```html
<div class="card">
  <div class="card-icon">📥</div>
  <div class="card-step">Step 1</div>
  <h3 class="card-title">Submit</h3>
  <p class="card-desc">AI agents submit problems via REST API or Muster MCP bridge. Problems are queued with metadata: language, environment, version, and cadence.</p>
</div>
```

- **Icon:** Emoji at `font-size: 2.5rem; margin-bottom: 12px;` — large and recognisable. Alternative: SVG inline icons.
- **Step number:** Small label above the title: `STEP 1`, `STEP 2`, etc. `var(--text-tertiary)`, uppercase, `var(--fs-caption)`.
- **Title:** `var(--fs-h3)`, `var(--text-primary)`, weight 600.
- **Description:** `var(--fs-body)`, `var(--text-secondary)`, max 3 lines.

#### Pipeline Strip (arrow flow)

Below the cards, a visual pipeline showing the flow with arrows:

```
Submit  →  Queue  →  Sandbox  →  Pi Agent  →  Graph  →  Hit
```

- Horizontal flex row, centered, `gap: 8px`
- Each step is a small pill/badge: `background: var(--bg-card); border: 1px solid var(--border); border-radius: 20px; padding: 6px 16px; font-size: 0.8125rem;`
- Arrows between steps: `→` character in `var(--accent)`
- On mobile: `flex-wrap: wrap; justify-content: center;`
- The arrows get a subtle shimmer animation (see §6.2d)

### 7.5 Supported Problem Types

**Purpose:** Show the breadth of what Off-by-One can handle. Six cards in a 3×2 grid.

```
┌──────────────────────────────────────────────────────────────────┐
│                   What Can It Solve?                              │
│                                                                   │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐           │
│  │ 🐚 Shell/Bash│  │ 🐹 Go        │  │ 🐍 Python    │           │
│  │              │  │              │  │              │           │
│  │ Log analysis,│  │ Concurrent   │  │ Streaming    │           │
│  │ file process,│  │ data structs,│  │ parsers,     │           │
│  │ git ops,     │  │ TCP proxies, │  │ bloom filters│           │
│  │ checksum,    │  │ Raft, workers│  │ sliding      │           │
│  │ text xforms  │  │ pools, LRU   │  │ windows,     │           │
│  │              │  │ caches       │  │ async gen    │           │
│  └──────────────┘  └──────────────┘  └──────────────┘           │
│                                                                   │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐           │
│  │ 📜 JS/Node  │  │ 📐 Math/     │  │ 🗄️ SQL       │           │
│  │              │  │    Physics   │  │              │           │
│  │ Virtual DOM, │  │ Spectral thm,│  │ Recursive    │           │
│  │ async pipes, │  │ Schrödinger, │  │ CTEs, window │           │
│  │ recursive    │  │ time dilation│  │ functions,   │           │
│  │ descent,     │  │ Master thm,  │  │ query opt,   │           │
│  │ deep objects │  │ P⊆NP, IVT   │  │ complex joins│           │
│  └──────────────┘  └──────────────┘  └──────────────┘           │
│                                                                   │
└──────────────────────────────────────────────────────────────────┘
```

Each card:
- **Icon:** Language/domain emoji at `font-size: 2rem; margin-bottom: 10px;`
- **Title:** Language/domain name, `var(--fs-h3)`, weight 600
- **Body:** Comma-separated list of problem types. If short enough to fit 2-3 lines, show inline. If longer, show first items and end with "… and more"
- **Accent color bar:** A 3px top border in the card's accent color (different per card):

| Card | Accent Bar Color |
|------|-----------------|
| Shell/Bash | `#89e051` (shell green) |
| Go | `#00add8` (Go cyan) |
| Python | `#ffde57` / `#4584b6` (Python blue) |
| JavaScript/Node.js | `#f7df1e` (JS yellow) |
| Math/Physics | `var(--purple)` |
| SQL | `var(--orange)` |

Alternative: if colored top borders feel too busy, use a small colored dot or left-border accent instead. **Recommendation: 3px left border accent** — cleaner, works well with the card design.

### 7.6 Tech Stack

**Purpose:** Show that Off-by-One is built on a solid, modern stack. Six cards in a 3×2 or 2×3 grid.

```
┌──────────────────────────────────────────────────────────────────┐
│                       Tech Stack                                  │
│                                                                   │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐           │
│  │ 🔷 Go 1.26.5 │  │ 🗃️ SQLite   │  │ 📦 Bubblewrap│           │
│  │              │  │    + FTS5    │  │              │           │
│  │ Core server, │  │ Graph DB with│  │ Lightweight   │           │
│  │ API, graph   │  │ full-text    │  │ Linux sandbox │           │
│  │ engine,      │  │ search.      │  │ for isolated  │           │
│  │ sandbox.     │  │ Problem class│  │ solving. No   │           │
│  │ ~14K lines,  │  │ tree with    │  │ Docker needed │           │
│  │ 76% coverage │  │ edges.       │  │               │           │
│  └──────────────┘  └──────────────┘  └──────────────┘           │
│                                                                   │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐           │
│  │ 🤖 Pi Agent  │  │ 🔌 Muster    │  │ 🌐 Web UI    │           │
│  │              │  │    MCP       │  │              │           │
│  │ TypeScript   │  │ Bidirectional│  │ 6 views +    │           │
│  │ monorepo     │  │ bridge. Auto-│  │ WebSocket AI │           │
│  │ coding agent │  │ generates    │  │ Chat. Vanilla │           │
│  │ for sandbox  │  │ 12+ MCP tools│  │ JS, 7 modules│           │
│  │ solving      │  │ from OAS     │  │              │           │
│  └──────────────┘  └──────────────┘  └──────────────┘           │
│                                                                   │
└──────────────────────────────────────────────────────────────────┘
```

Each card:
- **Icon:** Tech emoji, `font-size: 2rem;`
- **Title:** Tech name + version/key detail, `var(--fs-h3)`, weight 600
- **Body:** 2-3 lines describing the role, `var(--text-secondary)`
- **Key metric** (where applicable): Lines of code, coverage %, tool count — displayed as a small badge/pill inline or below the description.

### 7.7 Quick Start

**Purpose:** Get developers running in 30 seconds. Code-first, minimal prose.

```
┌──────────────────────────────────────────────────────────────────┐
│                       Quick Start                                 │
│                                                                   │
│  ┌──────────────────────────────────────────────────────────────┐ │
│  │  # Clone                                                     │ │
│  │  git clone https://github.com/totalwindupflightsystems/      │ │
│  │           off-by-one.git                                     │ │
│  │  cd off-by-one                                               │ │
│  │                                                              │ │
│  │  # Set your API key                                          │ │
│  │  echo "DEEPSEEK_API_KEY=sk-your-key" > .env                  │ │
│  │                                                              │ │
│  │  # Build & run                                               │ │
│  │  go build ./cmd/off-by-one/                                   │ │
│  │  ./off-by-one -bwrap /usr/bin/bwrap -pi-agent pi-agent       │ │
│  │                                                              │ │
│  │  # Open web UI → http://localhost:8766                       │ │
│  └──────────────────────────────────────────────────────────────┘ │
│                                                                   │
│  Prerequisites: Go 1.25+, Bubblewrap (optional), Pi Agent        │
│                                                                   │
│          [▸ Full Documentation]    [☆ Star on GitHub]             │
│                                                                   │
└──────────────────────────────────────────────────────────────────┘
```

#### Code Block Styling

- **Background:** `var(--code-bg)`
- **Border:** `1px solid var(--border)`
- **Border radius:** `12px` (same as cards)
- **Padding:** `24px 28px`
- **Font:** monospace stack, `var(--fs-code)`
- **Line height:** 1.6 for readability
- **Comments** (lines starting with `#`): `var(--text-tertiary)`, italic
- **Commands:** `var(--code-text)`
- **Prompts/shell chars:** `var(--text-secondary)`
- **Horizontal scroll** on overflow (long URLs)
- **Copy button:** Optional. A small "Copy" text button in the top-right corner. Implemented via a `<details>` + `<summary>` element trick (no JS), or just omitted. **Recommendation: omit copy button** — keeps the zero-JS purity and users can triple-click to select.

#### Prerequisites Line

A small, muted line below the code block listing what's needed. `var(--fs-small)`, `var(--text-tertiary)`.

#### CTA Buttons (Secondary Row)

Same style as hero CTAs but smaller padding (`10px 24px`). Links to README/docs and GitHub.

### 7.8 Footer

**Purpose:** Close the page. Minimal, professional, with essential links.

```
┌──────────────────────────────────────────────────────────────────┐
│                                                                   │
│                         Off-by-One                                │
│                        Pre-Solve Lab                              │
│                                                                   │
│           GitHub · MIT License · Built with Go + DeepSeek         │
│                                                                   │
│    Related: Muster · Pi Agent · GitReins · Hilo                  │
│                                                                   │
└──────────────────────────────────────────────────────────────────┘
```

- **Background:** `var(--bg-primary)` (same as page), with `border-top: 1px solid var(--border-subtle)`
- **Padding:** `48px 24px`
- **Logo:** Small wordmark (Off-by-One in colored spans, `var(--fs-body-lg)`)
- **Tagline:** "Pre-Solve Lab" in `var(--text-tertiary)`, `var(--fs-small)`
- **Meta line:** Separated by `·` (middle dot), links in `var(--accent)`
- **Related projects:** A row of small text links, `var(--fs-caption)`, `var(--text-tertiary)`, separated by `·`
- **No columns** — everything centered, stacked vertically with `gap: 12px` between groups

---

## 8. Component Specifications

### 8.1 Card

```
┌────────────────────────────────────────────┐
│  ┌──┐ ← accent bar (3px left border)       │
│  │  │                                       │
│  │  │  📥  ← icon (2rem)                   │
│  │  │                                       │
│  │  │  STEP 1  ← step label (caption)       │
│  │  │  Submit  ← title (h3)                │
│  │  │                                       │
│  │  │  AI agents submit problems via        │
│  │  │  REST API or Muster MCP bridge.       │
│  │  │  Problems are queued with metadata.   │
│  │  │  ← description (body text)            │
│  └──┘                                       │
└────────────────────────────────────────────┘
```

```css
.card {
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 28px;
  border-left: 3px solid transparent; /* accent color applied per card */
  transition: transform 0.15s ease, box-shadow 0.15s ease, border-color 0.15s ease;
}

.card:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 24px var(--shadow);
  border-color: var(--accent);
}

.card-icon {
  font-size: 2rem;
  line-height: 1;
  margin-bottom: 16px;
  display: block;
}

.card-step {
  font-size: var(--fs-caption);
  color: var(--text-tertiary);
  text-transform: uppercase;
  letter-spacing: 0.06em;
  margin-bottom: 6px;
}

.card-title {
  font-size: var(--fs-h3);
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 8px;
}

.card-desc {
  font-size: var(--fs-body);
  color: var(--text-secondary);
  line-height: 1.55;
}
```

### 8.2 Stat Item

```css
.stat-item {
  text-align: center;
  min-width: 100px;
}

.stat-number {
  font-size: var(--fs-stat);
  font-weight: 700;
  color: var(--green);
  line-height: 1.1;
  margin-bottom: 6px;
}

.stat-label {
  font-size: var(--fs-stat-label);
  color: var(--text-tertiary);
  text-transform: uppercase;
  letter-spacing: 0.08em;
  font-weight: 500;
}
```

### 8.3 Button

```css
.btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 14px 32px;
  border-radius: 8px;
  font-weight: 600;
  font-size: var(--fs-body);
  text-decoration: none;
  cursor: pointer;
  transition: all 0.15s ease;
}

.btn-primary {
  background: var(--accent);
  color: #ffffff;
  border: none;
}

.btn-primary:hover {
  background: var(--accent-hover);
  transform: translateY(-1px);
  box-shadow: 0 4px 12px var(--shadow);
}

.btn-secondary {
  background: transparent;
  color: var(--text-primary);
  border: 1px solid var(--border);
}

.btn-secondary:hover {
  border-color: var(--accent);
  transform: translateY(-1px);
  box-shadow: 0 4px 12px var(--shadow);
}

.btn-sm {
  padding: 10px 24px;
  font-size: var(--fs-small);
}
```

### 8.4 Section Title

```css
.section-title {
  font-size: var(--fs-h2);
  font-weight: 600;
  color: var(--text-primary);
  text-align: center;
  margin-bottom: 40px;
  letter-spacing: -0.01em;
}

.section-title::after {
  content: '';
  display: block;
  width: 48px;
  height: 3px;
  background: var(--accent);
  margin: 12px auto 0;
  border-radius: 2px;
}
```

A small accent underline below each section title reinforces the brand color and adds visual rhythm.

### 8.5 Code Block

```css
.code-block {
  background: var(--code-bg);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 24px 28px;
  font-family: 'SF Mono', 'Fira Code', 'Fira Mono', Menlo, Consolas, monospace;
  font-size: var(--fs-code);
  line-height: 1.65;
  color: var(--code-text);
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
  white-space: pre;
  max-width: 100%;
}

.code-block .comment {
  color: var(--text-tertiary);
  font-style: italic;
}

.code-block .prompt {
  color: var(--text-secondary);
}

.code-block .command {
  color: var(--accent);
}
```

The code block uses `<pre><code>` with `<span>` elements for syntax highlighting. Minimal: only comments (`#` lines), prompts (`$`), and commands get distinct colors.

### 8.6 Pipeline Strip

```css
.pipeline {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-top: 36px;
}

.pipeline-step {
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 20px;
  padding: 6px 16px;
  font-size: var(--fs-small);
  font-weight: 500;
  color: var(--text-primary);
  white-space: nowrap;
}

.pipeline-arrow {
  color: var(--accent);
  font-size: var(--fs-body-lg);
  font-weight: 600;
  user-select: none;
}
```

### 8.7 Theme Toggle Button

```css
.theme-toggle {
  background: none;
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 6px 10px;
  cursor: pointer;
  font-size: 1.2rem;
  line-height: 1;
  color: var(--text-secondary);
  transition: all 0.15s ease;
}

.theme-toggle:hover {
  border-color: var(--accent);
  color: var(--text-primary);
}
```

### 8.8 Accent Bar Colors (Per-Card)

Applied via inline style or utility classes:

| Card | Left Border Color |
|------|-------------------|
| Submit | `var(--accent)` (#58a6ff) |
| Solve | `var(--orange)` (#d2991d) |
| Cache | `var(--green)` (#3fb950) |
| Discover | `var(--purple)` (#a371f7) |
| Shell/Bash | `#89e051` |
| Go | `#00add8` |
| Python | `#4584b6` |
| JavaScript/Node.js | `#f7df1e` |
| Math/Physics | `var(--purple)` |
| SQL | `var(--orange)` |

Tech stack cards: all use `var(--accent)` as the left border.

---

## 9. Dark/Light Mode Toggle

### 9.1 Default Behavior

- Page loads in **dark mode** by default (`<html data-theme="dark">`)
- If the user has a saved preference in `localStorage`, use that
- If the user's OS has `prefers-color-scheme: light`, respect that
- The toggle button switches themes and persists the choice

### 9.2 JavaScript (Minimal, Embedded)

```html
<script>
(function() {
  var html = document.documentElement;
  var saved = localStorage.getItem('off-by-one-theme');

  function applyTheme(t) {
    html.setAttribute('data-theme', t);
    localStorage.setItem('off-by-one-theme', t);
  }

  // Determine initial theme
  if (saved) {
    applyTheme(saved);
  } else if (window.matchMedia('(prefers-color-scheme: light)').matches) {
    applyTheme('light');
  } else {
    applyTheme('dark');
  }

  // Toggle on button click
  document.addEventListener('DOMContentLoaded', function() {
    var btn = document.getElementById('theme-toggle');
    if (btn) {
      btn.addEventListener('click', function() {
        var current = html.getAttribute('data-theme');
        applyTheme(current === 'dark' ? 'light' : 'dark');
      });
    }
  });

  // Listen for OS theme changes
  window.matchMedia('(prefers-color-scheme: light)').addEventListener('change', function(e) {
    if (!localStorage.getItem('off-by-one-theme')) {
      applyTheme(e.matches ? 'light' : 'dark');
    }
  });
})();
</script>
```

This is the **only JavaScript** on the page. It's self-contained, IIFE-wrapped, and handles:
1. Initial load (saved pref > OS pref > dark default)
2. Toggle button click
3. OS theme changes (only if user hasn't manually picked)

### 9.3 Toggle Icon

| Theme | Icon | Character |
|-------|------|-----------|
| Dark (moon shown) | ☀️ | `&#x2600;&#xFE0F;` (sun, indicating "switch to light") |
| Light (sun shown) | 🌙 | `&#x1F319;` (moon, indicating "switch to dark") |

The icon shows what you'll switch TO, not what you're currently ON. This is the standard UX pattern.

### 9.4 Transition Between Themes

```css
html {
  transition: background-color 0.3s ease, color 0.3s ease;
}

html *, html *::before, html *::after {
  transition: background-color 0.3s ease, border-color 0.3s ease, color 0.2s ease, box-shadow 0.3s ease;
}
```

A smooth 300ms fade between themes. Disabled when `prefers-reduced-motion` is active.

---

## 10. HTML Document Structure

```html
<!DOCTYPE html>
<html lang="en" data-theme="dark">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <meta name="description" content="Off-by-One: Convert idle GPU time into pre-verified answers for AI agents. A Go-based pre-solve lab.">
  <meta name="color-scheme" content="dark light">
  <title>Off-by-One — Pre-Solve Lab</title>
  <style>
    /* === CSS Custom Properties (Dark Mode) === */
    :root { ... }

    /* === Light Mode Overrides === */
    [data-theme="light"] { ... }

    /* === Reset === */
    ...

    /* === Layout === */
    ...

    /* === Components === */
    ...

    /* === Animations === */
    ...

    /* === Reduced Motion === */
    ...

    /* === Responsive === */
    ...
  </style>
</head>
<body>

  <!-- Navbar -->
  <nav class="navbar">...</nav>

  <!-- Hero Section -->
  <section class="hero" id="hero">...</section>

  <!-- Stats Bar -->
  <section class="stats-bar" id="stats">...</section>

  <!-- How It Works -->
  <section class="section" id="how-it-works">...</section>

  <!-- Supported Problem Types -->
  <section class="section" id="supported">...</section>

  <!-- Tech Stack -->
  <section class="section" id="tech-stack">...</section>

  <!-- Quick Start -->
  <section class="section" id="quick-start">...</section>

  <!-- Footer -->
  <footer class="footer">...</footer>

  <!-- Theme Toggle Script -->
  <script>...</script>

</body>
</html>
```

### 10.1 Meta Tags

| Tag | Value | Purpose |
|-----|-------|---------|
| `description` | "Convert idle GPU time into pre-verified answers for AI agents. A Go-based pre-solve lab." | SEO, link previews |
| `color-scheme` | `dark light` | Tells browser both themes are supported (prevents auto-dark-mode overrides) |
| `viewport` | `width=device-width, initial-scale=1.0` | Mobile responsiveness |
| `charset` | `UTF-8` | Character encoding |

### 10.2 Open Graph (Optional)

If social sharing is desired:

```html
<meta property="og:title" content="Off-by-One — Pre-Solve Lab">
<meta property="og:description" content="Convert idle GPU time into pre-verified answers for AI agents.">
<meta property="og:type" content="website">
<meta property="og:url" content="https://github.com/totalwindupflightsystems/off-by-one">
```

---

## 11. Implementation Checklist

### Phase 1: Structure & Theming

- [ ] HTML5 document shell with all meta tags
- [ ] CSS custom properties for dark mode (`:root`)
- [ ] CSS custom properties override for light mode (`[data-theme="light"]`)
- [ ] CSS reset (margin, padding, box-sizing)
- [ ] Base typography (font stack, type scale, headings)
- [ ] Layout utilities (`.container`, `.section`, spacing)
- [ ] Theme toggle button HTML
- [ ] Theme toggle JavaScript (embedded, IIFE)

### Phase 2: Components

- [ ] `.card` component
- [ ] `.stat-item` component
- [ ] `.btn` / `.btn-primary` / `.btn-secondary` / `.btn-sm`
- [ ] `.section-title` component with accent underline
- [ ] `.code-block` with basic syntax highlighting
- [ ] `.pipeline` / `.pipeline-step` / `.pipeline-arrow`
- [ ] `.navbar` with sticky positioning
- [ ] `.footer`

### Phase 3: Sections

- [ ] Navbar (logo + theme toggle + GitHub link)
- [ ] Hero section (wordmark, subtitle, description, CTAs, inline stats)
- [ ] Stats bar (4 stats, full-bleed background)
- [ ] How It Works (4 cards + pipeline strip)
- [ ] Supported Problem Types (6 cards with accent borders)
- [ ] Tech Stack (6 cards)
- [ ] Quick Start (code block + prerequisites + CTAs)
- [ ] Footer (logo, links, related projects)

### Phase 4: Polish

- [ ] Animations (fade-up on cards, hover lift, pipeline shimmer, stat glow)
- [ ] `prefers-reduced-motion` media query
- [ ] Responsive breakpoints (all 5)
- [ ] Mobile testing (text sizing, card stacking, CTA stacking, code overflow)
- [ ] Light mode visual pass (all sections)
- [ ] Dark mode visual pass (all sections)
- [ ] Theme transition smoothness
- [ ] `color-scheme` meta tag for browser cooperation
- [ ] Accessibility: focus styles, `prefers-contrast`, keyboard navigation
- [ ] Validate HTML (no unclosed tags, valid attributes)
- [ ] Test in Firefox and Chrome

### Phase 5: Deployment

- [ ] Merge into `docs/index.html`
- [ ] GitHub Pages auto-deploys from `/docs` on master
- [ ] Verify at `https://totalwindupflightsystems.github.io/off-by-one`

---

## Appendix A: Color Accessibility Notes

- Dark mode text-primary (`#e6edf3`) on bg-primary (`#0d1117`): contrast ratio ~15.3:1 (AAA)
- Dark mode text-secondary (`#8b949e`) on bg-primary (`#0d1117`): contrast ratio ~5.9:1 (AA)
- Light mode text-primary (`#1f2328`) on bg-primary (`#ffffff`): contrast ratio ~12.6:1 (AAA)
- Accent (`#58a6ff`) on dark bg (`#0d1117`): contrast ratio ~4.6:1 (AA large text)
- Accent (`#0969da`) on light bg (`#ffffff`): contrast ratio ~4.5:1 (AA large text)
- Green stat numbers (`#3fb950`) on dark bg: contrast ratio ~5.1:1 (AA)
- Green stat numbers (`#1a7f37`) on light bg: contrast ratio ~4.7:1 (AA large text)

All primary text combinations meet WCAG AA or AAA. The stat number green is borderline on light mode — if accessibility is critical, darken the light-mode green to `#116329`.

## Appendix B: File Size Budget

| Component | Estimated Size |
|-----------|---------------|
| HTML structure | ~3 KB |
| CSS (theming + components) | ~8 KB |
| JS (theme toggle) | ~1 KB |
| Inline comments/whitespace | ~1 KB |
| **Total** | **~13 KB uncompressed** |

Gzipped: ~4 KB. Well under any reasonable budget.

## Appendix C: Browser Support

| Browser | Min Version | Notes |
|---------|------------|-------|
| Chrome | 90+ | Full support |
| Firefox | 90+ | Full support |
| Safari | 14+ | Full support (CSS custom properties, `prefers-reduced-motion`, `prefers-color-scheme` all supported) |
| Edge | 90+ | Full support (Chromium-based) |
| Mobile Safari | 14+ | Full support, `-webkit-overflow-scrolling: touch` for code blocks |

No IE11 support. No polyfills needed. The single JS feature used (`classList`, `addEventListener`, `localStorage`, `matchMedia`) has 97%+ global support.
