#!/usr/bin/env python3
"""OBS-OB-006 regression test — `make corpus-jsonl-selftest`.

data/answers.jsonl used to be TRACKED: 7450 lines / ~56 MB, rewritten in full by
every corpus export, so each export committed a fresh tens-of-MB blob and .git
grew without bound. The per-class files data/answers/<id>-<slug>.json are now
the canonical tracked form; the monolith is a generated, gitignored local
artifact rebuilt by scripts/build-answers-jsonl.py (`make corpus-jsonl`).

Three claims have to hold for that to be safe, and this test proves all three:

  A. REPRODUCTION — the builder reproduces the LAST TRACKED monolith
     byte-for-byte (line count + sha256), is deterministic, and `--check` is RED
     when the file is missing or stale. Skips LOUDLY (never silently passes)
     when the git object is unavailable, e.g. a shallow clone.
  B. MONOLITH-ABSENT CONSUMERS — check-corpus-hygiene.sh,
     check-corpus-tokens.py, check-public-pii.py and normalize-corpus-tokens.py
     all work with data/answers.jsonl absent, deriving their view from the
     per-class files; the violation classes they guard are STILL caught with the
     monolith absent (a check that cannot check must not pass), and a tree with
     no answer corpus at all is a hard failure rather than an empty green run.
  C. UNTRACKED — `git ls-files data/answers.jsonl` is empty and `.gitignore`
     ignores it.

Hermetic: temp fixture trees + git objects only. No network, no DB, no
credentials — and the live worktree is never mutated (every write goes to a
private temp dir; the repo is only READ via git plumbing).
"""
import json
import os
import shutil
import subprocess
import sys
import tempfile
import unittest

SELF_DIR = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.dirname(os.path.dirname(SELF_DIR))
SCRIPTS = os.path.join(REPO, "scripts")

BUILDER = os.path.join(SCRIPTS, "build-answers-jsonl.py")
TOKENS = os.path.join(SCRIPTS, "check-corpus-tokens.py")
PII = os.path.join(SCRIPTS, "check-public-pii.py")
HYGIENE = os.path.join(SCRIPTS, "check-corpus-hygiene.sh")
NORMALIZE = os.path.join(SCRIPTS, "normalize-corpus-tokens.py")

MONOLITH = os.path.join(REPO, "data", "answers.jsonl")

# The last committed monolith (tick-515 catch-up export, d7837da9). If this
# ever changes, the file was re-tracked or re-exported into git — which is
# exactly what OBS-OB-006 forbids, so update it only as a deliberate decision.
EXPECTED_SHA256 = "6dce11b815c1a7043d583e32f75fb70cc3ba67bd4fdd28a791ea21eb32fcdaa4"
EXPECTED_LINES = 7450


def run(argv, cwd=None, env=None):
    """Run a command, returning (rc, stdout, stderr) as text."""
    proc = subprocess.run(argv, cwd=cwd, env=env, capture_output=True, text=True)
    return proc.returncode, proc.stdout, proc.stderr


def git(*args):
    proc = subprocess.run(["git", "-C", REPO] + list(args), capture_output=True)
    return proc.returncode, proc.stdout, proc.stderr


def sha256_of(path):
    import hashlib
    digest = hashlib.sha256()
    with open(path, "rb") as stream:
        for chunk in iter(lambda: stream.read(1 << 20), b""):
            digest.update(chunk)
    return digest.hexdigest()


def class_doc(class_id, title, answers, description=None):
    return {
        "class_id": class_id,
        "title": title,
        "description": description or f"{title} description",
        "answers": answers,
    }


def answer(answer_id, **overrides):
    record = {
        "answer_id": answer_id,
        "language": "go",
        "environment": "docker",
        "version": "latest",
        "solution": f"solution {answer_id}",
        "evidence": f"evidence {answer_id}",
        "signatures": {"model": "bash", "result": "passed", "tests": 1},
        "status": "verified",
        "created_at": "2026-07-08 01:34:24",
    }
    record.update(overrides)
    return record


