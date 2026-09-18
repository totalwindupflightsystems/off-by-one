package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// jsonMarshal and jsonUnmarshal are thin wrappers around the encoding/json
// package so the call sites in decodeYAMLInto read naturally. They exist
// only as a layer of indirection — there is no test stub or alternative
// implementation behind them.
func jsonMarshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	// json.Encoder.Encode appends a trailing newline; strip it for byte-
	// exact output consistency with json.Marshal.
	out := buf.Bytes()
	if n := len(out); n > 0 && out[n-1] == '\n' {
		out = out[:n-1]
	}
	return out, nil
}

func jsonUnmarshal(data []byte, target any) error {
	return json.Unmarshal(data, target)
}

// decodeYAMLInto parses a YAML document into the target value. `target` must
// be a pointer. The supported subset is intentionally narrow — exactly
// what we need to parse the OpenAPI 3.0.3 spec at openapi.yaml.
//
// Supported syntax:
//   - block-style mappings (key: value, with indented continuations)
//   - block-style sequences (lines beginning with `- `)
//   - flow-style sequences and mappings ([a, b], {a: b}, and nested forms
//     thereof) on a single line
//   - quoted strings ('single' and "double")
//   - quoted mapping keys ('200': and "200": are unquoted on parse)
//   - bare strings (anything that doesn't look like another scalar)
//   - integers and floats
//   - booleans (true/false/yes/no)
//   - null (~)
//   - comments (# ... to end of line)
//
// Unsupported (will fail loudly — that's fine, our spec uses none of these):
//   - YAML anchors and aliases (*foo, &bar)
//   - multi-line flow collections (a flow collection must open and close on
//     one line; an unclosed [ or { is kept as a plain string)
//   - multi-document streams (--- ... ---)
//   - merge keys (<<)
//   - tags (!foo)
//   - block-scalar chomping/indentation indicators (|-, >+, |2)
//
// If a future spec needs any of these, swap in sigs.k8s.io/yaml.
func decodeYAMLInto(data []byte, target any) error {
	if target == nil {
		return fmt.Errorf("decodeYAMLInto: target must be a non-nil pointer")
	}
	doc, err := parseYAML(data)
	if err != nil {
		return err
	}
	// Re-marshal the parsed YAML value through JSON to leverage
	// encoding/json's well-tested reflection rules for the assignment.
	// The parsed values are already JSON-compatible (strings, numbers,
	// booleans, nil, map[string]any, []any), so the round-trip is a
	// safe and cheap way to handle *map[string]any, *[]any, and
	// concrete struct targets uniformly.
	buf, err := jsonMarshal(doc)
	if err != nil {
		return fmt.Errorf("re-marshal parsed yaml: %w", err)
	}
	return jsonUnmarshal(buf, target)
}

// --- YAML parser ---------------------------------------------------------

type yamlNode struct {
	kind   yamlKind
	value  any      // string, int64, float64, bool, nil
	keys   []string // for mapping: ordered key list
	values map[string]*yamlNode
	items  []*yamlNode // for sequence
	line   int
}

type yamlKind int

const (
	ykScalar yamlKind = iota
	ykMapping
	ykSequence
	ykNull
)

func parseYAML(data []byte) (any, error) {
	// Tokenize into logical blocks (mappings/sequences at the same indent).
	lines, err := splitYAMLLines(data)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, nil
	}
	first := lines[0]
	// Top-level may be:
	//   - a bare scalar (one line, no colon-space, no leading "-")
	//   - a mapping (line with "key: value" or "key:" with continuations)
	//   - a sequence (line beginning with "- " or just "-")
	content := strings.TrimLeft(first.text, " 	")
	isMapping := containsKeyColon(content)
	isSequence := strings.HasPrefix(content, "- ") || content == "-"
	if !isMapping && !isSequence {
		return yamlToValue(parseYAMLScalar(content, first.lineNo)), nil
	}
	idx := 0
	if isSequence {
		// Top-level sequence: parse the whole thing at the first line's indent.
		// (Most YAML documents start at indent 0, but be defensive.)
		seq, err := parseYAMLSequence(lines, &idx, first.indent)
		if err != nil {
			return nil, err
		}
		return yamlToValue(seq), nil
	}
	val, err := parseYAMLBlock(lines, &idx, 0)
	if err != nil {
		return nil, err
	}
	return yamlToValue(val), nil
}

