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
PHONE = re.compile(r"(?<![0-9])(?:\+?[0-9]{1,3}[ .-]?)?(?:\([0-9]{3}\)|[0-9]{3})[ .-][0-9]{3}[ .-][0-9]{4}(?![0-9])")
IPV4 = re.compile(r"(?<![0-9.])(?:25[0-5]|2[0-4][0-9]|1[0-9]{2}|[1-9]?[0-9])(?:\.(?:25[0-5]|2[0-4][0-9]|1[0-9]{2}|[1-9]?[0-9])){3}(?![0-9.])")
IPV6_CANDIDATE = re.compile(r"(?<![A-Za-z0-9_:])(?:[0-9A-Fa-f]{0,4}:){2,}[0-9A-Fa-f:.%]+(?![A-Za-z0-9_:])")
CHECKS = (("personal home path", HOME_PATH), ("email address", EMAIL), ("phone number", PHONE), ("IPv4 address", IPV4))


def scan_text(text):
    counts = {}
    for label, pattern in CHECKS:
        matches = len(pattern.findall(text))
        if matches:
            counts[label] = matches
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