def write_json(path, value, indent=2):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as stream:
        json.dump(value, stream, ensure_ascii=False, indent=indent)
        stream.write("\n")


class FixtureTree:
    """A throwaway corpus root with copies of the scripts it must drive.

    The consumers derive their root from their own path (normalize) or take an
    explicit root (check-corpus-tokens.py / check-public-pii.py /
    CORPUS_HYGIENE_ROOT), so a fixture with its own scripts/ + data/ exercises
    the real code paths without touching the live tree.
    """

    def __init__(self, base):
        self.root = base
        os.makedirs(os.path.join(base, "scripts"), exist_ok=True)
        os.makedirs(os.path.join(base, "data", "answers"), exist_ok=True)
        os.makedirs(os.path.join(base, "site"), exist_ok=True)
        for name in ("build-answers-jsonl.py", "export-answers.py", "normalize-corpus-tokens.py"):
            shutil.copy2(os.path.join(SCRIPTS, name), os.path.join(base, "scripts", name))
        with open(os.path.join(base, "site", "index.html"), "w", encoding="utf-8") as stream:
            stream.write("<html><body>clean ~/text only</body></html>\n")

    @property
    def answers_dir(self):
        return os.path.join(self.root, "data", "answers")

    @property
    def monolith(self):
        return os.path.join(self.root, "data", "answers.jsonl")

    def write_class(self, filename, class_id, title, answers, description=None):
        write_json(os.path.join(self.answers_dir, filename),
                   class_doc(class_id, title, answers, description))

    def build(self, out=None, extra_args=()):
        target = out or os.path.join(self.root, "build", "out.jsonl")
        return run([sys.executable, os.path.join(self.root, "scripts", "build-answers-jsonl.py"),
                    "--out", target] + list(extra_args)), target


class BaseFixtureTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp(prefix="ob1-build-answers-",
                                    dir=os.environ.get("TMPDIR") or None)
        self.addCleanup(shutil.rmtree, self.tmp, True)

    def fixture(self, name):
        return FixtureTree(os.path.join(self.tmp, name))

    def clean_corpus(self, fix):
        fix.write_class("0001-alpha.json", 1, "Alpha", [answer(1), answer(2)])
        fix.write_class("0002-beta.json", 2, "Beta", [answer(3)])


