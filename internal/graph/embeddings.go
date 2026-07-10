// Package graph provides embeddings-powered semantic similarity search over
// problem classes using OpenRouter's embeddings API. Embedding vectors are
// stored in a dedicated SQLite table; cosine similarity ranking surfaces
// semantically similar problem classes even when keyword search misses them.
package graph

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ── Embedder interface ──────────────────────────────────────────────

// Embedder produces vector embeddings for text. Implementations may call
// remote APIs (OpenRouter) or use local models for testing.
type Embedder interface {
	// Embed returns a normalized embedding vector for the given text.
	Embed(ctx context.Context, text string) ([]float64, error)
}

// ── OpenRouter embedder ─────────────────────────────────────────────

// OpenRouterEmbedder calls the OpenRouter embeddings API. It requires an
// API key (OPENROUTER_API_KEY) and optionally a model name.
type OpenRouterEmbedder struct {
	APIKey    string
	BaseURL   string
	Model     string
	Client    *http.Client
	initOnce  sync.Once
}

// DefaultOpenRouterEmbeddingModel is the model name for OpenRouter embeddings.
// openai/text-embedding-3-small produces 1536-dimensional vectors at low cost.
const DefaultOpenRouterEmbeddingModel = "openai/text-embedding-3-small"

func (e *OpenRouterEmbedder) init() {
	if e.BaseURL == "" {
		e.BaseURL = "https://openrouter.ai/api/v1"
	}
	if e.Model == "" {
		e.Model = DefaultOpenRouterEmbeddingModel
	}
	if e.Client == nil {
		e.Client = &http.Client{Timeout: 30 * time.Second}
	}
}

// Embed calls the OpenRouter embeddings API and returns the embedding vector.
func (e *OpenRouterEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
	e.initOnce.Do(e.init)
	if e.APIKey == "" {
		return nil, errors.New("graph: OPENROUTER_API_KEY is empty")
	}
	if text == "" {
		return nil, errors.New("graph: text must not be empty")
	}

	body := map[string]interface{}{
		"model": e.Model,
		"input": text,
	}
	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("graph: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		e.BaseURL+"/embeddings", bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("graph: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+e.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("graph: embed request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errBody struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errBody)
		if errBody.Error.Message != "" {
			return nil, fmt.Errorf("graph: embeddings API returned %d: %s", resp.StatusCode, errBody.Error.Message)
		}
		return nil, fmt.Errorf("graph: embeddings API returned %d", resp.StatusCode)
	}

	var result struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("graph: decode response: %w", err)
	}
	if len(result.Data) == 0 {
		return nil, errors.New("graph: no embedding returned")
	}
	return result.Data[0].Embedding, nil
}

// ── Embedding storage schema ────────────────────────────────────────

const embeddingsSchema = `
CREATE TABLE IF NOT EXISTS problem_class_embeddings (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    class_id    INTEGER NOT NULL UNIQUE REFERENCES problem_classes(id),
    embedding   BLOB NOT NULL,
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);
`

// InitEmbeddings ensures the embeddings table exists. Safe to call
// multiple times — uses IF NOT EXISTS.
func (s *Store) InitEmbeddings(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, embeddingsSchema); err != nil {
		return fmt.Errorf("graph: init embeddings: %w", err)
	}
	return nil
}

// ── Store embedding ─────────────────────────────────────────────────

