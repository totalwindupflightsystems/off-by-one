#!/usr/bin/env python3
"""Fail closed if common personal identifiers remain in public corpus/site.

Gitleaks scans credentials, but the answer corpus is allowlisted for example
secrets. This independent guard checks structured corpus text plus rendered
HTML. It reports only category, file, and count — never matched values.
"""
import ipaddress
import json
import os
import re
import sys

ROOT = sys.argv[1] if len(sys.argv) > 1 else os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
HOME_PATH = re.compile(
    r"(?i)(?:\\*/home|\\*/users|\\*/mnt/[a-z]/users|"
    r"\\*/cygdrive/[a-z]/users|[a-z]:[/\\]+users|"
    r"\\*/documents and settings)[/\\]+[A-Za-z0-9._-]+"
)
EMAIL = re.compile(r"(?i)\b[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}\b")
# DF-OFF-BY-ONE-33: dotless user@host identifiers (user@localhost,
# user@builder-machine) are PII the dotted-TLD email pattern misses, but a
# bare dotless regex matches legitimate programming syntax (CI action tags
# actions/checkout@v4, package versions vitest@4, Go module versions
# net/http@go1.26.5, workflow scope ids run@step1, decorator-adjacent
# fragments like n@dataclass). Count only candidates the classifier accepts
# as real user@host identifiers. Keep in sync with _DOTLESS_* /
# _is_dotless_user_host in scripts/export-answers.py and
# dotlessCandidatePattern / isDotlessUserHost in internal/export/git.go.
DOTLESS_CANDIDATE = re.compile(r"(?i)\b([A-Z0-9._%+-]+)@([A-Z0-9_-]+)\b")
_DOTLESS_VERSION_HOST_RE = re.compile(r"(?i)^v?\d")
_DOTLESS_SHA_HOST_RE = re.compile(r"(?i)^[0-9a-f]{7,40}$")
_DOTLESS_SCOPE_HOST_RE = re.compile(r"(?i)^(?:step\d*|each|item-\d+)$")
_DOTLESS_VX_HOST_RE = re.compile(r"(?i)^vx+$")
_DOTLESS_VERSION_LOCAL_RE = re.compile(r"\d+\.\d+")
_DOTLESS_REF_HOSTS = frozenset({
    "latest", "main", "master", "next", "head",
    "version", "init",
})
PHONE = re.compile(r"(?<![0-9])(?:\+?[0-9]{1,3}[ .-]?)?(?:\([0-9]{3}\)|[0-9]{3})[ .-][0-9]{3}[ .-][0-9]{4}(?![0-9])")
IPV4 = re.compile(r"(?<![0-9.])(?:25[0-5]|2[0-4][0-9]|1[0-9]{2}|[1-9]?[0-9])(?:\.(?:25[0-5]|2[0-4][0-9]|1[0-9]{2}|[1-9]?[0-9])){3}(?![0-9.])")
IPV6_CANDIDATE = re.compile(r"(?<![A-Za-z0-9_:])(?:[0-9A-Fa-f]{0,4}:){2,}[0-9A-Fa-f:.%]+(?![A-Za-z0-9_:])")
CHECKS = (("personal home path", HOME_PATH), ("email address", EMAIL), ("phone number", PHONE), ("IPv4 address", IPV4))


def _is_dotless_user_host(local, host, following):
    """Mirror of export-answers.py — true user@host identifier vs syntax."""
    if len(local) < 2 or len(host) < 2:
        return False
    if not local[0].isalnum() or not local[-1].isalnum() or ".." in local:
        return False
    if _DOTLESS_VERSION_LOCAL_RE.search(local):
        return False
    if _DOTLESS_VERSION_HOST_RE.match(host):
        return False
    low = host.lower()
    if low in _DOTLESS_REF_HOSTS or _DOTLESS_VX_HOST_RE.match(low):
        return False
    if _DOTLESS_SCOPE_HOST_RE.match(host):
        return False
    if _DOTLESS_SHA_HOST_RE.match(host) and any(c.isdigit() for c in host):
        return False
    if following.startswith("+"):
        return False
    if len(following) == 2 and following[0] == "." and following[1].isalnum():
        return False
    return True


def scan_text(text):
    counts = {}
    for label, pattern in CHECKS:
        matches = len(pattern.findall(text))
        if matches:
            counts[label] = matches
    dotless = 0
    for match in DOTLESS_CANDIDATE.finditer(text):
        following = text[match.end():match.end() + 2]
        if _is_dotless_user_host(match.group(1), match.group(2), following):
            dotless += 1
    if dotless:
        counts["dotless host identifier"] = dotless
    ipv6 = 0
    for match in IPV6_CANDIDATE.finditer(text):
        token = match.group(0).split("%", 1)[0]
        try:
            ipaddress.IPv6Address(token)
        except ipaddress.AddressValueError:
            continue
        ipv6 += 1
    if ipv6:
        counts["IPv6 address"] = ipv6
    return counts


def scan_json(value):
    counts = {}
    if isinstance(value, dict):
        for key, item in value.items():
            if isinstance(key, str):
                for label, count in scan_text(key).items():
                    counts[label] = counts.get(label, 0) + count
            for label, count in scan_json(item).items():
                counts[label] = counts.get(label, 0) + count
    elif isinstance(value, list):
        for item in value:
            for label, count in scan_json(item).items():
                counts[label] = counts.get(label, 0) + count
    elif isinstance(value, str):
        return scan_text(value)
    return counts


def main():
    failures = []

    def record(path, counts):
        for category, count in counts.items():
            failures.append((os.path.relpath(path, ROOT), category, count))

    data = os.path.join(ROOT, "data")
    jsonl = os.path.join(data, "answers.jsonl")
    if os.path.isfile(jsonl):
        with open(jsonl, encoding="utf-8") as stream:
            for line_number, line in enumerate(stream, 1):
                if not line.strip():
                    continue
                try:
                    item = json.loads(line)
                except json.JSONDecodeError:
                    failures.append((os.path.relpath(jsonl, ROOT), "invalid JSON line", line_number))
                    continue
                record(jsonl, scan_json(item))

    answers = os.path.join(data, "answers")
    if os.path.isdir(answers):
        for directory, _, names in os.walk(answers):
            for name in names:
                if not name.endswith(".json"):
                    continue
                path = os.path.join(directory, name)
                try:
                    with open(path, encoding="utf-8") as stream:
                        item = json.load(stream)
                except (OSError, json.JSONDecodeError):
                    failures.append((os.path.relpath(path, ROOT), "invalid JSON file", 1))
                    continue
                record(path, scan_json(item))

    # Non-JSON catalog files (index/readme/count docs) and generated HTML are
    # scanned as text. JSON data is parsed first so serialization escapes do
    # not create false matches across quoted field boundaries.
    for area in ("data", "site"):
        base = os.path.join(ROOT, area)
        if not os.path.isdir(base):
            continue
        for directory, _, names in os.walk(base):
            for name in names:
                path = os.path.join(directory, name)
                if area == "data" and (path == jsonl or (path.startswith(answers + os.sep) and name.endswith(".json"))):
                    continue
                try:
                    with open(path, encoding="utf-8", errors="replace") as stream:
                        text = stream.read()
                except OSError:
                    continue
                record(path, scan_text(text))

    if failures:
        print("public PII hygiene VIOLATION (values withheld):", file=sys.stderr)
        for path, category, count in failures:
            print(f"  {path}: {category} x{count}", file=sys.stderr)
        return 1
    print("public PII hygiene OK: no personal paths, emails, phones, or IP addresses under data/ or site/")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