class ReproductionTest(BaseFixtureTest):
    """A. the builder reproduces the last tracked monolith exactly."""

    def _last_tracked_blob(self):
        # Every commit in HEAD's history that touched the path — after the
        # untracking commit, `rev-list -n1` names the DELETE commit, whose tree
        # no longer carries the blob. Walk newest-first and take the first
        # revision that actually has it.
        rc, out, err = git("rev-list", "HEAD", "--", "data/answers.jsonl")
        revisions = out.decode().split()
        if rc != 0 or not revisions:
            self.skipTest(
                "LOUD SKIP: `git rev-list HEAD -- data/answers.jsonl` found no "
                "commit that tracked the monolith, so byte-for-byte reproduction "
                "CANNOT be proven here (shallow clone / no history). "
                f"rc={rc} stderr={err.decode().strip()!r}")
        missing = []
        for revision in revisions:
            proc = subprocess.run(["git", "-C", REPO, "show",
                                   f"{revision}:data/answers.jsonl"], capture_output=True)
            if proc.returncode == 0:
                return revision, proc.stdout
            missing.append(revision[:12])
        self.skipTest(
            "LOUD SKIP: none of the revisions that touched data/answers.jsonl "
            f"carries the blob locally ({len(missing)} tried, newest "
            f"{missing[0]}), so reproduction CANNOT be proven here — a partial "
            "clone. Reproduction is unverified, not green.")

    def test_reproduces_last_tracked_monolith_byte_for_byte(self):
        revision, blob = self._last_tracked_blob()
        out = os.path.join(self.tmp, "rebuilt.jsonl")
        rc, stdout, stderr = run([sys.executable, BUILDER, "--out", out])
        self.assertEqual(rc, 0, f"builder failed: {stderr}")

        with open(out, "rb") as stream:
            rebuilt = stream.read()

        self.assertEqual(rebuilt.count(b"\n"), blob.count(b"\n"),
                         "line count differs from the tracked monolith")
        self.assertEqual(rebuilt.count(b"\n"), EXPECTED_LINES,
                         f"expected {EXPECTED_LINES} lines")
        # Report the first differing line, not just "not equal" — that is what
        # makes a serialisation regression debuggable.
        if rebuilt != blob:
            expected_lines = blob.split(b"\n")
            actual_lines = rebuilt.split(b"\n")
            for number, (want, got) in enumerate(zip(expected_lines, actual_lines), 1):
                if want != got:
                    self.fail(
                        f"first differing line {number}:\n"
                        f"  tracked: {want[:300]!r}\n"
                        f"  rebuilt: {got[:300]!r}")
            self.fail("byte streams differ after a common prefix "
                      f"({len(expected_lines)} vs {len(actual_lines)} lines)")
        self.assertEqual(sha256_of(out), EXPECTED_SHA256,
                         "rebuilt sha256 differs from the tracked monolith "
                         f"({revision})")
        self.assertIn(EXPECTED_SHA256, stdout)
        self.assertIn(str(EXPECTED_LINES), stdout)

    def test_rebuild_is_deterministic(self):
        first = os.path.join(self.tmp, "a.jsonl")
        second = os.path.join(self.tmp, "b.jsonl")
        for path in (first, second):
            rc, _, stderr = run([sys.executable, BUILDER, "--out", path, "--quiet"])
            self.assertEqual(rc, 0, stderr)
        self.assertEqual(sha256_of(first), sha256_of(second),
                         "two rebuilds of the same corpus differ")

    def test_check_is_green_on_a_fresh_build(self):
        out = os.path.join(self.tmp, "fresh.jsonl")
        self.assertEqual(run([sys.executable, BUILDER, "--out", out])[0], 0)
        rc, stdout, stderr = run([sys.executable, BUILDER, "--check", "--out", out])
        self.assertEqual(rc, 0, stderr)
        self.assertIn("CHECK OK", stdout)
        self.assertIn(EXPECTED_SHA256, stdout)

    def test_check_is_red_when_the_file_is_missing(self):
        missing = os.path.join(self.tmp, "nope.jsonl")
        rc, _, stderr = run([sys.executable, BUILDER, "--check", "--out", missing])
        self.assertEqual(rc, 1, "a --check with nothing to check must be red")
        self.assertIn("missing", stderr)
        self.assertFalse(os.path.exists(missing), "--check must not create the file")

    def test_check_is_red_on_a_stale_file(self):
        fix = self.fixture("stale")
        self.clean_corpus(fix)
        (proc, target) = fix.build()
        self.assertEqual(proc[0], 0, proc[2])
        with open(target, "a", encoding="utf-8") as stream:
            stream.write('{"class_id": 99, "class": "stale"}\n')
        rc, stdout, stderr = run([sys.executable, os.path.join(fix.root, "scripts",
                                                              "build-answers-jsonl.py"),
                                  "--check", "--out", target])
        self.assertEqual(rc, 1, "a stale monolith must be red")
        self.assertIn("CHECK FAILED", stderr)


