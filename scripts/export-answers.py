#!/usr/bin/env python3
"""
Export the verified answer corpus from SQLite into flat files for distribution.

Outputs (relative to repo root):
  data/answers.jsonl            — master bulk file, one answer per line (JSON)
  data/answers/<slug>.json      — one file per problem class (PR-friendly)
  data/INDEX.md                 — human-readable catalog of problem classes
  data/COUNTS.md                — live corpus counts (auto-stamped every export)
  data/README.md                — usage guide for consumers

The flat files ARE the distribution layer: teams clone the repo (or fetch a
single file), browse, and contribute new answers via PR — no server required.
SQLite remains the operational store for the live server.

Usage:
  python3 scripts/export-answers.py [path-to-off-by-one.db]

Privacy hygiene (PUBLIC-PII-001): this repo's corpus is public, and agents
can be helpful enough to copy real workstation details into an answer. The
export applies common PII substitutions by default: personal home roots from
Linux/macOS/Windows paths become `~`; email, phone, IPv4 and IPv6 literals
become `<email>`, `<phone>` and `<ip-address>`. The schema and surrounding
technical text stay intact. A separate fail-closed corpus/site guard checks
the generated public artifacts; gitleaks remains the credential scanner and
is not treated as a general PII detector.
"""
import ipaddress
import json
import os
import re
import sqlite3
import sys
from datetime import datetime, timezone

DB_PATH = sys.argv[1] if len(sys.argv) > 1 else os.path.join(
    os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "off-by-one.db"
)
REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
DATA_DIR = os.path.join(REPO_ROOT, "data")
ANSWERS_DIR = os.path.join(DATA_DIR, "answers")

# Self-test / canary / probe classes that must never reach the public export.
# ANCHORED matching (DF-OFF-BY-ONE-15): probe families are prefix families,
# so patterns pin ^ prefixes, exact ^...$ titles, or separator-delimited
# tokens — the same anchored strings as placeholderClassPatterns in
# internal/graph/placeholder.go (keep the two lists in sync). A bare
# substring search 404ed real engineering classes whose titles merely
# CONTAIN a protected word mid-slug (e.g. "ob1-dogfood-fib-off-by-one-index",
# "ob1-ctx-canary-deploy-oom"). Keep these specific: real engineering
# classes whose titles merely contain "test" (e.g.
# "test-mocking-http-requests", "test-property-based-shrinking") must NOT be
# excluded.
EXCLUDED_CLASS_PATTERNS = [
    r"^off-by-one-self-test",                # off-by-one-self-test-* family
    r"(?:^|[-_])self[-_]test(?:[-_]|$)",     # self-test token (fused "selftest" is a real word)
    r"^self[-_]dogfood",                     # self-dogfood probes
    r"^test-self-dogfood",                   # test-self-dogfood
    r"^dogfood-field-test",                  # dogfood-field-test-* probe prefix
    r"^dogfood$",                            # bare placeholder row (exact)
    r"^docs-canary",                         # docs-canary-* probe prefix
    r"^canary$",                             # bare placeholder row (exact)
    r"^test$",                               # bare "test" placeholder class
    r"^test-gap-sweep",
    r"^test-foreman-",
    r"^e2e-tick",                            # e2e-tickNN probe classes
    r"tick\d+-e2e\b",                        # reversed-form probes: foreman-tick82-e2e, tick89-e2e, tick90-e2e
    r"^shell-script-e2e$",                   # 0001-0017-era probe class (exact)
    r"^e2e-verification-pipeline$",          # e2e-verification-pipeline probe (exact)
    r"^foreman-e2e-verification-pipeline$",  # foreman-e2e-verification-pipeline probe (exact)
    r"(?:^|[-_])foreman[-_]audit(?:[-_]|$)", # tick88-foreman-audit probe (token mid-slug)
    r"^ds-007",                              # DS-007 probe family (ds-007, ds-007-tick-106)
    r"^shell-say-hello-test$",
    r"^shell-echo-hello-fix$",
]
_EXCLUDED_RES = [re.compile(p, re.IGNORECASE) for p in EXCLUDED_CLASS_PATTERNS]