// containsKeyColon reports whether s contains an unquoted "key: " or "key:"
// sequence. Used to disambiguate a bare scalar top-level (e.g., "hello world")
// from a one-line mapping (e.g., "key: value").
func containsKeyColon(s string) bool {
	inSingle, inDouble := false, false
	for i, r := range s {
		switch r {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case ':':
			if !inSingle && !inDouble && (i+1 == len(s) || s[i+1] == ' ' || s[i+1] == '	') {
				return true
			}
		}
	}
	return false
}

type yamlLine struct {
	indent int
	text   string
	lineNo int
}

func splitYAMLLines(data []byte) ([]yamlLine, error) {
	var out []yamlLine
	raw := strings.Split(string(data), "\n")
	for i, s := range raw {
		// Strip trailing \r.
		s = strings.TrimRight(s, "\r")
		// Empty line.
		if strings.TrimSpace(s) == "" {
			continue
		}
		// Comment-only line.
		trimmed := strings.TrimLeft(s, " \t")
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := 0
		for _, r := range s {
			if r == ' ' {
				indent++
			} else if r == '\t' {
				// Tabs are forbidden in YAML for indentation.
				return nil, fmt.Errorf("line %d: tab in indentation", i+1)
			} else {
				break
			}
		}
		out = append(out, yamlLine{indent: indent, text: s, lineNo: i + 1})
	}
	return out, nil
}

func parseYAMLBlock(lines []yamlLine, idx *int, baseIndent int) (*yamlNode, error) {
	if *idx >= len(lines) {
		return nil, nil
	}
	first := lines[*idx]
	if first.indent < baseIndent {
		return nil, nil
	}
	if first.indent > baseIndent {
		return nil, fmt.Errorf("line %d: unexpected indent %d (expected %d)", first.lineNo, first.indent, baseIndent)
	}

	// Peek the first non-empty content to decide mapping vs sequence.
	content := strings.TrimLeft(first.text, " 	")
	if strings.HasPrefix(content, "- ") || content == "-" {
		return parseYAMLSequence(lines, idx, baseIndent)
	}
	return parseYAMLMapping(lines, idx, baseIndent)
}
func parseYAMLMapping(lines []yamlLine, idx *int, baseIndent int) (*yamlNode, error) {
	node := &yamlNode{kind: ykMapping, values: map[string]*yamlNode{}}
	for *idx < len(lines) && lines[*idx].indent == baseIndent {
		line := lines[*idx]
		// Consume this line.
		(*idx)++
		text := strings.TrimLeft(line.text, " 	")
		// Find the key/value separator: an unquoted ':' followed by
		// whitespace or end-of-line. The scan tracks quote state so a
		// colon INSIDE a quoted key is never chosen as the separator.
		colon := findMappingSeparator(text)
		if colon < 0 {
			return nil, fmt.Errorf("line %d: expected ':' in mapping entry: %q", line.lineNo, text)
		}
		// Keys may be bare or quoted; quoted keys are unquoted here so the
		// emitted JSON carries `200`, not `'200'`.
		key := unquoteYAMLKey(strings.TrimSpace(text[:colon]))
		if key == "" {
			return nil, fmt.Errorf("line %d: empty key in mapping", line.lineNo)
		}
		rest := text[colon+1:]
		rest = strings.TrimLeft(rest, " \t")

		if rest == "" {
			// Value is on subsequent indented lines.
			if *idx >= len(lines) || lines[*idx].indent <= baseIndent {
				// Empty mapping value — treat as null.
				node.keys = append(node.keys, key)
				node.values[key] = &yamlNode{kind: ykNull, line: line.lineNo}
				continue
			}
			childIndent := lines[*idx].indent
			child, err := parseYAMLBlock(lines, idx, childIndent)
			if err != nil {
				return nil, err
			}
			node.keys = append(node.keys, key)
			node.values[key] = child
			continue
		}

		// Block scalar indicator: "|" (literal, newlines preserved) or
		// ">" (folded, newlines become spaces). We support the simple
		// forms — no chomping indicators (-, +, nothing) beyond defaults.
		if rest == "|" || rest == ">" {
			block := parseYAMLBlockScalar(lines, idx, baseIndent, rest == "|", line.lineNo)
			node.keys = append(node.keys, key)
			node.values[key] = block
			continue
		}

		// Inline scalar value.
		node.keys = append(node.keys, key)
		node.values[key] = parseYAMLScalar(rest, line.lineNo)
	}
	return node, nil
}

