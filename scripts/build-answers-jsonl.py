#!/usr/bin/env python3
"""Rebuild data/answers.jsonl — the flat bulk corpus — from the canonical
per-class files under data/answers/.

data/answers/<class_id>-<slug>.json is the TRACKED, canonical distribution
form. data/answers.jsonl is a generated ~56 MB convenience artifact: untracked
(gitignored, see .gitignore) and rebuilt on demand with `make corpus-jsonl`.
It used to be committed, which added a fresh tens-of-MB blob to history on
every corpus export (OBS-OB-006).

The output is byte-for-byte identical to the monolith that was tracked before
the untracking, so anything that read the committed file can regenerate it
locally instead of cloning 56 MB:

  python3 scripts/build-answers-jsonl.py --out /tmp/answers.jsonl && \\
    cmp /tmp/answers.jsonl <(git show <rev-before-untracking>:data/answers.jsonl)

Serialization contract (load-bearing — do not "tidy" it):
  * classes ordered by NUMERIC class_id ascending (not by filename);
  * one line per entry of each class file's "answers" array, in file order;
  * key order per line: class_id, class, title, description, then the answer
    record's own keys in the order the class file lists them;
  * json.dumps(..., ensure_ascii=False) with the default separators, one line
    each, '\n' line endings, trailing newline at EOF.

`class` is the slug carried by the filename (the exporter derives the filename
from the class title, so the filename IS the authoritative slug).

Usage:
  python3 scripts/build-answers-jsonl.py [--out PATH] [--check] [--quiet]

  --out PATH   destination file (default: data/answers.jsonl)
  --check      rebuild in memory and compare against the existing --out file:
               prints line count + sha256 of both sides and exits 1 when the
               file is missing or differs (a check that cannot check is red,
               never a quiet pass)
  --quiet      suppress the progress header; the line count + sha256 summary
               is ALWAYS printed

Stdlib only: no DB, no network, no credentials.
"""
import argparse
import hashlib
import json
import os
import re
import sys
import tempfile

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
ANSWERS_DIR = os.path.join(ROOT, "data", "answers")
DEFAULT_OUT = os.path.join(ROOT, "data", "answers.jsonl")

# <class_id>-<slug>.json, zero-padded by the exporter but not required to be.
CLASS_FILE_RE = re.compile(r"^(\d+)-(.+)\.json$")

# The class-level fields that lead each line, in this order.
META_KEYS = ("class_id", "class", "title", "description")


class BuildError(Exception):
    """A class file broke the corpus contract; the build must not guess."""


def load_classes(answers_dir):
    """Return [(class_id, slug, path, class_dict)] sorted by class_id."""
    if not os.path.isdir(answers_dir):
        raise BuildError(f"no per-class corpus directory at {answers_dir}")

    classes = []
    seen = {}
    for name in sorted(os.listdir(answers_dir)):
        if not name.endswith(".json"):
            continue
        match = CLASS_FILE_RE.match(name)
        if not match:
            raise BuildError(
                f"{name}: class files must be named <class_id>-<slug>.json")
        path = os.path.join(answers_dir, name)
        with open(path, encoding="utf-8") as stream:
            cls = json.load(stream)
        if not isinstance(cls, dict):
            raise BuildError(f"{path}: top-level JSON value is not an object")

        class_id = cls.get("class_id")
        file_id = int(match.group(1))
        if not isinstance(class_id, int) or isinstance(class_id, bool):
            raise BuildError(f"{path}: class_id is not an integer ({class_id!r})")
        if class_id != file_id:
            raise BuildError(
                f"{path}: class_id {class_id} does not match the filename id {file_id}")
        if class_id in seen:
            raise BuildError(
                f"{path}: duplicate class_id {class_id} (already in {seen[class_id]})")
        seen[class_id] = name
        classes.append((class_id, match.group(2), path, cls))

    if not classes:
        raise BuildError(f"no class files found under {answers_dir}")

    classes.sort(key=lambda entry: entry[0])
    return classes