# PUBLIC-PII-001: basic built-in PII redaction for the public corpus. Gitleaks
# scans credentials, but the corpus is allowlisted for example secrets; these
# separate transforms protect common personal artifacts across all text fields.
# Keep the equivalent checks in scripts/check-public-pii.py and Go export.
_HOST_PATH_RE = re.compile(
    r"(?i)(?:\\*/home|\\*/users|\\*/mnt/[a-z]/users|"
    r"\\*/cygdrive/[a-z]/users|[a-z]:[/\\]+users|"
    r"\\*/documents and settings)[/\\]+[A-Za-z0-9._-]+"
)
_EMAIL_RE = re.compile(r"(?i)\b[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}\b")
_PHONE_RE = re.compile(
    r"(?<![0-9])(?:\+?[0-9]{1,3}[ .-]?)?(?:\([0-9]{3}\)|[0-9]{3})"
    r"[ .-][0-9]{3}[ .-][0-9]{4}(?![0-9])"
)
_IPV4_RE = re.compile(
    r"(?<![0-9.])(?:25[0-5]|2[0-4][0-9]|1[0-9]{2}|[1-9]?[0-9])"
    r"(?:\.(?:25[0-5]|2[0-4][0-9]|1[0-9]{2}|[1-9]?[0-9])){3}(?![0-9.])"
)
_IPV6_CANDIDATE_RE = re.compile(r"(?<![A-Za-z0-9_:])(?:[0-9A-Fa-f]{0,4}:){2,}[0-9A-Fa-f:.%]+(?![A-Za-z0-9_:])")
_PII_PLACEHOLDERS = {"<email>", "<phone>", "<ip-address>"}


def sanitize_host_paths(text):
    """Replace Linux/macOS/Windows personal home roots with `~` (keep suffix)."""
    if not text:
        return text
    return _HOST_PATH_RE.sub("~", text)


def sanitize_ip_addresses(text):
    """Replace valid IPv4/IPv6 literals; preserve ordinary dotted versions."""
    if not text:
        return text
    text = _IPV4_RE.sub("<ip-address>", text)

    def redact_ipv6(match):
        token = match.group(0)
        candidate = token.split("%", 1)[0] if "%" in token else token
        try:
            ipaddress.IPv6Address(candidate)
        except ipaddress.AddressValueError:
            return token
        return "<ip-address>"

    return _IPV6_CANDIDATE_RE.sub(redact_ipv6, text)


def sanitize_common_pii(text):
    """Replace common personal contact/network identifiers with placeholders."""
    if not text:
        return text
    text = sanitize_host_paths(text)
    text = _EMAIL_RE.sub("<email>", text)
    text = _PHONE_RE.sub("<phone>", text)
    return sanitize_ip_addresses(text)


# REVIEW-OB-009: internal project/tool names are lab-internal identifiers —
# the corpus and the generated site are public, so they must never appear in
# any emitted text field. Each (pattern, placeholder) pair rewrites one
# internal name to a neutral token. Ordered: the gitreins tool path comes
# first and swallows its own /home/<user>/ or ~/ prefix, so the account name
# cannot survive as a host-path leak when the generic rewrite runs after.
# Mirrored in internal/export/git.go internalNameRedactions (keep in sync).
_NAME_REDACTIONS = [
    (re.compile(r"(?:/home/[A-Za-z0-9_-]+|~)?/?\.local/bin/gitreins"), "<tool>"),
    (re.compile(r"hermes-dagger"), "<project>"),
    (re.compile(r"chimera-v2"), "<project>"),
    (re.compile(r"warpfs"), "<project>"),
    (re.compile(r"crier"), "<project>"),
]


def sanitize_internal_names(text):
    """Rewrite every internal project/tool name in a corpus text field.

    None/empty pass through unchanged. Unrelated text — including the word
    "gitreins" alone (only the .local/bin/gitreins install path is
    redacted) — is untouched.
    """
    if not text:
        return text
    for pattern, placeholder in _NAME_REDACTIONS:
        text = pattern.sub(placeholder, text)
    return text


def sanitize_corpus_text(text):
    """Full corpus scrub: internal identifiers first, then common PII."""
    if not text:
        return text
    return sanitize_common_pii(sanitize_internal_names(text))


def sanitize_pii_value(value):
    """Recursively sanitize strings in parsed structured metadata."""
    if isinstance(value, str):
        return sanitize_corpus_text(value)
    if isinstance(value, list):
        return [sanitize_pii_value(item) for item in value]
    if isinstance(value, dict):
        return {sanitize_corpus_text(str(key)): sanitize_pii_value(item)
                for key, item in value.items()}
    return value