func parseYAMLSequence(lines []yamlLine, idx *int, baseIndent int) (*yamlNode, error) {
	node := &yamlNode{kind: ykSequence}
	for *idx < len(lines) && lines[*idx].indent == baseIndent {
		line := lines[*idx]
		text := strings.TrimLeft(line.text, " 	")
		if !strings.HasPrefix(text, "- ") && text != "-" {
			return node, nil
		}
		// Consume the dash.
		(*idx)++
		rest := text[1:]
		rest = strings.TrimLeft(rest, " 	")
		if rest == "" {
			// Sequence item starts on subsequent indented lines.
			if *idx >= len(lines) || lines[*idx].indent <= baseIndent {
				node.items = append(node.items, &yamlNode{kind: ykNull, line: line.lineNo})
				continue
			}
			childIndent := lines[*idx].indent
			child, err := parseYAMLBlock(lines, idx, childIndent)
			if err != nil {
				return nil, err
			}
			node.items = append(node.items, child)
			continue
		}
		// Inline item — could be a scalar or the start of an inline mapping.
		// Continuation lines of an inline mapping appear at indent STRICTLY
		// GREATER than baseIndent (the mapping's deeper keys sit two spaces
		// past the dash). We collect every such line as part of the
		// synthetic mapping block, then stop when we hit either a new
		// sequence item (a line at baseIndent that begins with "- ") or
		// a line at indent <= baseIndent (a sibling in the parent block).
		if isInlineMappingStart(rest) {
			// The synthetic block must be parsed at the inline mapping's
			// OWN indentation — the column where its first key starts, just
			// past the "- " marker — not at the sequence's baseIndent.
			// parseYAMLMapping only consumes lines at its baseIndent, so
			// parsing at baseIndent (the old behaviour) dropped every
			// continuation key: `- name: q` followed by `in: query` /
			// `required: true` / `schema:` yielded {name: q} and silently
			// discarded the rest, because the collection loop below had
			// already advanced past those lines.
			//
			// mappingIndent starts at the first key's column and is lowered
			// to the shallowest collected line: a continuation key at the
			// mapping's column is then parsed by the same mapping (the
			// normal case), while a DEEPER line that is not a key — e.g.
			// block-scalar content under `description: |` — cannot drag the
			// mapping's base indent down with it and lose its own content.
			mappingIndent := line.indent + (len(text) - len(rest))
			synthetic := make([]yamlLine, 0, 4)
			for *idx < len(lines) {
				next := lines[*idx]
				if next.indent <= baseIndent {
					break
				}
				if next.indent < mappingIndent {
					mappingIndent = next.indent
				}
				synthetic = append(synthetic, next)
				(*idx)++
			}
			// Re-add the first line at the resolved indent, first in order.
			synthetic = append([]yamlLine{{
				indent: mappingIndent,
				text:   strings.Repeat(" ", mappingIndent) + rest,
				lineNo: line.lineNo,
			}}, synthetic...)
			subIdx := 0
			child, err := parseYAMLBlock(synthetic, &subIdx, mappingIndent)
			if err != nil {
				return nil, err
			}
			node.items = append(node.items, child)
			continue
		}
		// Plain scalar.
		node.items = append(node.items, parseYAMLScalar(rest, line.lineNo))
	}
	return node, nil
}

func isLikelyScalar(s string) bool {
	// A string is "likely a scalar" if it has no colon followed by space/EOF
	// (which would make it a mapping key). Strings that DO have a colon
	// are mapping starts unless the colon is inside quotes.
	inSingle, inDouble := false, false
	for i, r := range s {
		switch r {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case ':':
			if !inSingle && !inDouble && (i+1 == len(s) || s[i+1] == ' ' || s[i+1] == '\t') {
				return false
			}
		}
	}
	return true
}