class BuilderContractTest(BaseFixtureTest):
    """Line shape, ordering and the fail-closed paths of the builder."""

    def test_orders_classes_by_class_id_not_filename(self):
        fix = self.fixture("ordering")
        # Unpadded ids: the FILENAME string order ("10-…" < "9-…") is the
        # opposite of the numeric class_id order (9 < 10). Sorting on the
        # filename would silently reorder classes.
        fix.write_class("10-ten.json", 10, "Ten", [answer(100)])
        fix.write_class("9-nine.json", 9, "Nine", [answer(90)])
        proc, target = fix.build()
        self.assertEqual(proc[0], 0, proc[2])
        with open(target, encoding="utf-8") as stream:
            rows = [json.loads(line) for line in stream if line.strip()]
        self.assertEqual([row["class_id"] for row in rows], [9, 10],
                         "classes must be ordered by the class_id field")
        self.assertEqual([row["class"] for row in rows], ["nine", "ten"],
                         "`class` comes from the filename slug")

    def test_key_order_and_answer_fields_are_preserved(self):
        fix = self.fixture("keyorder")
        fix.write_class("0001-alpha.json", 1, "Alpha", [answer(1)])
        proc, target = fix.build()
        self.assertEqual(proc[0], 0, proc[2])
        with open(target, encoding="utf-8") as stream:
            line = stream.readline()
        pairs = json.loads(line, object_pairs_hook=list)
        keys = [key for key, _ in pairs]
        self.assertEqual(keys[:4], ["class_id", "class", "title", "description"],
                         f"meta key order broken: {keys}")
        # Everything after the meta block is the answer record in file order.
        self.assertEqual(keys[4:], ["answer_id", "language", "environment", "version",
                                    "solution", "evidence", "signatures", "status",
                                    "created_at"])

    def test_non_ascii_is_written_unescaped(self):
        fix = self.fixture("unicode")
        fix.write_class("0001-alpha.json", 1, "Alpha", [answer(1, solution="café — ok")])
        proc, target = fix.build()
        self.assertEqual(proc[0], 0, proc[2])
        with open(target, "rb") as stream:
            raw = stream.read()
        self.assertIn("café".encode("utf-8"), raw,
                      "ensure_ascii=False is load-bearing (byte-identity)")

    def test_trailing_newline_and_line_endings(self):
        fix = self.fixture("endings")
        self.clean_corpus(fix)
        proc, target = fix.build()
        self.assertEqual(proc[0], 0, proc[2])
        with open(target, "rb") as stream:
            raw = stream.read()
        self.assertTrue(raw.endswith(b"\n"), "artifact must end with a newline")
        self.assertNotIn(b"\r", raw, "artifact must use \\n line endings")
        self.assertEqual(raw.count(b"\n"), 3)

    def test_quiet_still_reports_count_and_sha(self):
        fix = self.fixture("quiet")
        self.clean_corpus(fix)
        proc, target = fix.build(extra_args=("--quiet",))
        rc, stdout, stderr = proc
        self.assertEqual(rc, 0, stderr)
        self.assertIn(sha256_of(target), stdout)
        self.assertIn("3 lines", stdout)

    def test_fail_closed_on_duplicate_class_id(self):
        fix = self.fixture("dup")
        fix.write_class("0001-alpha.json", 1, "Alpha", [answer(1)])
        fix.write_class("0001-dupe.json", 1, "Dupe", [answer(2)])
        proc, _ = fix.build()
        self.assertEqual(proc[0], 1)
        self.assertIn("duplicate class_id", proc[2])

    def test_fail_closed_on_filename_id_mismatch(self):
        fix = self.fixture("mismatch")
        fix.write_class("0001-alpha.json", 42, "Alpha", [answer(1)])
        proc, _ = fix.build()
        self.assertEqual(proc[0], 1)
        self.assertIn("does not match the filename id", proc[2])

    def test_fail_closed_on_missing_answers_array(self):
        fix = self.fixture("noanswers")
        write_json(os.path.join(fix.answers_dir, "0001-alpha.json"),
                   {"class_id": 1, "title": "Alpha", "description": "d"})
        proc, _ = fix.build()
        self.assertEqual(proc[0], 1)
        self.assertIn("answers", proc[2])

    def test_empty_corpus_fails_instead_of_writing_an_empty_artifact(self):
        fix = self.fixture("empty")
        proc, target = fix.build()
        self.assertEqual(proc[0], 1)
        self.assertIn("no class files found", proc[2])
        self.assertFalse(os.path.exists(target))


