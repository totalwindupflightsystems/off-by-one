package graph

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// This file is the production producer of problem_edges. Before it existed
// Store.CreateEdge had no caller outside tests, so every
// /api/v1/problems/<class>/related request answered `{"related":[]}` and
// POST /api/v1/problems/discover returned an empty related list, regardless
// of how much corpus the database held (OB-GAP-067).
//
// Why title lexemes: the flat corpus (data/answers/*.json) carries
// {class_id, title, description, created_at, answers[]}, and answers[].signatures
// is SOLVE metadata ({model, result, tests, problem_class}) — it is not an
// error signature and it is per-answer, not per-class. The only relationship
// signal the data actually carries is the class title slug, whose tokens are
// chosen by whoever named the problem ("python-sdk-idle-audit",
// "python-audit-idle-maintenance"). Token overlap is therefore the honest,
// deterministic, explainable link we can compute — and it is deliberately
// presented as `similar`, not as `same_root_cause`: shared words are
// evidence of family resemblance, not proof of a shared cause.
//
// OB-GAP-073 (cost): the first version of this linker scanned the class
// table once per class and tokenized every candidate title inside that scan,
// so a full pass was O(classes²) tokenizations plus one autocommit INSERT per
// edge (~7800 write transactions on the live corpus). Both paths now share
// one scoring rule and one batched write:
//
//   - LinkAllSimilar tokenizes every title exactly once into a titleIndex
//     (plus an inverted token→classes map) and reads candidate overlaps out
//     of that index instead of re-scanning and re-tokenizing the table.
//   - every edge INSERT of a pass goes through writeSimilarEdges, which runs
//     the batch in a single transaction.
//
// None of the graph semantics changed: same selected neighbours, same
// weights, same two directions, same idempotent re-runs. The pre-optimization
// algorithm is pinned as a parity oracle in linker_test.go.

// minTitleTokenLen is the shortest token TitleTokens keeps. One- and
// two-character fragments ("go", "so", "js", "ds", "py") are language or
// topic prefixes that appear in hundreds of titles, so they carry no family
// signal; keeping them would link almost every class to almost every other.
const minTitleTokenLen = 3

// titleStopwords is the complete set of tokens TitleTokens drops on top of
// the length rule. It stays deliberately tiny — generic English glue plus
// two words that describe the deliverable of a task rather than its family
// ("fix", "bug"). Words that look generic but DO carry the family signal
// ("error", "nil", "idle", "audit", "drift", "stale", "flaky") are not
// stopwords: dropping them would erase exactly the relationships this
// linker exists to record.
var titleStopwords = map[string]bool{
	"the":  true,
	"and":  true,
	"for":  true,
	"with": true,
	"fix":  true,
	"bug":  true,
}

// TitleTokens splits a problem-class title into the significant lexemes used
// for similarity linking. Titles are slugs, so the input is lowercased,
// split on every non-alphanumeric run, and filtered down to tokens of at
// least minTitleTokenLen characters that are not titleStopwords. Duplicates
// are dropped while preserving first-seen order, so the same title always
// produces the same slice.
func TitleTokens(title string) []string {
	out := make([]string, 0, 4)
	seen := make(map[string]bool, 4)
	for _, tok := range strings.FieldsFunc(strings.ToLower(title), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if len(tok) < minTitleTokenLen || titleStopwords[tok] || seen[tok] {
			continue
		}
		seen[tok] = true
		out = append(out, tok)
	}
	return out
}

// titleTokenizer is the tokenizer every linking path calls (directly or
// through tokenSet). It is a package-level variable ONLY so a test can count
// how many titles one linking pass tokenizes — OB-GAP-073's core contract is
// that a full pass tokenizes each title once, not once per other class, and
// an invocation count is the only non-flaky way to assert that. Production
// leaves it at TitleTokens; tests swap it and restore it, so it must never be
// reassigned while a linking pass is running (the package's tests are
// sequential and do not call t.Parallel).
var titleTokenizer = TitleTokens

// tokenSet is the set form of TitleTokens output — used for the overlap
// computation, where position is irrelevant and repeats must not count.
func tokenSet(tokens []string) map[string]bool {
	set := make(map[string]bool, len(tokens))
	for _, t := range tokens {
		set[t] = true
	}
	return set
}

// similarCandidate is one class paired with the similarity the linker
// computed against the class being linked.
type similarCandidate struct {
	id     int64
	title  string
	shared int
	weight float64
}

