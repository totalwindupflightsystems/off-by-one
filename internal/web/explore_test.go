package web

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fetchExploreJS serves /js/explore.js through the real handler and
// returns the body — the exact bytes a browser would execute.
func fetchExploreJS(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(Handler())
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/js/explore.js")
	if err != nil {
		t.Fatalf("GET /js/explore.js: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(body)
}

// countArgs returns the number of top-level comma-separated arguments
// in the parenthesised list starting at openParen (src[openParen] == '(').
// Quotes are honoured so commas inside string literals do not count.
func countArgs(src string, openParen int) (int, error) {
	if openParen >= len(src) || src[openParen] != '(' {
		return 0, fmt.Errorf("no '(' at offset %d", openParen)
	}
	depth := 0
	args := 0
	seenAny := false
	var quote byte
	for i := openParen; i < len(src); i++ {
		c := src[i]
		if quote != 0 {
			if c == quote && src[i-1] != '\\' {
				quote = 0
			}
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
		case '(':
			depth++
			if depth > 1 {
				continue
			}
		case ')':
			depth--
			if depth == 0 {
				if seenAny {
					args++
				}
				return args, nil
			}
		case ',':
			if depth == 1 {
				args++
				seenAny = false
				continue
			}
		}
		if depth >= 1 && c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			seenAny = true
		}
	}
	return 0, fmt.Errorf("unbalanced parens from offset %d", openParen)
}

// declArity finds "function <name>(...)" and returns its parameter count.
func declArity(t *testing.T, src, name string) int {
	t.Helper()
	needle := "function " + name
	idx := strings.Index(src, needle)
	if idx < 0 {
		t.Fatalf("declaration of %s not found in explore.js", name)
	}
	open := strings.Index(src[idx:], "(")
	if open < 0 {
		t.Fatalf("no parameter list after %s declaration", name)
	}
	n, err := countArgs(src, idx+open)
	if err != nil {
		t.Fatalf("parse %s declaration: %v", name, err)
	}
	return n
}

// callArities returns the argument count of every call site of name
// (occurrences of "name(" not part of its "function name" declaration).
func callArities(t *testing.T, src, name string) []int {
	t.Helper()
	var out []int
	needle := name + "("
	start := 0
	for {
		idx := strings.Index(src[start:], needle)
		if idx < 0 {
			break
		}
		idx += start
		start = idx + len(needle)
		// Skip the declaration itself ("...function renderVersionChildren(").
		head := strings.TrimRight(src[:idx], " \t")
		if strings.HasSuffix(head, "function") {
			continue
		}
		n, err := countArgs(src, idx+len(name))
		if err != nil {
			t.Fatalf("parse call site at offset %d: %v", idx, err)
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		t.Fatalf("no call sites found for %s", name)
	}
	return out
}

// TestExploreJS_RenderChildrenArityAgreement is the regression test for
// DF-OFF-BY-ONE-42: toggleSubNode dispatches to the three render*Children
// functions with a uniform (container, classTitle, parentValue, answers)
// argument list, but renderVersionChildren was declared with only two
// parameters. The real `answers` array therefore landed nowhere, the
// handler threw on answers.forEach, and expanding a version node left
// .tree-children empty — no answer was reachable via the tree.
//
// For each render*Children function, the declaration arity must equal the
// arity of every call site, and the toggleSubNode dispatch must use the
// uniform 4-argument shape like its siblings.
func TestExploreJS_RenderChildrenArityAgreement(t *testing.T) {
	src := fetchExploreJS(t)

	for _, name := range []string{"renderEnvChildren", "renderLangChildren", "renderVersionChildren"} {
		decl := declArity(t, src, name)
		for _, call := range callArities(t, src, name) {
			if call != decl {
				t.Errorf("%s arity mismatch: declaration takes %d arg(s) but a call site passes %d",
					name, decl, call)
			}
		}
	}

	// The version-level dispatch in toggleSubNode must use the same
	// 4-arg shape as the env/lang dispatches it sits next to.
	if !strings.Contains(src, "renderVersionChildren(children, classTitle, parentValue, answers)") {
		t.Errorf("toggleSubNode version dispatch does not use the uniform " +
			"(children, classTitle, parentValue, answers) shape")
	}
}

// TestExploreJS_VersionChildrenRendersAnswerLeaves proves the render chain
// at the deepest drill level: for a class whose taxonomy payload carries
// answers (payload shape proven by internal/api TestTaxonomy_WithData),
// expanding class → environment → language → version ends in
// renderVersionChildren iterating every answer and appending one
// .tree-answer leaf per answer via renderAnswerLeaf.
func TestExploreJS_VersionChildrenRendersAnswerLeaves(t *testing.T) {
	src := fetchExploreJS(t)

	// Extract the renderVersionChildren body.
	declIdx := strings.Index(src, "function renderVersionChildren")
	if declIdx < 0 {
		t.Fatal("renderVersionChildren declaration not found")
	}
	brace := strings.Index(src[declIdx:], "{")
	if brace < 0 {
		t.Fatal("renderVersionChildren has no body")
	}
	bodyStart := declIdx + brace
	depth := 0
	bodyEnd := -1
	for i := bodyStart; i < len(src); i++ {
		switch src[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				bodyEnd = i
			}
		}
		if bodyEnd >= 0 {
			break
		}
	}
	if bodyEnd < 0 {
		t.Fatal("unbalanced braces in renderVersionChildren")
	}
	body := src[bodyStart:bodyEnd]

	// Every answer must be rendered: the body iterates the answers array
	// and appends one leaf per element.
	if !strings.Contains(body, "answers.forEach") {
		t.Errorf("renderVersionChildren does not iterate answers — version nodes render nothing")
	}
	if !strings.Contains(body, "renderAnswerLeaf(") || !strings.Contains(body, "appendChild") {
		t.Errorf("renderVersionChildren does not append renderAnswerLeaf output per answer")
	}

	// The leaf itself carries the .tree-answer class the acceptance
	// criterion looks for.
	leafIdx := strings.Index(src, "function renderAnswerLeaf")
	if leafIdx < 0 {
		t.Fatal("renderAnswerLeaf declaration not found")
	}
	leafBody := src[leafIdx:]
	if end := strings.Index(leafBody, "\n  }\n"); end >= 0 {
		leafBody = leafBody[:end]
	}
	if !strings.Contains(leafBody, "tree-node tree-answer") {
		t.Errorf("renderAnswerLeaf does not build a .tree-answer node")
	}

	// The drill chain above the version level must group answers down
	// through environment → language → version so that the array handed
	// to renderVersionChildren is the class's own answers (filtered).
	for _, needle := range []string{
		"a.env || a.environment || 'any'",
		"a.lang || a.language || 'any'",
		"a.version || 'any'",
	} {
		if !strings.Contains(src, needle) {
			t.Errorf("drill-down grouping key %q missing — chain to version level broken", needle)
		}
	}
}