class MonolithAbsentConsumersTest(BaseFixtureTest):
    """B. every consumer works — and still catches violations — without it."""

    def hygiene(self, fix):
        env = dict(os.environ)
        env["CORPUS_HYGIENE_ROOT"] = fix.root
        return run(["bash", HYGIENE], env=env)

    def test_clean_tree_passes_with_the_monolith_absent(self):
        fix = self.fixture("clean")
        self.clean_corpus(fix)
        self.assertFalse(os.path.exists(fix.monolith))

        rc, stdout, stderr = self.hygiene(fix)
        self.assertEqual(rc, 0, f"hygiene guard failed without the monolith:\n{stdout}\n{stderr}")

        rc, stdout, stderr = run([sys.executable, TOKENS, fix.root])
        self.assertEqual(rc, 0, stderr)
        self.assertIn("absent (generated artifact", stdout,
                      "the tokens checker must SAY it derived the view, not pass silently")

        rc, stdout, stderr = run([sys.executable, PII, fix.root])
        self.assertEqual(rc, 0, stderr)
        self.assertIn("absent (generated artifact", stdout)

    def test_prose_env_is_still_caught_without_the_monolith(self):
        fix = self.fixture("prose-env")
        fix.write_class("0001-alpha.json", 1, "Alpha",
                        [answer(1, environment="~/auger worktree")])
        rc, _, stderr = run([sys.executable, TOKENS, fix.root])
        self.assertEqual(rc, 1, "prose env must be caught from the per-class files")
        self.assertIn("0001-alpha.json", stderr)
        rc, stdout, stderr = self.hygiene(fix)
        self.assertEqual(rc, 1)
        self.assertIn("0001-alpha.json", stdout + stderr)

    def test_prose_version_is_still_caught_without_the_monolith(self):
        fix = self.fixture("prose-version")
        fix.write_class("0001-alpha.json", 1, "Alpha",
                        [answer(1, version="api.example.com/v1, verified today")])
        rc, _, stderr = run([sys.executable, TOKENS, fix.root])
        self.assertEqual(rc, 1)
        self.assertIn("0001-alpha.json", stderr)

    def test_pii_is_still_caught_without_the_monolith(self):
        fix = self.fixture("pii")
        fix.write_class("0001-alpha.json", 1, "Alpha",
                        [answer(1, evidence="trace archived from test-user@example.com")])
        rc, _, stderr = run([sys.executable, PII, fix.root])
        self.assertEqual(rc, 1, "email in the per-class corpus must still fail")
        self.assertIn("0001-alpha.json", stderr)
        self.assertIn("email address", stderr)
        self.assertNotIn("test-user@example.com", stderr,
                         "the PII checker must never echo the matched value")

    def test_operator_path_is_still_caught_without_the_monolith(self):
        fix = self.fixture("host-path")
        fix.write_class("0001-alpha.json", 1, "Alpha",
                        [answer(1, solution="edit /home/kara/x then run it")])
        rc, stdout, stderr = self.hygiene(fix)
        self.assertEqual(rc, 1)
        self.assertIn("0001-alpha.json", stdout + stderr)

    def test_no_answer_corpus_at_all_is_a_hard_failure(self):
        fix = self.fixture("vacuous")
        shutil.rmtree(fix.answers_dir)
        rc, _, stderr = run([sys.executable, TOKENS, fix.root])
        self.assertEqual(rc, 1,
                         "a tree with no answer view must fail, not scan nothing")
        self.assertIn("no answer corpus to scan", stderr)

    def test_normalize_absent_monolith_does_not_crash_or_create_it(self):
        fix = self.fixture("normalize-absent")
        self.clean_corpus(fix)
        self.assertFalse(os.path.exists(fix.monolith))
        rc, stdout, stderr = run([sys.executable,
                                  os.path.join(fix.root, "scripts",
                                               "normalize-corpus-tokens.py"),
                                  fix.root])
        self.assertEqual(rc, 0, f"normalize crashed without the monolith:\n{stdout}\n{stderr}")
        self.assertIn("not present (generated artifact", stdout)
        self.assertNotIn("Traceback", stderr)
        self.assertFalse(os.path.exists(fix.monolith),
                         "normalize must not materialise an absent bulk artifact")

    def test_normalize_present_monolith_rebuilds_from_the_rewritten_classes(self):
        fix = self.fixture("normalize-present")
        fix.write_class("0001-alpha.json", 1, "Alpha",
                        [answer(1, environment="~/auger worktree")])
        with open(fix.monolith, "w", encoding="utf-8") as stream:
            stream.write('{"class_id": 1, "environment": "stale prose"}\n')

        rc, stdout, stderr = run([sys.executable,
                                  os.path.join(fix.root, "scripts",
                                               "normalize-corpus-tokens.py"),
                                  fix.root])
        self.assertEqual(rc, 0, f"{stdout}\n{stderr}")
        self.assertIn("rebuilt from the per-class files", stdout)

        # The rebuilt monolith must equal a fresh build of the rewritten corpus…
        fresh = os.path.join(fix.root, "build", "fresh.jsonl")
        build = run([sys.executable, os.path.join(fix.root, "scripts",
                                                  "build-answers-jsonl.py"),
                     "--out", fresh])
        self.assertEqual(build[0], 0, build[2])
        self.assertEqual(sha256_of(fix.monolith), sha256_of(fresh),
                         "normalize's rebuild must match build-answers-jsonl.py")

        # …and carry the normalized token, not the prose.
        with open(fix.monolith, encoding="utf-8") as stream:
            row = json.loads(stream.readline())
        self.assertNotIn("stale prose", json.dumps(row))
        for field in ("environment", "version"):
            value = row.get(field) or ""
            self.assertFalse(any(ch in value for ch in " ;~"),
                             f"{field}={value!r} still carries prose")

        # And the sweep stays idempotent.
        again = run([sys.executable, os.path.join(fix.root, "scripts",
                                                  "normalize-corpus-tokens.py"),
                     fix.root])
        self.assertEqual(again[0], 0, again[2])
        self.assertIn("per-class files changed: 0", again[1])