// StoreEmbedding serialises the embedding vector as little-endian float64
// bytes and upserts into the problem_class_embeddings table.
func (s *Store) StoreEmbedding(ctx context.Context, classID int64, embedding []float64) error {
	if classID <= 0 {
		return errors.New("graph: classID must be positive")
	}
	if len(embedding) == 0 {
		return errors.New("graph: embedding must not be empty")
	}
	blob := floatsToBlob(embedding)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO problem_class_embeddings (class_id, embedding) VALUES (?, ?)
		 ON CONFLICT(class_id) DO UPDATE SET embedding = excluded.embedding`,
		classID, blob)
	if err != nil {
		return fmt.Errorf("graph: store embedding: %w", err)
	}
	return nil
}

// GetEmbedding retrieves the stored embedding vector for a problem class.
// Returns ErrNotFound if no embedding has been stored.
func (s *Store) GetEmbedding(ctx context.Context, classID int64) ([]float64, error) {
	var blob []byte
	err := s.db.QueryRowContext(ctx,
		`SELECT embedding FROM problem_class_embeddings WHERE class_id = ?`, classID,
	).Scan(&blob)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("graph: get embedding: %w", err)
	}
	return blobToFloats(blob), nil
}

// HasEmbedding returns true if an embedding exists for the given class ID.
func (s *Store) HasEmbedding(ctx context.Context, classID int64) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM problem_class_embeddings WHERE class_id = ?`, classID,
	).Scan(&count)
	return count > 0, err
}

// ── Cosine similarity search ────────────────────────────────────────

// SimilarityHit is a single result from an embedding-based similarity search.
type SimilarityHit struct {
	ClassID    int64
	Title      string
	Similarity float64
}