func isInlineMappingStart(s string) bool {
	return !isLikelyScalar(s)
}

// parseYAMLBlockScalar reads a YAML block scalar (| or >) starting at the
// current line index. The content lines must be more indented than
// baseIndent. Returns a scalar node with the joined text.
//
// Within a block scalar, content may have variable indentation — the
// canonical example is a YAML literal block whose first line is a
// bulleted list (the list dashes are at deeper indent than the rest of
// the text). We collect every line whose indent is greater than
// baseIndent, dedenting by baseIndent+1 (the minimum block content
// indent), and stop when we hit a line at indent <= baseIndent.
func parseYAMLBlockScalar(lines []yamlLine, idx *int, baseIndent int, literal bool, lineNo int) *yamlNode {
	if *idx >= len(lines) || lines[*idx].indent <= baseIndent {
		return &yamlNode{kind: ykScalar, value: "", line: lineNo}
	}
	minContentIndent := -1
	var collected []yamlLine
	for *idx < len(lines) {
		l := lines[*idx]
		if l.indent <= baseIndent {
			break
		}
		// Skip empty (blank) lines — they live in the block as separators
		// and are preserved.
		if strings.TrimSpace(l.text) == "" {
			collected = append(collected, yamlLine{indent: 0, text: ""})
			(*idx)++
			continue
		}
		if minContentIndent < 0 || l.indent < minContentIndent {
			minContentIndent = l.indent
		}
		collected = append(collected, l)
		(*idx)++
	}
	// Re-emit: dedent each line by minContentIndent.
	stripped := make([]string, 0, len(collected))
	for _, l := range collected {
		if l.text == "" {
			stripped = append(stripped, "")
			continue
		}
		if len(l.text) < minContentIndent {
			stripped = append(stripped, "")
			continue
		}
		stripped = append(stripped, l.text[minContentIndent:])
	}
	if literal {
		return &yamlNode{kind: ykScalar, value: strings.Join(stripped, "\n"), line: lineNo}
	}
	// Folded: newlines within paragraphs become spaces; blank lines separate
	// paragraphs (kept as newlines).
	var out strings.Builder
	for i, line := range stripped {
		if line == "" {
			if out.Len() > 0 {
				out.WriteByte('\n')
			}
			continue
		}
		if i > 0 && stripped[i-1] != "" {
			out.WriteByte(' ')
		}
		out.WriteString(line)
	}
	return &yamlNode{kind: ykScalar, value: out.String(), line: lineNo}
}

func parseYAMLScalar(text string, lineNo int) *yamlNode {
	text = strings.TrimRight(text, " \t")
	// Strip trailing inline comments outside quotes.
	text = stripInlineComment(text)
	text = strings.TrimRight(text, " \t")
	if text == "" || text == "~" || text == "null" || text == "Null" || text == "NULL" {
		return &yamlNode{kind: ykNull, line: lineNo}
	}
	switch strings.ToLower(text) {
	case "true", "yes", "on":
		return &yamlNode{kind: ykScalar, value: true, line: lineNo}
	case "false", "no", "off":
		return &yamlNode{kind: ykScalar, value: false, line: lineNo}
	}
	// Quoted strings.
	if len(text) >= 2 {
		if text[0] == '"' && text[len(text)-1] == '"' {
			return &yamlNode{kind: ykScalar, value: unescapeDoubleQuoted(text[1 : len(text)-1]), line: lineNo}
		}
		if text[0] == '\'' && text[len(text)-1] == '\'' {
			return &yamlNode{kind: ykScalar, value: text[1 : len(text)-1], line: lineNo}
		}
	}
	// Flow-style collections: a balanced "[a, b]" decodes to a sequence and a
	// balanced "{a: b}" to a mapping. This runs before the number/bare-string
	// fallbacks so a flow sequence is never emitted as a single string.
	if flow, ok := parseYAMLFlow(text, lineNo); ok {
		return flow
	}
	// Numbers.
	if i, err := strconv.ParseInt(text, 10, 64); err == nil {
		return &yamlNode{kind: ykScalar, value: i, line: lineNo}
	}
	if f, err := strconv.ParseFloat(text, 64); err == nil {
		return &yamlNode{kind: ykScalar, value: f, line: lineNo}
	}
	// Bare string.
	return &yamlNode{kind: ykScalar, value: text, line: lineNo}
}