// similarEdge is one directed edge the linker has decided to write. Edges are
// planned entirely in memory first so the write phase can be a single batch
// (see writeSimilarEdges) instead of one autocommit INSERT per row.
type similarEdge struct {
	source int64
	target int64
	weight float64
}

// clampLinkParams applies the documented defaults for the two linking knobs.
// LinkAllSimilar clamps once per pass and LinkSimilarClasses clamps once per
// call — the same rule the per-class body used to apply inline.
func clampLinkParams(minShared, maxNeighbors int) (int, int) {
	if minShared < 1 {
		minShared = 1
	}
	if maxNeighbors < 1 {
		maxNeighbors = 1
	}
	return minShared, maxNeighbors
}

// similarWeight is the single definition of the similarity score:
// |A∩B| / min(|A|,|B|), clamped to (0,1]. Two titles whose significant
// tokens are the same score 1.0, while one shared word between two long
// titles scores low and sorts last. ok is false when the candidate has no
// significant tokens — such a candidate cannot qualify and is dropped, which
// also keeps the division away from a zero denominator.
func similarWeight(shared, srcSize, candSize int) (float64, bool) {
	smaller := srcSize
	if candSize < smaller {
		smaller = candSize
	}
	if smaller == 0 {
		return 0, false
	}
	return clampWeight(float64(shared) / float64(smaller)), true
}

// rankCandidates applies the deterministic ranking and the maxNeighbors
// cut-off: strongest overlap first, then more shared tokens, then title
// ascending — so the neighbours chosen for a class are identical on every
// run, and the cut-off always drops the same candidates. Titles are unique,
// so this comparator is a total order and the result does not depend on the
// order candidates were discovered in.
func rankCandidates(candidates []similarCandidate, maxNeighbors int) []similarCandidate {
	sort.Slice(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.weight != b.weight {
			return a.weight > b.weight
		}
		if a.shared != b.shared {
			return a.shared > b.shared
		}
		return a.title < b.title
	})
	if len(candidates) > maxNeighbors {
		candidates = candidates[:maxNeighbors]
	}
	return candidates
}

// planSimilarEdges expands the ranked neighbours of one class into the edges
// to be written: both directions for each neighbour, in the order the
// per-class linker has always written them.
func planSimilarEdges(classID int64, candidates []similarCandidate) []similarEdge {
	edges := make([]similarEdge, 0, 2*len(candidates))
	for _, c := range candidates {
		edges = append(edges, similarEdge{source: classID, target: c.id, weight: c.weight})
		edges = append(edges, similarEdge{source: c.id, target: classID, weight: c.weight})
	}
	return edges
}

// writeSimilarEdges inserts every planned edge and returns the number of rows
// actually created. An existing edge collides with the unique
// (source, target, relationship) index, is skipped, and therefore does not
// count — that is what makes a re-run create 0.
//
// The whole batch runs in ONE transaction. Before batching, each edge was its
// own autocommit INSERT, so a full pass on the live corpus paid ~7800 write
// transactions for rows that mostly already existed (OB-GAP-073). The batch
// is also atomic: a failure or a cancelled context rolls the batch back and
// leaves problem_edges exactly as it was, and the next (idempotent) linking
// run fills in whatever is missing. That is why the error path returns 0
// rather than the number of rows the aborted batch had managed to insert.
//
// The transaction covers INSERTs only — candidate selection is pure
// in-memory work done before this call, so a pass never holds the SQLite
// write lock while it computes, and readers (WAL) are never blocked by it.
func (s *Store) writeSimilarEdges(ctx context.Context, edges []similarEdge) (int, error) {
	if len(edges) == 0 {
		return 0, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin similar edge batch: %w", err)
	}
	created := 0
	for _, e := range edges {
		if _, err := insertEdge(ctx, tx, e.source, e.target, EdgeSimilar, e.weight); err != nil {
			if errors.Is(err, ErrDuplicate) {
				continue // already linked by an earlier run — idempotent
			}
			_ = tx.Rollback()
			return 0, fmt.Errorf("create similar edge %d->%d: %w", e.source, e.target, err)
		}
		created++
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit similar edge batch: %w", err)
	}
	return created, nil
}

