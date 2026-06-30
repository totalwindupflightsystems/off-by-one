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
//   - quoted strings ('single' and "double")
//   - bare strings (anything that doesn't look like another scalar)
//   - integers and floats
//   - booleans (true/false/yes/no)
//   - null (~)
//   - comments (# ... to end of line)
//
// Unsupported (will fail loudly — that's fine, our spec uses none of these):
//   - YAML anchors and aliases (*foo, &bar)
//   - flow-style mappings/sequences ({a: b}, [1, 2])
//   - multi-document streams (--- ... ---)
//   - merge keys (<<)
//   - tags (!foo)
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
		text := strings.TrimLeft(line.text, " \t")
		// Find the key/value separator. Keys are unquoted, then ': ' or
		// end-of-line. Quoted keys aren't supported in our subset.
		colon := -1
		for i, r := range text {
			if r == ':' && (i+1 == len(text) || text[i+1] == ' ' || text[i+1] == '\t') {
				colon = i
				break
			}
		}
		if colon < 0 {
			return nil, fmt.Errorf("line %d: expected ':' in mapping entry: %q", line.lineNo, text)
		}
		key := text[:colon]
		key = strings.TrimSpace(key)
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
			synthetic := make([]yamlLine, 0, 4)
			synthetic = append(synthetic, yamlLine{indent: baseIndent, text: strings.Repeat(" ", baseIndent) + rest, lineNo: line.lineNo})
			for *idx < len(lines) {
				next := lines[*idx]
				if next.indent <= baseIndent {
					break
				}
				trimmed := strings.TrimLeft(next.text, " 	")
				// A new sequence item at baseIndent starts with "- ".
				if next.indent == baseIndent && (strings.HasPrefix(trimmed, "- ") || trimmed == "-") {
					break
				}
				synthetic = append(synthetic, next)
				(*idx)++
			}
			subIdx := 0
			child, err := parseYAMLBlock(synthetic, &subIdx, baseIndent)
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
