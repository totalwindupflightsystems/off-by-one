package api

import (
	"context"
	"errors"
	"strconv"

	"github.com/totalwindupflightsystems/off-by-one/internal/graph"
	"github.com/totalwindupflightsystems/off-by-one/internal/ingest"
)

// resolveClass resolves a problem-class address — the {class} path segment of
// the read routes, or the problem_class field of a discover request — to the
// stored problem class.
//
// List and answer payloads publish numeric ids ("id":2816) as the class
// identity, so an agent that echoes an id back must be able to address the
// class with it. Before REVIEW-OB-020 every read route resolved ONLY the
// sanitized title via Store.GetProblemClassByTitle, so a numeric address
// 404'd even though the id was right there in the payload it came from.
//
// Resolution order:
//
//  1. Sanitize the segment (ingest.SanitizeForID) and look it up as a title —
//     byte-for-byte the behaviour the routes had before, so slug addressing
//     is unchanged.
//  2. Only when that lookup reports graph.ErrNotFound AND the segment is all
//     digits, parse it as int64 and fall back to Store.GetProblemClass(ctx,id).
//
// Title match wins over id match: a class whose title is literally all digits
// stays addressable by that title. Digits survive SanitizeForID (they are
// copied through, submit.go), so the sanitized form is the safest thing to
// test for "all digits" — it is the exact token the title lookup just used,
// and trailing punctuation or case is normalized the same way for ids as it
// already is for titles.
//
// A missing class (neither arm matches) returns graph.ErrNotFound so callers
// keep their existing 404 writeError shape; every other error is returned
// untouched so callers surface it as 500.
func (s *Server) resolveClass(ctx context.Context, segment string) (*graph.ProblemClass, error) {
	slug := ingest.SanitizeForID(segment)
	pc, err := s.Store.GetProblemClassByTitle(ctx, slug)
	if err == nil {
		return pc, nil
	}
	if !errors.Is(err, graph.ErrNotFound) {
		return nil, err
	}
	id, ok := allDigits(slug)
	if !ok {
		return nil, err
	}
	return s.Store.GetProblemClass(ctx, id)
}

// allDigits reports whether s is a non-empty run of ASCII digits and, if so,
// returns its int64 value. Anything else — empty, signed, spaced, overflowing
// int64 — is not a numeric address, and the caller keeps its not-found result.
func allDigits(s string) (int64, bool) {
	if s == "" {
		return 0, false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
	}
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}
