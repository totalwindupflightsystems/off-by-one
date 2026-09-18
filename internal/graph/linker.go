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

// LinkSimilarClasses links classID to up to maxNeighbors other problem
// classes whose titles share at least minShared lexemes, writing a
// `similar` edge in BOTH directions for each neighbour. It returns the
// number of edges created — 0 on a re-run, because every edge the previous
// run wrote collides with the unique (source, target, relationship) index
// and is skipped. A class with no qualifying neighbour returns 0 and no
// error: that is an ordinary outcome, not a failure.
//
// Weight is overlap over the smaller of the two token sets —
// |A∩B| / min(|A|,|B|), always in (0,1]: two titles whose significant
// tokens are the same score 1.0, while one shared word between two long
// titles scores low and sorts last.
//
// Both directions are written because every traversal read in the codebase
// is single-directional (ListEdgesFrom, relatedClasses, Discovery, and the
// /related handler), so a one-sided write would make `similar` mean
// "A points at B" — /problems/A/related would list B while
// /problems/B/related stayed empty, which is not what a similarity
// relationship claims.
func (s *Store) LinkSimilarClasses(ctx context.Context, classID int64, minShared, maxNeighbors int) (int, error) {
	if classID == 0 {
		return 0, errors.New("graph: class id must be non-zero")
	}
	if minShared < 1 {
		minShared = 1
	}
	if maxNeighbors < 1 {
		maxNeighbors = 1
	}

	src, err := s.GetProblemClass(ctx, classID)
	if err != nil {
		return 0, err
	}
	srcTokens := tokenSet(TitleTokens(src.Title))
	if len(srcTokens) == 0 {
		// Nothing to match on (e.g. a title made entirely of stopwords or
		// two-letter prefixes). No candidates can qualify.
		return 0, nil
	}

	candidates, err := s.similarCandidates(ctx, classID, srcTokens, minShared)
	if err != nil {
		return 0, err
	}

	// Deterministic ranking: strongest overlap first, then more shared
	// tokens, then title ascending — so the neighbours chosen for a class
	// are identical on every run, and a maxNeighbors cut-off always drops
	// the same candidates.
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

	created := 0
	for _, c := range candidates {
		for _, dir := range [][2]int64{{classID, c.id}, {c.id, classID}} {
			if _, err := s.CreateEdge(ctx, dir[0], dir[1], EdgeSimilar, c.weight); err != nil {
				if errors.Is(err, ErrDuplicate) {
					continue // already linked by an earlier run — idempotent
				}
				return created, fmt.Errorf("create similar edge %d->%d: %w", dir[0], dir[1], err)
			}
			created++
		}
	}
	return created, nil
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
		candTokens := tokenSet(TitleTokens(title))
		shared := overlapCount(srcTokens, candTokens)
		if shared < minShared {
			continue
		}
		smaller := len(srcTokens)
		if len(candTokens) < smaller {
			smaller = len(candTokens)
		}
		if smaller == 0 {
			continue
		}
		out = append(out, similarCandidate{
			id:     id,
			title:  title,
			shared: shared,
			weight: clampWeight(float64(shared) / float64(smaller)),
		})
	}
	return out, rows.Err()
}

// LinkAllSimilar runs LinkSimilarClasses for every problem class and returns
// the total number of edges created. Re-running is safe: every edge written
// by the first run already exists, so the second run returns 0 and the
// problem_edges table is unchanged.
func (s *Store) LinkAllSimilar(ctx context.Context, minShared, maxNeighbors int) (int, error) {
	ids, err := s.allProblemClassIDs(ctx)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, id := range ids {
		n, err := s.LinkSimilarClasses(ctx, id, minShared, maxNeighbors)
		if err != nil {
			return total, fmt.Errorf("link class %d: %w", id, err)
		}
		total += n
	}
	return total, nil
}

// allProblemClassIDs returns every class ID in ascending order. The IDs are
// collected and the cursor closed before any linking happens: the linking
// loop INSERTs into problem_edges, and holding an open read cursor over
// problem_classes across those writes would interleave a reader with writers
// on the same pooled SQLite connections for the whole run.
func (s *Store) allProblemClassIDs(ctx context.Context) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM problem_classes ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list problem class ids: %w", err)
	}
	defer rows.Close()

	ids := make([]int64, 0, 256)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
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