// LinkSimilarClasses links classID to up to maxNeighbors other problem
// classes whose titles share at least minShared lexemes, writing a
// `similar` edge in BOTH directions for each neighbour. It returns the
// number of edges created — 0 on a re-run, because every edge the previous
// run wrote collides with the unique (source, target, relationship) index
// and is skipped. A class with no qualifying neighbour returns 0 and no
// error: that is an ordinary outcome, not a failure.
//
// Weight is overlap over the smaller of the two token sets — see
// similarWeight.
//
// Both directions are written because every traversal read in the codebase
// is single-directional (ListEdgesFrom, relatedClasses, Discovery, and the
// /related handler), so a one-sided write would make `similar` mean
// "A points at B" — /problems/A/related would list B while
// /problems/B/related stayed empty, which is not what a similarity
// relationship claims.
//
// This is the single-class entry point (the import path uses it after it
// adds or updates a class); it scans the class table once for one class.
// LinkAllSimilar is the whole-corpus pass and does not go through here — it
// scores every class from one tokenized snapshot instead of re-scanning the
// table per class. Both paths share similarWeight, rankCandidates and
// writeSimilarEdges, so the semantics cannot drift.
func (s *Store) LinkSimilarClasses(ctx context.Context, classID int64, minShared, maxNeighbors int) (int, error) {
	if classID == 0 {
		return 0, errors.New("graph: class id must be non-zero")
	}
	minShared, maxNeighbors = clampLinkParams(minShared, maxNeighbors)

	src, err := s.GetProblemClass(ctx, classID)
	if err != nil {
		return 0, err
	}
	srcTokens := tokenSet(titleTokenizer(src.Title))
	if len(srcTokens) == 0 {
		// Nothing to match on (e.g. a title made entirely of stopwords or
		// two-letter prefixes). No candidates can qualify.
		return 0, nil
	}

	candidates, err := s.similarCandidates(ctx, classID, srcTokens, minShared)
	if err != nil {
		return 0, err
	}
	edges := planSimilarEdges(classID, rankCandidates(candidates, maxNeighbors))
	return s.writeSimilarEdges(ctx, edges)
}

