package ingest

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// SubmitFromHTTPRequest parses an HTTP request body into a Submission
// and calls Submit. The body is expected to be JSON. Validation errors
// return ErrInvalidCadence / ErrEmptyProblemClass / ErrDuplicate, which
// the API handler maps to 400 / 409 responses.
//
// This is a convenience wrapper — the queue itself is HTTP-agnostic.
// The API server in internal/api/ uses this to keep the request
// validation in one place.
func SubmitFromHTTPRequest(ctx context.Context, q *Queue, body []byte) (string, *Entry, error) {
	var sub Submission
	if err := decodeJSON(body, &sub); err != nil {
		return "", nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return q.Submit(ctx, sub)
}

// StatusForHTTP maps a queue status string to an HTTP status code for
// the API. Returns 200 for success, 404 for not-found, 409 for
// duplicate, 400 for validation errors.
func StatusForHTTP(err error) int {
	switch {
	case errors.Is(err, ErrInvalidCadence), errors.Is(err, ErrEmptyProblemClass):
		return http.StatusBadRequest
	case errors.Is(err, ErrDuplicate):
		return http.StatusConflict
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}

// SanitizeForID lowercases and replaces non-alphanumeric characters with
// hyphens. Used when the user-supplied problem_class contains spaces
// or uppercase; we slugify it for storage as the problem_class title
// (which is unique).
func SanitizeForID(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevDash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "unknown"
	}
	return out
}