# REVIEW-OB-010: environment/version are exact-match discovery filters
# (internal/graph/discovery.go ranks answers by (env, lang, version) tuple
# specificity; empty values act as wildcards). Free-text prose in these
# fields ("linux host, go 1.26.6", "~/auger; DuckBrain live substrate",
# "api.deepseek.com/v1, verified 2026-09-15T02:39Z") makes an answer
# unreachable via tuple-scoped discovery and leaks host context into the
# public catalog. Both fields are normalized to canonical tokens at export:
# a token-shaped value (no whitespace, ';', '~' or '<'/'>' — the REVIEW-OB-009
# redaction placeholders, e.g. "hilo/<project>", are not meaningful filter
# tokens) passes through lowercased; a leading comma/semicolon segment that
# is itself a clean token wins; prose maps to a canonical token when a
# recognizable pattern exists; anything else is dropped to "" (the
# discovery wildcard) — prose never reaches the filtered fields. Mirrored
# in internal/graph/normalize.go (keep the two in sync).
_TOKEN_FORBIDDEN_RE = re.compile(r"[\s;~<>]")

# Ordered — first match wins: CI/container platforms precede bare OS tokens
# (a GitHub Actions runner IS ubuntu; the CI context is the distinguishing
# one) and OS tokens precede language runtimes.
_ENV_PATTERNS = [
    (re.compile(r"github[- ]actions|actions runner"), "github-actions"),
    (re.compile(r"docker"), "docker"),
    (re.compile(r"kubernetes|k8s"), "kubernetes"),
    (re.compile(r"darwin|macos|mac os"), "darwin"),
    (re.compile(r"wsl|linux|ubuntu|debian|posix"), "linux"),
    (re.compile(r"windows"), "windows"),
    (re.compile(r"production"), "production"),
    (re.compile(r"vitest|nodejs|\bnode\b|nextjs|pnpm|\bnpm\b"), "node"),
    (re.compile(r"pytest|python|venv"), "python3"),
    (re.compile(r"\bgo ?1\.26|\bgo1\.26"), "go1.26"),
    (re.compile(r"golang|\bgo\b"), "go"),
    (re.compile(r"\bbash\b"), "bash"),
    (re.compile(r"\bshell\b|\bsh\b"), "shell"),
]

_VERSION_TOKEN_RE = re.compile(r"^v?\d+(?:\.\d+)*$")
_VERSION_BRANCH_TOKENS = ("latest", "main", "master")


def _clean_token(value):
    """Return value lowercased/trimmed if already token-shaped, else None."""
    v = (value or "").strip().lower()
    if not v or _TOKEN_FORBIDDEN_RE.search(v):
        return None
    return v


def normalize_env(value):
    """Canonicalize an environment filter token (REVIEW-OB-010).

    Prose with no recognizable pattern drops to "" — the discovery
    wildcard; it never survives into the filtered field.
    """
    tok = _clean_token(value)
    if tok is not None:
        return tok
    v = (value or "").strip().lower()
    if not v:
        return ""
    first = re.split(r"[,;]", v, maxsplit=1)[0].strip()
    if first and not _TOKEN_FORBIDDEN_RE.search(first):
        return first
    for pattern, token in _ENV_PATTERNS:
        if pattern.search(v):
            return token
    return ""


def normalize_version(value):
    """Canonicalize a version filter token (REVIEW-OB-010).

    Prose yields the first whitespace-delimited token that is
    version-shaped ("gitreins 0.14.0" -> "0.14.0"), then a branch token
    ("main e50aea4" -> "main"), and drops to "" when neither exists.
    """
    tok = _clean_token(value)
    if tok is not None:
        return tok
    v = (value or "").strip().lower()
    if not v:
        return ""
    fields = [t.strip(",;") for t in v.split()]
    for t in fields:
        if _VERSION_TOKEN_RE.match(t):
            return t
    for t in fields:
        if t in _VERSION_BRANCH_TOKENS:
            return t
    return ""


def is_excluded_class(title: str) -> bool:
    """True if a raw (pre-slugify) class title is a self-test/canary/probe."""
    t = title.lower()
    return any(p.search(t) for p in _EXCLUDED_RES)


