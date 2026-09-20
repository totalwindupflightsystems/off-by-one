package graph

import (
	"database/sql/driver"
	"fmt"
	"sync"

	sqlite "modernc.org/sqlite"
)

// PlaceholderClassSQLFunc is the name of the SQLite scalar function that
// exposes IsPlaceholderClass to SQL. Callers must not hard-code the name:
// reference it (or, better, NotPlaceholderClassSQL) so a rename lands in
// one place.
//
// The function is registered on the SQLite driver by openDSN, i.e. before
// the first connection of the process exists, so every connection a
// graph.Store opens can call it (OB-GAP-084).
const PlaceholderClassSQLFunc = "ob1_is_placeholder_class"

// NotPlaceholderClassSQL is the SQL predicate that EXCLUDES placeholder
// (self-test/canary/probe) rows from a queue_entries query — the SQL twin
// of `!IsPlaceholderClass(problem_class)`. It names the problem_class
// column of queue_entries.
//
// The Go slice in placeholder.go stays the authoritative definition of a
// placeholder class: this predicate evaluates that slice through the
// ob1_is_placeholder_class scalar function, so the two cannot drift the way
// an independent LIKE/GLOB transcription could (SQL LIKE `_` is a
// single-character wildcard, and the families here include `tick\d+-e2e`
// patterns a hand-written pattern table would have to re-derive). The
// parity is pinned by TestPlaceholderClassSQLFunc_MatchesGoPredicate (this
// package) and by the SQL-vs-Go oracle in internal/ingest (OB-GAP-084).
//
// The exclusion exists so SQLite — not Go — applies the filter: the
// listings then get their page from LIMIT/OFFSET and their match count
// from COUNT(*) over the same bounded key range, instead of materialising
// every matching row and slicing it in Go. Before OB-GAP-084 the read
// paths fetched the whole status-filtered set on every poll.
const NotPlaceholderClassSQL = "NOT " + PlaceholderClassSQLFunc + "(problem_class)"

// registerPlaceholderClassSQLFunc registers the scalar function once per
// process. The driver rejects a duplicate registration ("a function named
// %q is already registered"), so the result is memoised: opening a second
// Store must not fail, and a genuine registration failure must surface on
// every Open rather than being silently swallowed after the first.
var registerPlaceholderClassSQLFunc = sync.OnceValue(func() error {
	return sqlite.RegisterDeterministicScalarFunction(
		PlaceholderClassSQLFunc,
		1,
		func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			return placeholderArg(args), nil
		},
	)
})

// RegisterPlaceholderClassSQLFunc makes the placeholder predicate callable
// from SQL. It is idempotent and safe for concurrent use; openDSN calls it
// before opening the first connection, so callers only need this when they
// assemble a database handle outside graph.Open.
func RegisterPlaceholderClassSQLFunc() error {
	if err := registerPlaceholderClassSQLFunc(); err != nil {
		return fmt.Errorf("register sql function %s: %w", PlaceholderClassSQLFunc, err)
	}
	return nil
}

// placeholderArg evaluates the predicate over the single SQL argument.
//
// A SQL NULL (or an unexpected argument type) answers false — i.e. "not a
// placeholder" — matching the Go predicate's treatment of an absent title
// and, more importantly, never hiding a row: problem_class is NOT NULL in
// the schema, so this branch only decides how a malformed input is treated,
// and the safe direction is to list the row rather than swallow it.
func placeholderArg(args []driver.Value) int64 {
	if len(args) != 1 {
		return 0
	}
	switch v := args[0].(type) {
	case string:
		if IsPlaceholderClass(v) {
			return 1
		}
	case []byte:
		if IsPlaceholderClass(string(v)) {
			return 1
		}
	}
	return 0
}