// SimilaritySearch finds problem classes whose embeddings are most similar
// to the query vector using cosine similarity. Returns up to `limit` results
// ordered by descending similarity. Only returns classes that have embeddings
// stored.
func (s *Store) SimilaritySearch(ctx context.Context, queryVec []float64, limit int) ([]SimilarityHit, error) {
	if len(queryVec) == 0 {
		return nil, errors.New("graph: query vector must not be empty")
	}
	if limit <= 0 || limit > 100 {
		limit = 10
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT e.class_id, pc.title, e.embedding
		FROM problem_class_embeddings e
		JOIN problem_classes pc ON pc.id = e.class_id
	`)
	if err != nil {
		return nil, fmt.Errorf("graph: similarity search: %w", err)
	}
	defer rows.Close()

	var candidates []candidate
	for rows.Next() {
		var h SimilarityHit
		var blob []byte
		if err := rows.Scan(&h.ClassID, &h.Title, &blob); err != nil {
			return nil, err
		}
		vec := blobToFloats(blob)
		h.Similarity = cosineSimilarity(queryVec, vec)
		candidates = append(candidates, candidate{hit: h, vector: vec})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Sort by similarity descending, keep top limit.
	// Simple insertion sort — the number of embeddings is expected to be
	// small (hundreds, not millions). For larger collections, switch to
	// a vector extension or approximate nearest neighbours.
	sortBySimilarity(candidates, limit)

	out := make([]SimilarityHit, 0, limit)
	for i := range candidates {
		if i >= limit {
			break
		}
		out = append(out, candidates[i].hit)
	}
	return out, nil
}

// ── Convenience: Embed + search ─────────────────────────────────────

// SimilarClasses embeds the given text and returns the top-N most similar
// problem classes. A nil Embedder means "not configured" — return empty slice.
func (s *Store) SimilarClasses(ctx context.Context, emb Embedder, text string, limit int, excludeClassID int64) ([]SimilarityHit, error) {
	if emb == nil {
		return nil, nil
	}
	vec, err := emb.Embed(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("graph: similar classes embed: %w", err)
	}
	hits, err := s.SimilaritySearch(ctx, vec, limit)
	if err != nil {
		return nil, fmt.Errorf("graph: similar classes search: %w", err)
	}
	// Filter out the excluded class.
	filtered := hits[:0]
	for _, h := range hits {
		if h.ClassID != excludeClassID {
			filtered = append(filtered, h)
		}
	}
	return filtered, nil
}

// ── Embed + store for a problem class ───────────────────────────────

// EmbedAndStoreClass embeds the description and stores it for the given
// problem class. A nil Embedder is a no-op.
func (s *Store) EmbedAndStoreClass(ctx context.Context, emb Embedder, classID int64, description string) error {
	if emb == nil {
		return nil
	}
	if err := s.InitEmbeddings(ctx); err != nil {
		return err
	}
	// Combine title + description for a richer embedding.
	vec, err := emb.Embed(ctx, description)
	if err != nil {
		return fmt.Errorf("graph: embed class: %w", err)
	}
	return s.StoreEmbedding(ctx, classID, vec)
}

// ── Helpers ─────────────────────────────────────────────────────────

// float64 → []byte (little-endian)
func floatsToBlob(vec []float64) []byte {
	out := make([]byte, len(vec)*8)
	for i, v := range vec {
		bits := math.Float64bits(v)
		out[i*8] = byte(bits)
		out[i*8+1] = byte(bits >> 8)
		out[i*8+2] = byte(bits >> 16)
		out[i*8+3] = byte(bits >> 24)
		out[i*8+4] = byte(bits >> 32)
		out[i*8+5] = byte(bits >> 40)
		out[i*8+6] = byte(bits >> 48)
		out[i*8+7] = byte(bits >> 56)
	}
	return out
}

// []byte → []float64
func blobToFloats(blob []byte) []float64 {
	n := len(blob) / 8
	out := make([]float64, n)
	for i := range out {
		bits := uint64(blob[i*8]) |
			uint64(blob[i*8+1])<<8 |
			uint64(blob[i*8+2])<<16 |
			uint64(blob[i*8+3])<<24 |
			uint64(blob[i*8+4])<<32 |
			uint64(blob[i*8+5])<<40 |
			uint64(blob[i*8+6])<<48 |
			uint64(blob[i*8+7])<<56
		out[i] = math.Float64frombits(bits)
	}
	return out
}

// cosineSimilarity returns the cosine similarity between two vectors.
// Returns 0.0 if either vector has zero magnitude.
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0.0
	}
	var dot, magA, magB float64
	for i := range a {
		dot += a[i] * b[i]
		magA += a[i] * a[i]
		magB += b[i] * b[i]
	}
	if magA == 0 || magB == 0 {
		return 0.0
	}
	return dot / (math.Sqrt(magA) * math.Sqrt(magB))
}

// candidate pairs a similarity hit with its raw vector for sorting.
type candidate struct {
	hit    SimilarityHit
	vector []float64
}

// sortBySimilarity performs a simple selection sort of the top `limit`
// candidates by similarity descending. For small N (<1000) this is fine;
// switch to a heap for larger collections.
func sortBySimilarity(candidates []candidate, limit int) {
	if len(candidates) <= 1 {
		return
	}
	for i := 0; i < len(candidates) && i < limit; i++ {
		best := i
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].hit.Similarity > candidates[best].hit.Similarity {
				best = j
			}
		}
		if best != i {
			candidates[i], candidates[best] = candidates[best], candidates[i]
		}
	}
}

// ── Wire into Discovery ─────────────────────────────────────────────

// DiscoveryResultWithSimilar extends DiscoveryResult with embedding-based
// similarity hits. The Similar field is populated when an Embedder is
// configured and DiscoveryWithSimilar is called instead of Discovery.
type DiscoveryResultWithSimilar struct {
	DiscoveryResult
	Similar []SimilarityHit
}

// DiscoveryWithSimilar is like Discovery but also runs an embedding-based
// similarity search using the problem class description. When emb is nil,
// this is identical to Discovery.
func (s *Store) DiscoveryWithSimilar(ctx context.Context, emb Embedder, title, env, lang, version string, includeRelated bool) (*DiscoveryResultWithSimilar, error) {
	base, err := s.Discovery(ctx, title, env, lang, version, includeRelated)
	if err != nil {
		return nil, err
	}
	result := &DiscoveryResultWithSimilar{DiscoveryResult: *base}

	if emb == nil {
		return result, nil
	}
	if err := s.InitEmbeddings(ctx); err != nil {
		return nil, err
	}

	// Embed the problem class description and search for similar.
	desc := strings.TrimSpace(base.Class.Description)
	if desc == "" {
		desc = base.Class.Title
	}
	hits, err := s.SimilarClasses(ctx, emb, desc, 10, base.Class.ID)
	if err != nil {
		return nil, err
	}
	result.Similar = hits
	return result, nil
}