def slugify(title: str) -> str:
    s = title.strip().lower()
    s = re.sub(r"[^a-z0-9]+", "-", s)
    s = re.sub(r"-{2,}", "-", s).strip("-")
    return s[:80] or "untitled"


def main() -> None:
    db = sqlite3.connect(DB_PATH)
    db.row_factory = sqlite3.Row
    c = db.cursor()

    c.execute("""
        SELECT pc.id AS class_id, pc.title, pc.description, pc.created_at AS class_created,
               an.id AS answer_id, an.lang, an.env, an.version, an.solution,
               an.evidence, an.signatures, an.status, an.created_at AS answer_created
        FROM problem_classes pc
        JOIN answer_nodes an ON an.class_id = pc.id
        WHERE an.status = 'verified'
        ORDER BY pc.id, an.id
    """)
    rows = c.fetchall()

    # Group by class. Every text field is sanitized on the way in
    # (REVIEW-OB-006 host paths, REVIEW-OB-009 internal names) so no
    # operator path or lab-internal identifier can reach any output file,
    # whichever writer (JSONL, per-class JSON, INDEX.md) consumes the record.
    classes: dict[int, dict] = {}
    for r in rows:
        cls = classes.setdefault(r["class_id"], {
            "class_id": r["class_id"],
            "title": sanitize_corpus_text(r["title"]),
            "description": sanitize_corpus_text(r["description"]),
            "created_at": r["class_created"],
            "answers": [],
        })
        cls["answers"].append({
            "answer_id": r["answer_id"],
            "language": sanitize_corpus_text(r["lang"]),
            # REVIEW-OB-010: env/version are exact-match discovery filters —
            # sanitize, then canonicalize so prose never enters the export.
            "environment": normalize_env(sanitize_corpus_text(r["env"])),
            "version": normalize_version(sanitize_corpus_text(r["version"])),
            "solution": sanitize_corpus_text(r["solution"]),
            "evidence": sanitize_corpus_text(r["evidence"]),
            "signatures": sanitize_pii_value(json.loads(r["signatures"])) if r["signatures"] else None,
            "status": r["status"],
            "created_at": r["answer_created"],
        })

    # Filter out self-test/canary/probe classes — lab plumbing, not verified
    # engineering answers. They must not appear in any export output.
    excluded = [cls for cls in classes.values() if is_excluded_class(cls["title"])]
    for cls in excluded:
        del classes[cls["class_id"]]
    if excluded:
        print(f"Excluded {len(excluded)} self-test/canary/probe classes:")
        for cls in sorted(excluded, key=lambda x: x["class_id"]):
            print(f"  - [{cls['class_id']}] {cls['title']} "
                  f"({len(cls['answers'])} answers)")

    os.makedirs(ANSWERS_DIR, exist_ok=True)

    # 1. Master JSONL (one answer per line, class metadata embedded)
    jsonl_path = os.path.join(DATA_DIR, "answers.jsonl")
    n_answers = 0
    with open(jsonl_path, "w") as f:
        for cls in classes.values():
            for ans in cls["answers"]:
                record = {
                    "class_id": cls["class_id"],
                    "class": slugify(cls["title"]),
                    "title": cls["title"],
                    "description": cls["description"],
                    **ans,
                }
                f.write(json.dumps(record, ensure_ascii=False) + "\n")
                n_answers += 1

    # 2. One file per class
    for cls in classes.values():
        slug = slugify(cls["title"])
        path = os.path.join(ANSWERS_DIR, f"{cls['class_id']:04d}-{slug}.json")
        with open(path, "w") as f:
            json.dump(cls, f, ensure_ascii=False, indent=2)
            f.write("\n")

    # 2b. Remove stale per-class files so data/answers/ stays in sync with
    # the export (excluded classes, renamed slugs, deleted classes).
    expected_files = {
        f"{cls['class_id']:04d}-{slugify(cls['title'])}.json"
        for cls in classes.values()
    }
    removed = 0
    for fname in sorted(os.listdir(ANSWERS_DIR)):
        if re.match(r"^\d{4}-.+\.json$", fname) and fname not in expected_files:
            os.remove(os.path.join(ANSWERS_DIR, fname))
            removed += 1
            print(f"  removed stale {fname}")
    if removed:
        print(f"Removed {removed} stale per-class files")

    # 3. INDEX.md
    index_path = os.path.join(DATA_DIR, "INDEX.md")
    now = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M UTC")
    with open(index_path, "w") as f:
        f.write(f"# Off-by-One Answer Index\n\n")
        f.write(f"**{len(classes)} problem classes · {n_answers} verified answers** · "
                f"exported {now}\n\n")
        f.write("Browse per-class files in [`data/answers/`](answers/), or use the "
                "master [`answers.jsonl`](answers.jsonl).\n\n")
        f.write("| Class ID | Problem | Answers | Languages |\n")
        f.write("|----------|---------|---------|-----------|\n")
        for cls in sorted(classes.values(), key=lambda x: -len(x["answers"])):
            langs = sorted({a["language"] for a in cls["answers"] if a["language"]})
            f.write(f"| {cls['class_id']} | {cls['title']} | {len(cls['answers'])} "
                    f"| {', '.join(langs)} |\n")

    # 3b. COUNTS.md — small auto-stamped counts file. README and docs link to
    # this instead of hardcoding corpus counts that drift within days.
    counts_path = os.path.join(DATA_DIR, "COUNTS.md")
    with open(counts_path, "w") as f:
        f.write("# Corpus counts\n\n")
        f.write(f"**{len(classes)} problem classes · {n_answers} verified answers** · "
                f"exported {now}\n\n")
        f.write("Source of truth: data/INDEX.md (regenerated every sync).\n")

    # 4. README.md (consumer guide)
    readme_path = os.path.join(DATA_DIR, "README.md")
    with open(readme_path, "w") as f:
        f.write("""# Off-by-One — Answer Distribution

This directory is the **flat-file distribution layer** of the Off-by-One
pre-solve lab. Every file is plain JSON / Markdown in a git repo — **no server,
no SQLite, no setup** needed to consume or contribute.

## What's here

| Path | Content |
|------|---------|
| `answers.jsonl` | Master file — one verified answer per line (JSON) |
| `answers/` | One JSON file per problem class — browse, diff, PR |
| `INDEX.md` | Catalog of every problem class + language coverage |
| `COUNTS.md` | Live corpus counts — auto-stamped every export |

## Privacy-by-default

Answers are scrubbed before publication because agents can copy real workstation
context into otherwise useful diagnostics. Personal Linux/macOS/Windows home
roots become `~`; email addresses, phone numbers, and IPv4/IPv6 literals become
`<email>`, `<phone>`, and `<ip-address>`. The same policy is enforced on the
read-only public API and by CI against the generated `data/` and `site/` artifacts.
Gitleaks still scans credentials; it is not the PII filter.

## Consume (3 ways)

**1. Grab one file (no clone):**
```bash
curl -O https://raw.githubusercontent.com/totalwindupflightsystems/off-by-one/master/data/answers/0001-unknown.json
```

**2. Clone everything:**
```bash
git clone --depth 1 https://github.com/totalwindupflightsystems/off-by-one
# answers in data/answers.jsonl
```

**3. Search locally (no server):**
```bash
grep -l '"title": ".*raft.*"' data/answers/*.json   # find a class
jq '.answers[0].solution' data/answers/0001-unknown.json  # read a solution
```

## Record shape

Each line of `answers.jsonl` (and each entry in `answers/*.json`):

```json
{
  "class_id": 1,
  "class": "hello-world",
  "title": "Hello World",
  "description": "...",
  "answer_id": 1,
  "language": "go",
  "environment": "docker",
  "version": "latest",
  "solution": "...",
  "evidence": "...",
  "signatures": {"model": "bash", "result": "passed", "tests": 1},
  "status": "verified",
  "created_at": "2026-07-08 01:34:24"
}
```

## Contribute

Add or fix an answer? Open a PR that adds/updates a file under `answers/`
(or appends to `answers.jsonl`) — maintainers verify and merge.

## Freshness

Exported from the live lab's SQLite store. Regenerate with:
```bash
python3 scripts/export-answers.py
```
""")

    print(f"Exported {len(classes)} classes / {n_answers} answers → {DATA_DIR}")
    print(f"  {jsonl_path}")
    print(f"  {ANSWERS_DIR}/ (per-class files)")
    print(f"  {index_path}")
    print(f"  {counts_path}")
    print(f"  {readme_path}")


if __name__ == "__main__":
    main()