// similarCandidates returns every class other than classID whose title
// shares at least minShared lexemes with srcTokens, with its overlap weight.
// Candidates that do not qualify are dropped here (not later) so the
// maxNeighbors cut-off can never be consumed by a non-matching class.
func (s *Store) similarCandidates(ctx context.Context, classID int64, srcTokens map[string]bool, minShared int) ([]similarCandidate, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, title FROM problem_classes WHERE id != ?`, classID)
	if err != nil {
		return nil, fmt.Errorf("list candidate classes: %w", err)
	}
	defer rows.Close()

	var out []similarCandidate
	for rows.Next() {
		var id int64
		var title string
		if err := rows.Scan(&id, &title); err != nil {
			return nil, err
		}
		candTokens := tokenSet(titleTokenizer(title))
		shared := overlapCount(srcTokens, candTokens)
		if shared < minShared {
			continue
		}
		weight, ok := similarWeight(shared, len(srcTokens), len(candTokens))
		if !ok {
			continue
		}
		out = append(out, similarCandidate{
			id:     id,
			title:  title,
			shared: shared,
			weight: weight,
		})
	}
	return out, rows.Err()
}

// tokenizedClass is one problem class whose title has been tokenized exactly
// once for a whole-corpus linking pass.
type tokenizedClass struct {
	id     int64
	title  string
	tokens map[string]bool
}

// titleIndex is the tokenized snapshot of the problem_classes table that
// LinkAllSimilar works from, plus the inverted map that makes candidate
// lookup cheap: byToken[tok] lists the positions (into classes) of the titles
// that carry that lexeme, so the classes worth scoring for a source class are
// the union of the postings of its own tokens rather than the whole table.
type titleIndex struct {
	classes []tokenizedClass
	byToken map[string][]int
}

// loadTitleIndex reads every class title and tokenizes it ONCE. The rows are
// fully consumed and the cursor closed before the caller writes anything: the
// batch that follows INSERTs into problem_edges, and holding an open read
// cursor over problem_classes across those writes would interleave a reader
// with a writer on the same pooled SQLite connections for the whole pass.
func (s *Store) loadTitleIndex(ctx context.Context) (*titleIndex, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, title FROM problem_classes ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list candidate classes: %w", err)
	}
	defer rows.Close()

	idx := &titleIndex{byToken: make(map[string][]int)}
	for rows.Next() {
		var id int64
		var title string
		if err := rows.Scan(&id, &title); err != nil {
			return nil, err
		}
		tokens := tokenSet(titleTokenizer(title))
		pos := len(idx.classes)
		idx.classes = append(idx.classes, tokenizedClass{id: id, title: title, tokens: tokens})
		for tok := range tokens {
			idx.byToken[tok] = append(idx.byToken[tok], pos)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return idx, nil
}

// candidateScratch holds the reusable per-class work buffers of one linking
// pass: shared[pos] is the number of lexemes the class at that position
// shares with the class currently being linked, and touched lists the
// positions that were counted, so they can be reset for the next class. One
// scratch per pass keeps the scoring loop allocation-free per class.
type candidateScratch struct {
	shared  []int
	touched []int
}

func newCandidateScratch(classes int) *candidateScratch {
	return &candidateScratch{shared: make([]int, classes)}
}

// candidatesFor returns the candidates for the class at srcPos: every other
// class sharing at least minShared lexemes, with its shared count and weight.
// It produces exactly the set the per-class table scan used to produce (the
// oracle in linker_test.go pins that), only from pre-tokenized sets and via
// the inverted index rather than by re-reading and re-tokenizing every other
// title.
func (idx *titleIndex) candidatesFor(srcPos, minShared int, scratch *candidateScratch) []similarCandidate {
	src := idx.classes[srcPos]
	if len(src.tokens) == 0 {
		// Nothing to match on — no candidate can qualify. (Mirrors the
		// per-class early return.)
		return nil
	}

	scratch.touched = scratch.touched[:0]
	for tok := range src.tokens {
		for _, pos := range idx.byToken[tok] {
			if pos == srcPos {
				continue // a class is never its own neighbour
			}
			if scratch.shared[pos] == 0 {
				scratch.touched = append(scratch.touched, pos)
			}
			scratch.shared[pos]++
		}
	}

	out := make([]similarCandidate, 0, len(scratch.touched))
	for _, pos := range scratch.touched {
		shared := scratch.shared[pos]
		scratch.shared[pos] = 0 // reset for the next source class
		if shared < minShared {
			continue
		}
		cand := idx.classes[pos]
		weight, ok := similarWeight(shared, len(src.tokens), len(cand.tokens))
		if !ok {
			continue
		}
		out = append(out, similarCandidate{
			id:     cand.id,
			title:  cand.title,
			shared: shared,
			weight: weight,
		})
	}
	return out
}

// LinkAllSimilar runs the linker for every problem class and returns the
// total number of edges created. Re-running is safe: every edge written by
// the first run already exists, so the second run returns 0 and the
// problem_edges table is unchanged.
//
// One pass (OB-GAP-073):
//
//  1. tokenize every title once into a titleIndex;
//  2. score each class against the candidates its tokens point at — the same
//     overlap rule, ranking and maxNeighbors cut-off as the per-class path;
//  3. write every planned edge in a single transaction.
//
// Steps 1-2 are the same candidate set and the same truncation the old
// per-class scan produced; only the work to get there changed, from
// O(classes²) title tokenizations and one table scan per class to one
// tokenization per class and one postings lookup per token.
func (s *Store) LinkAllSimilar(ctx context.Context, minShared, maxNeighbors int) (int, error) {
	minShared, maxNeighbors = clampLinkParams(minShared, maxNeighbors)

	idx, err := s.loadTitleIndex(ctx)
	if err != nil {
		return 0, err
	}
	scratch := newCandidateScratch(len(idx.classes))
	var edges []similarEdge
	for pos := range idx.classes {
		candidates := idx.candidatesFor(pos, minShared, scratch)
		edges = append(edges, planSimilarEdges(idx.classes[pos].id, rankCandidates(candidates, maxNeighbors))...)
	}
	return s.writeSimilarEdges(ctx, edges)
}

// overlapCount counts the tokens present in both sets.
func overlapCount(a, b map[string]bool) int {
	// Iterate the smaller side so a long title paired with a short one
	// costs the short one's size.
	if len(b) < len(a) {
		a, b = b, a
	}
	n := 0
	for tok := range a {
		if b[tok] {
			n++
		}
	}
	return n
}

// clampWeight keeps a similarity weight inside (0,1]. Overlap over the
// smaller token set cannot exceed 1 while tokens are deduped, but the guard
// keeps a future scoring change from writing a weight the API/UI contract
// (0 < weight <= 1) would reject. A non-positive score is normalised to 1.0,
// matching CreateEdge's own treatment of a non-positive weight.
func clampWeight(w float64) float64 {
	if w <= 0 || w > 1 {
		return 1
	}
	return w
}