def build_lines(answers_dir):
    """Serialise the whole corpus to JSONL lines (no trailing newline)."""
    classes = load_classes(answers_dir)
    lines = []
    for class_id, slug, path, cls in classes:
        answers = cls.get("answers")
        if not isinstance(answers, list):
            raise BuildError(f"{path}: missing an \"answers\" array")
        for index, ans in enumerate(answers):
            if not isinstance(ans, dict):
                raise BuildError(f"{path}: answers[{index}] is not an object")
            record = {
                "class_id": class_id,
                "class": slug,
                "title": cls.get("title"),
                "description": cls.get("description"),
            }
            # dict.update keeps the position of keys that already exist, so a
            # stray meta key inside an answer cannot reorder the line — same
            # semantics as the exporter's `{**meta, **ans}`.
            record.update(ans)
            lines.append(json.dumps(record, ensure_ascii=False))
    return len(classes), lines


def sha256_of(data):
    return hashlib.sha256(data).hexdigest()


def summarize(label, payload, lines):
    print(f"{label}: {lines} lines · sha256 {sha256_of(payload)} · {len(payload)} bytes")


def first_difference(expected, actual):
    """1-based line number of the first differing line, or None."""
    exp = expected.split(b"\n")
    act = actual.split(b"\n")
    for number, (left, right) in enumerate(zip(exp, act), 1):
        if left != right:
            return number
    if len(exp) != len(act):
        return min(len(exp), len(act)) + 1
    return None


def write_atomic(path, payload):
    directory = os.path.dirname(os.path.abspath(path)) or "."
    os.makedirs(directory, exist_ok=True)
    handle, tmp = tempfile.mkstemp(prefix=".answers-jsonl-", dir=directory)
    try:
        with os.fdopen(handle, "wb") as stream:
            stream.write(payload)
        os.replace(tmp, path)
    except BaseException:
        try:
            os.unlink(tmp)
        except OSError:
            pass
        raise


def main(argv=None):
    parser = argparse.ArgumentParser(
        description="Rebuild data/answers.jsonl from data/answers/*.json")
    parser.add_argument("--out", default=DEFAULT_OUT,
                        help="destination file (default: data/answers.jsonl)")
    parser.add_argument("--check", action="store_true",
                        help="compare a rebuild against the existing --out file; "
                             "exit 1 when it is missing or differs")
    parser.add_argument("--quiet", action="store_true",
                        help="suppress the progress header (the count/sha256 "
                             "summary is always printed)")
    args = parser.parse_args(argv)

    try:
        if not args.quiet:
            print(f"reading canonical per-class corpus: {ANSWERS_DIR}")
        class_count, lines = build_lines(ANSWERS_DIR)
    except (BuildError, OSError, json.JSONDecodeError) as err:
        print(f"build-answers-jsonl: {err}", file=sys.stderr)
        return 1

    payload = ("\n".join(lines) + "\n").encode("utf-8")
    if not args.quiet:
        print(f"classes: {class_count}")

    if args.check:
        if not os.path.exists(args.out):
            print(f"build-answers-jsonl: {args.out} is missing — nothing to "
                  f"compare; build it with: make corpus-jsonl", file=sys.stderr)
            summarize("rebuilt", payload, len(lines))
            return 1
        try:
            with open(args.out, "rb") as stream:
                existing = stream.read()
        except OSError as err:
            print(f"build-answers-jsonl: cannot read {args.out}: {err}", file=sys.stderr)
            return 1
        existing_lines = existing.count(b"\n")
        summarize("existing", existing, existing_lines)
        summarize("rebuilt ", payload, len(lines))
        if existing != payload:
            where = first_difference(payload, existing)
            print(f"build-answers-jsonl: CHECK FAILED — {args.out} differs from a "
                  f"fresh rebuild (first differing line: {where})", file=sys.stderr)
            return 1
        print(f"build-answers-jsonl: CHECK OK — {args.out} is byte-identical to a "
              f"fresh rebuild")
        return 0

    try:
        write_atomic(args.out, payload)
    except OSError as err:
        print(f"build-answers-jsonl: cannot write {args.out}: {err}", file=sys.stderr)
        return 1
    if not args.quiet:
        print(f"wrote {args.out}")
    summarize(args.out, payload, len(lines))
    return 0


if __name__ == "__main__":
    sys.exit(main())
