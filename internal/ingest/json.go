package ingest

import "encoding/json"

// decodeJSON is a thin wrapper around json.Unmarshal. Exists so the
// HTTP path can swap in a different decoder (e.g., json.Decoder for
// streaming) without changing every call site.
func decodeJSON(body []byte, v any) error {
	return json.Unmarshal(body, v)
}