class TrackingTest(unittest.TestCase):
    """C. the monolith is untracked and ignored."""

    def test_monolith_is_not_tracked(self):
        rc, out, err = git("ls-files", "data/answers.jsonl")
        self.assertEqual(rc, 0, err.decode())
        self.assertEqual(out.decode().strip(), "",
                         "data/answers.jsonl is still tracked — the whole point "
                         "of OBS-OB-006 is that it must not be")

    def test_monolith_is_gitignored(self):
        rc, out, err = git("check-ignore", "-v", "data/answers.jsonl")
        self.assertEqual(rc, 0,
                         "data/answers.jsonl must be gitignored so a corpus "
                         f"export cannot re-add it: rc={rc} {err.decode().strip()}")
        self.assertIn("data/answers.jsonl", out.decode())

    def test_build_targets_are_registered(self):
        with open(os.path.join(REPO, "Makefile"), encoding="utf-8") as stream:
            makefile = stream.read()
        for target in ("corpus-jsonl:", "corpus-jsonl-check:", "corpus-jsonl-selftest:"):
            self.assertIn(target, makefile, f"Makefile is missing the {target} target")
        phony = next(line for line in makefile.splitlines()
                     if line.startswith(".PHONY:"))
        for target in ("corpus-jsonl", "corpus-jsonl-check", "corpus-jsonl-selftest"):
            self.assertIn(target, phony, f"{target} missing from .PHONY")


if __name__ == "__main__":
    unittest.main(verbosity=2)
