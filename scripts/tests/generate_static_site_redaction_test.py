#!/usr/bin/env python3
"""OB-GAP-093 regression test: internal project/tool names (the REVIEW-OB-009
NAME_PATTERN set) must never leak into the PUBLIC static site through
class-id-derived surfaces — the filename slug, <title>, og:title, meta
description, h1 and the canonical/sitemap URLs in
scripts/generate-static-site.py.

The export pipeline (scripts/export-answers.py) sanitizes TEXT fields, but
the class_id is an identifier: the generator derives the public slug and the
title fallback from it. This test pins:

  2638-dexdat-core-...        -> slug 2638-project-core-...   (slug-safe token)
  title "dexdat-core ..."     -> title "<project>-core ..."   (readable placeholder)
  public_stem()               -> deterministic (same input, same bytes)
  build_slug_map()            -> two distinct raw ids mapping to one slug
                                 abort generation loudly (no silent overwrite)
  prune_stale_class_pages()   -> a renamed/redacted class never lingers under
                                 its old slug

Pure in-process unittest: no network, no DB, no credentials. The end-to-end
proof is `make check-corpus-hygiene` over the regenerated site/ tree.
"""
import importlib.util
import json
import os
import re
import tempfile
import unittest

_HERE = os.path.dirname(os.path.abspath(__file__))
_GEN_PATH = os.path.join(_HERE, "..", "generate-static-site.py")
_spec = importlib.util.spec_from_file_location("generate_static_site", _GEN_PATH)
_gen = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_gen)

public_stem = _gen.public_stem
build_slug_map = _gen.build_slug_map
prune_stale_class_pages = _gen.prune_stale_class_pages
class_page = _gen.class_page

# The real flagged class_id stem from OB-GAP-093 (CI job 'Corpus host-path
# hygiene', REVIEW-OB-009 NAME_PATTERN).
LEAKY_STEM = "2638-dexdat-core-bunker-qa-compose-up-fail-fresh-agent"

# Every internal-name token the hygiene guard rejects (NAME_PATTERN in
# scripts/check-corpus-hygiene.sh), in a stem-shaped spelling.
INTERNAL_NAME_STEMS = [
    "1-hermes-dagger-thing",
    "2-chimera-v2-thing",
    "3-warpfs-thing",
    "4-crier-thing",
    "5--.local-bin-gitreins-thing",
    "6-wojons-repo-thing",
    "7-get-h3-thing",
    "8-dexdat-thing",
    "9-9router-thing",
    "10-task-router-thing",
    "11-musterflow-thing",
]
NAME_RX = re.compile(
    r"hermes-dagger|chimera-v2|warpfs|crier|\.local/bin/gitreins|wojons/|get-h3|dexdat|9router|task-router|musterflow",
    re.IGNORECASE,
)


class TestPublicStemRedaction(unittest.TestCase):
    def test_flagged_class_slug_is_redacted(self):
        self.assertEqual(
            public_stem(LEAKY_STEM),
            "2638-project-core-bunker-qa-compose-up-fail-fresh-agent",
        )

    def test_slug_is_slug_safe(self):
        slug = public_stem(LEAKY_STEM)
        self.assertRegex(slug, r"^[a-z0-9-]+$")

    def test_every_internal_name_is_redacted(self):
        for stem in INTERNAL_NAME_STEMS:
            with self.subTest(stem=stem):
                self.assertIsNone(NAME_RX.search(public_stem(stem)))

    def test_case_insensitive(self):
        self.assertNotIn("dexdat", public_stem("12-DEXDAT-Core").lower())

    def test_deterministic(self):
        self.assertEqual(public_stem(LEAKY_STEM), public_stem(LEAKY_STEM))

    def test_clean_stem_unchanged(self):
        self.assertEqual(
            public_stem("100-sqlite-fts5-search"), "100-sqlite-fts5-search"
        )

    def test_clean_stem_byte_parity(self):
        # A clean stem is returned verbatim — even a trailing dash — so the
        # public slug stays byte-identical to the corpus filename (the
        # source link and existing external URLs keep working).
        odd = "1461-python-foreman-board-demo-drift-from-judge-pnpm-test-"
        self.assertEqual(public_stem(odd), odd)


