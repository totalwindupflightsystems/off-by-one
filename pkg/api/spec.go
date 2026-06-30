// Package api holds the OpenAPI 3.0.3 specification for Off-by-One.
//
// The spec is embedded from openapi.yaml and exposed as a byte slice for
// serving at /openapi.json. Muster (and any other OpenAPI consumer) reads
// this endpoint on startup to auto-discover endpoints and generate MCP
// tools.
package api

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

//go:embed openapi.yaml
var openapiYAML []byte

// SHA256 returns the hex-encoded SHA-256 digest of the embedded spec. Used
// in the ETag header so clients can cache the document.
func SHA256() string {
	sum := sha256.Sum256(openapiYAML)
	return hex.EncodeToString(sum[:])
}

// JSON returns the spec parsed as a generic JSON object. The OpenAPI
// document is JSON-compatible by design, so a hand-rolled line-by-line
// YAML parse is sufficient: the spec is authored as plain YAML (no anchors,
// no merge keys, no flow-style collections), and the only YAML-specific
// syntax used is block-style mappings, sequences, and quoted strings.
//
// We chose to avoid introducing a yaml.v3 dependency here to keep the
// zero-dep bootstrap story intact. The contract is: "the spec at
// /openapi.json is a valid JSON object whose top-level keys are
// openapi/info/servers/paths/components." Muster reads `paths.*` directly.
//
// If a future commit needs full YAML support (anchors, multi-doc, etc.),
// swap in sigs.k8s.io/yaml or gopkg.in/yaml.v3 — the OpenAPIHandler API
// will not change.
func JSON() (map[string]any, error) {
	var spec map[string]any
	if err := decodeYAMLInto(openapiYAML, &spec); err != nil {
		return nil, fmt.Errorf("decode openapi.yaml: %w", err)
	}
	return spec, nil
}

// JSONBytes returns the spec as JSON-encoded bytes. This is what the
// /openapi.json handler serves.
func JSONBytes() ([]byte, error) {
	spec, err := JSON()
	if err != nil {
		return nil, err
	}
	return json.Marshal(spec)
}

// YAMLBytes returns the raw embedded YAML — used by clients that prefer
// YAML over JSON, and as a fallback when JSON parsing fails.
func YAMLBytes() []byte { return openapiYAML }
