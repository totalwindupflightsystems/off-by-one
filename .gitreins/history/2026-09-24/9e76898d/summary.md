# Verdict: OB-DF12

**Task:** Restore site/ regeneration in ob1-distribute.sh PART 1
**Evaluated:** 2026-09-24T22:48:34.120569
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	3.019s
- ✓ **tier2**
  - COMPLETE
  ✓ scripts/ob1-distribute.sh PART 1 invokes scripts/generate-static-site.py, includes site/ and scripts/generate-static-site.py in both its git diff --quiet change-guard and git add lists; the committed site/ is freshly regenerated: site/index.html class count equals data/COUNTS.md count, site/classes/ page count equals the number of data/answers/*.json files, sitemap.xml loc count equals pages plus one, and a site/classes/ page exists for the class bwrap-uid-map-permission-denied: scripts/ob1-distribute.sh:33 invokes `/usr/bin/python3 scripts/generate-static-site.py >/tmp/ob1_site.log 2>&1 || { ... exit 1; }` in PART 1; line 39 `if ! git diff --quiet data/ site/ scripts/export-answers.py scripts/generate-static-site.py README.md;` and line 40 `git add data/ site/ scripts/export-answers.py scripts/generate-static-site.py README.md` — both site/ and scripts/generate-static-site.py present in the change-guard AND the add list (bash -n: SYNTAX OK). Freshness proven by re-running the generator: `/usr/bin/python3 scripts/generate-static-site.py` -> rc=0, stdout `index.html: 2189 classes / 2281 answers` and `sitemap.xml: 2189 class URLs · robots.txt written · total 2190 pages`, and `git status --porcelain site/` returned EMPTY afterwards, i.e. the committed site/ is byte-identical to a fresh regeneration. Counts: data/COUNTS.md = `2189 problem classes`; site/index.html:102 `<div class="big">2189</div><div class="lbl">Problem classes</div>` == 2189; `ls data/answers/*.json | wc -l` = 2189 and `ls site/classes/*.html | wc -l` = 2189 (git ls-tree -r HEAD site/classes = 2189; set-difference between answer stems and page stems is empty, so no stale pages); `grep -c '<loc>' site/sitemap.xml` = 2190 = 2189 class pages + 1 index URL (first loc `https://ob1.it.com/`); site/classes/2142-bwrap-uid-map-permission-denied.html exists and is tracked in HEAD (blob d8b8f6edf153be62d9002b61e51ee361f5638761). No test suite applies to this shell/asset criterion; verification was by executing the generator and inspecting committed artifacts. [resolution 0.01; scripts/ob1-distribute.sh, scripts/generate-static-site.py, data/COUNTS.md]
PART 1 of ob1-distribute.sh now runs generate-static-site.py and guards/adds site/ + the generator, and the committed site/ is a fresh regeneration with 2189 class pages matching data/answers/ and COUNTS.md, 2190 sitemap locs, and a page for bwrap-uid-map-permission-denied.

## Summary

Judge Result: OB-DF12

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	3.019s

Stage tier2: PASS
  COMPLETE
  ✓ scripts/ob1-distribute.sh PART 1 invokes scripts/generate-static-site.py, includes site/ and scripts/generate-static-site.py in both its git diff --quiet change-guard and git add lists; the committed site/ is freshly regenerated: site/index.html class count equals data/COUNTS.md count, site/classes/ page count equals the number of data/answers/*.json files, sitemap.xml loc count equals pages plus one, and a site/classes/ page exists for the class bwrap-uid-map-permission-denied: scripts/ob1-distribute.sh:33 invokes `/usr/bin/python3 scripts/generate-static-site.py >/tmp/ob1_site.log 2>&1 || { ... exit 1; }` in PART 1; line 39 `if ! git diff --quiet data/ site/ scripts/export-answers.py scripts/generate-static-site.py README.md;` and line 40 `git add data/ site/ scripts/export-answers.py scripts/generate-static-site.py README.md` — both site/ and scripts/generate-static-site.py present in the change-guard AND the add list (bash -n: SYNTAX OK). Freshness proven by re-running the generator: `/usr/bin/python3 scripts/generate-static-site.py` -> rc=0, stdout `index.html: 2189 classes / 2281 answers` and `sitemap.xml: 2189 class URLs · robots.txt written · total 2190 pages`, and `git status --porcelain site/` returned EMPTY afterwards, i.e. the committed site/ is byte-identical to a fresh regeneration. Counts: data/COUNTS.md = `2189 problem classes`; site/index.html:102 `<div class="big">2189</div><div class="lbl">Problem classes</div>` == 2189; `ls data/answers/*.json | wc -l` = 2189 and `ls site/classes/*.html | wc -l` = 2189 (git ls-tree -r HEAD site/classes = 2189; set-difference between answer stems and page stems is empty, so no stale pages); `grep -c '<loc>' site/sitemap.xml` = 2190 = 2189 class pages + 1 index URL (first loc `https://ob1.it.com/`); site/classes/2142-bwrap-uid-map-permission-denied.html exists and is tracked in HEAD (blob d8b8f6edf153be62d9002b61e51ee361f5638761). No test suite applies to this shell/asset criterion; verification was by executing the generator and inspecting committed artifacts. [resolution 0.01; scripts/ob1-distribute.sh, scripts/generate-static-site.py, data/COUNTS.md]
PART 1 of ob1-distribute.sh now runs generate-static-site.py and guards/adds site/ + the generator, and the committed site/ is a fresh regeneration with 2189 class pages matching data/answers/ and COUNTS.md, 2190 sitemap locs, and a page for bwrap-uid-map-permission-denied.

Overall: PASS ✓