class TestSlugCollisionCheck(unittest.TestCase):
    def test_distinct_raw_ids_colliding_abort(self):
        # 'dexdat' redacts to 'project', colliding with the literal stem.
        with self.assertRaises(SystemExit):
            build_slug_map(["1-dexdat-core", "1-project-core"])

    def test_distinct_slugs_pass(self):
        mapping = build_slug_map(["1-dexdat-core", "2-project-core"])
        self.assertEqual(mapping["1-dexdat-core"], "1-project-core")
        self.assertEqual(mapping["2-project-core"], "2-project-core")

    def test_empty_passes(self):
        self.assertEqual(build_slug_map([]), {})


class TestClassPageRedaction(unittest.TestCase):
    def _page(self):
        data = {
            "title": "dexdat-core-bunker-qa-compose-up-fail-fresh-agent",
            "description": "Problem class: dexdat-core compose up fails.",
            "answers": [
                {
                    "solution": "# Fix\n\nRebuild the image and retry.\n",
                    "language": "bash",
                    "environment": "linux",
                }
            ],
        }
        return class_page(LEAKY_STEM, data)

    def test_no_internal_name_anywhere_in_page(self):
        self.assertIsNone(NAME_RX.search(self._page()))

    def test_title_keeps_readable_placeholder(self):
        html = self._page()
        # esc() escapes the placeholder for HTML; the rendered page reads
        # '<project>-core-...'.
        self.assertIn("&lt;project&gt;-core-bunker-qa", html)
        self.assertIn("<title>&lt;project&gt;-core", html)
        self.assertIn('og:title" content="&lt;project&gt;-core', html)

    def test_canonical_uses_public_slug(self):
        html = self._page()
        self.assertIn(
            "https://ob1.it.com/classes/2638-project-core-bunker-qa-compose-up-fail-fresh-agent.html",
            html,
        )


class TestPruneStaleClassPages(unittest.TestCase):
    def test_stale_page_removed_current_kept(self):
        with tempfile.TemporaryDirectory() as d:
            current = os.path.join(d, "2638-project-core.html")
            stale = os.path.join(d, "2638-dexdat-core.html")
            other = os.path.join(d, "not-a-page.txt")
            for path in (current, stale, other):
                with open(path, "w") as f:
                    f.write("x")
            removed = prune_stale_class_pages(d, {"2638-project-core"})
            self.assertEqual(removed, ["2638-dexdat-core.html"])
            self.assertTrue(os.path.exists(current))
            self.assertTrue(os.path.exists(other))
            self.assertFalse(os.path.exists(stale))


class TestEndToEndDeterminism(unittest.TestCase):
    """Run the real main() twice against a scratch corpus carrying an
    unredacted class_id: identical filenames across runs, no internal name
    anywhere in the output tree, and the stale page from a 'previous'
    generation pruned."""

    def test_main_twice_is_deterministic_and_clean(self):
        with tempfile.TemporaryDirectory() as root:
            data_dir = os.path.join(root, "data", "answers")
            out_dir = os.path.join(root, "site")
            os.makedirs(data_dir)
            os.makedirs(os.path.join(out_dir, "classes"))
            row = {
                "class_id": "dexdat-core-thing",
                "title": "dexdat-core-thing",
                "description": "",
                "created_at": "2026-01-01",
                "answers": [],
            }
            with open(os.path.join(data_dir, LEAKY_STEM + ".json"), "w") as f:
                json.dump(row, f)
            # Simulate a stale page from a pre-redaction generation.
            stale = os.path.join(out_dir, "classes", "999-dexdat-stale.html")
            with open(stale, "w") as f:
                f.write("stale")

            old_data, old_out = _gen.DATA_DIR, _gen.OUT_DIR
            _gen.DATA_DIR, _gen.OUT_DIR = data_dir, out_dir
            try:
                _gen.main()
                first = sorted(os.listdir(os.path.join(out_dir, "classes")))
                _gen.main()
                second = sorted(os.listdir(os.path.join(out_dir, "classes")))
            finally:
                _gen.DATA_DIR, _gen.OUT_DIR = old_data, old_out

            self.assertEqual(first, second)
            self.assertEqual(
                first, ["2638-project-core-bunker-qa-compose-up-fail-fresh-agent.html"]
            )
            for dirpath, _dirs, files in os.walk(out_dir):
                for fn in files:
                    self.assertIsNone(NAME_RX.search(fn))
                    with open(os.path.join(dirpath, fn)) as f:
                        self.assertIsNone(NAME_RX.search(f.read()))


if __name__ == "__main__":
    unittest.main()