// findMappingSeparator returns the byte index of the ':' that separates a
// block-mapping key from its value, or -1 when the line has no separator. A
// separator is an UNQUOTED ':' followed by whitespace or end-of-line. Quote
// state is tracked (with backslash escapes honoured inside double quotes, as
// unescapeDoubleQuoted does) so a colon inside a quoted key is never chosen.
func findMappingSeparator(s string) int {
	inSingle, inDouble, escaped := false, false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case escaped:
			escaped = false
		case inDouble && c == '\\':
			escaped = true
		case c == '\'' && !inDouble:
			inSingle = !inSingle
		case c == '"' && !inSingle:
			inDouble = !inDouble
		case c == ':' && !inSingle && !inDouble:
			if i+1 == len(s) || s[i+1] == ' ' || s[i+1] == '\t' {
				return i
			}
		}
	}
	return -1
}

// unquoteYAMLKey strips one matching pair of surrounding quotes from a
// mapping key and resolves the in-quote escape, matching YAML's rules for
// scalar keys: double quotes via unescapeDoubleQuoted, and single quotes
// where a doubled single quote stands for one literal apostrophe. Unquoted
// keys are returned unchanged, as are strings that only look quoted on one
// side ("'200" keeps its quote).
func unquoteYAMLKey(key string) string {
	if len(key) >= 2 {
		if key[0] == '"' && key[len(key)-1] == '"' {
			return unescapeDoubleQuoted(key[1 : len(key)-1])
		}
		if key[0] == '\'' && key[len(key)-1] == '\'' {
			return strings.ReplaceAll(key[1:len(key)-1], "''", "'")
		}
	}
	return key
}

// parseYAMLFlow decodes a single-line flow-style collection ("[a, b]" or
// "{k: v}", including nested forms) into a ykSequence/ykMapping node. It
// reports ok=false when text is not a balanced collection of that kind — an
// unclosed "[a, b" or a trailing "]" is left to the callers' scalar fallback,
// so malformed values degrade to a plain string instead of failing the parse.
func parseYAMLFlow(text string, lineNo int) (*yamlNode, bool) {
	if len(text) < 2 {
		return nil, false
	}
	var closer byte
	switch text[0] {
	case '[':
		closer = ']'
	case '{':
		closer = '}'
	default:
		return nil, false
	}
	inner, ok := flowInterior(text, closer)
	if !ok {
		return nil, false
	}
	parts := splitFlowTopLevel(inner)
	if text[0] == '[' {
		node := &yamlNode{kind: ykSequence, line: lineNo}
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			node.items = append(node.items, parseYAMLScalar(part, lineNo))
		}
		return node, true
	}
	node := &yamlNode{kind: ykMapping, values: map[string]*yamlNode{}, line: lineNo}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		colon := findFlowColon(part)
		var key string
		if colon < 0 {
			// A bare entry in a flow mapping is YAML shorthand for a null
			// value: "{a, b}" is {a: null, b: null}.
			key = unquoteYAMLKey(part)
			node.keys = append(node.keys, key)
			node.values[key] = &yamlNode{kind: ykNull, line: lineNo}
			continue
		}
		key = unquoteYAMLKey(strings.TrimSpace(part[:colon]))
		value := strings.TrimSpace(part[colon+1:])
		node.keys = append(node.keys, key)
		node.values[key] = parseYAMLScalar(value, lineNo)
	}
	return node, true
}

// flowInterior returns the text between the opening bracket at s[0] and its
// matching closer, and reports whether s is exactly one balanced collection.
// The bracket kinds must nest properly ("[a}" is rejected) and nothing but
// the closer may follow the matching one, so "[a] b" is not a flow sequence.
func flowInterior(s string, closer byte) (string, bool) {
	stack := []byte{closer}
	inSingle, inDouble, escaped := false, false, false
	for i := 1; i < len(s); i++ {
		c := s[i]
		switch {
		case escaped:
			escaped = false
			continue
		case inDouble:
			if c == '\\' {
				escaped = true
			} else if c == '"' {
				inDouble = false
			}
			continue
		case inSingle:
			if c == '\'' {
				if i+1 < len(s) && s[i+1] == '\'' {
					// '' is an escaped quote, not the end of the string.
					i++
					continue
				}
				inSingle = false
			}
			continue
		}
		switch c {
		case '\'':
			inSingle = true
		case '"':
			inDouble = true
		case '[':
			stack = append(stack, ']')
		case '{':
			stack = append(stack, '}')
		case ']', '}':
			if c != stack[len(stack)-1] {
				return "", false
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				if i != len(s)-1 {
					return "", false
				}
				return s[1:i], true
			}
		}
	}
	return "", false
}

// splitFlowTopLevel splits a flow-collection interior on commas that sit at
// nesting depth zero and outside quotes, so ["a, b", c] yields two elements
// and [[a], [b]] yields two nested collections.
func splitFlowTopLevel(s string) []string {
	var parts []string
	depth, start := 0, 0
	inSingle, inDouble, escaped := false, false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case escaped:
			escaped = false
			continue
		case inDouble:
			if c == '\\' {
				escaped = true
			} else if c == '"' {
				inDouble = false
			}
			continue
		case inSingle:
			if c == '\'' {
				if i+1 < len(s) && s[i+1] == '\'' {
					i++
					continue
				}
				inSingle = false
			}
			continue
		}
		switch c {
		case '\'':
			inSingle = true
		case '"':
			inDouble = true
		case '[', '{':
			depth++
		case ']', '}':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, s[start:])
}

// findFlowColon returns the byte index of the first top-level unquoted ':' in
// a flow-mapping entry, or -1. Unlike block mappings, a flow entry needs no
// trailing space after the colon ("{a:1}" and "{\"a\":1}" are both accepted).
func findFlowColon(s string) int {
	depth := 0
	inSingle, inDouble, escaped := false, false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case escaped:
			escaped = false
			continue
		case inDouble:
			if c == '\\' {
				escaped = true
			} else if c == '"' {
				inDouble = false
			}
			continue
		case inSingle:
			if c == '\'' {
				if i+1 < len(s) && s[i+1] == '\'' {
					i++
					continue
				}
				inSingle = false
			}
			continue
		}
		switch c {
		case '\'':
			inSingle = true
		case '"':
			inDouble = true
		case '[', '{':
			depth++
		case ']', '}':
			if depth > 0 {
				depth--
			}
		case ':':
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func stripInlineComment(s string) string {
	inSingle, inDouble := false, false
	for i, r := range s {
		switch r {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble {
				// Must be preceded by whitespace to be a comment.
				if i == 0 || unicode.IsSpace(rune(s[i-1])) {
					return s[:i]
				}
			}
		}
	}
	return s
}

func unescapeDoubleQuoted(s string) string {
	// Handle a small subset: \\ \" \n \t \r. Other backslash sequences pass
	// through literally — our spec doesn't use any.
	var b bytes.Buffer
	escape := false
	for _, r := range s {
		if escape {
			switch r {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case '\\':
				b.WriteByte('\\')
			case '"':
				b.WriteByte('"')
			default:
				b.WriteByte('\\')
				b.WriteRune(r)
			}
			escape = false
			continue
		}
		if r == '\\' {
			escape = true
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// yamlToValue converts a parsed yamlNode into a Go value matching the
// shape that encoding/json expects when unmarshalling into a generic
// map[string]any / []any / scalar.
func yamlToValue(n *yamlNode) any {
	if n == nil {
		return nil
	}
	switch n.kind {
	case ykNull:
		return nil
	case ykScalar:
		return n.value
	case ykMapping:
		out := make(map[string]any, len(n.keys))
		for _, k := range n.keys {
			out[k] = yamlToValue(n.values[k])
		}
		return out
	case ykSequence:
		out := make([]any, 0, len(n.items))
		for _, item := range n.items {
			out = append(out, yamlToValue(item))
		}
		return out
	}
	return nil
}
