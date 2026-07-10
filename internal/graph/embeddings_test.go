package graph

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockEmbedder returns a predictable vector. For cosine similarity to work
// correctly, each text maps to a unique direction.
type mockEmbedder struct {
	vectors map[string][]float64
}

func newMockEmbedder() *mockEmbedder {
	// Each dimension vector is [v, 0, 0, ..., 0] so cosine similarity
	// between different embeddings is 0 (orthogonal) and with itself is 1.
	// For realistic tests we use more dimensions.
	return &mockEmbedder{vectors: map[string][]float64{}}
}

func (m *mockEmbedder) Embed(ctx context.Context, text string) ([]float64, error) {
	_ = ctx
	if v, ok := m.vectors[text]; ok {
		return v, nil
	}
	// Generate a deterministic vector from the text hash.
	// This is a simple approach: seed the vector from the length.
	n := len(text)
	vec := make([]float64, 8)
	for i := range vec {
		vec[i] = float64((n+i*73)%173) / 173.0
	}
	m.vectors[text] = vec
	return vec, nil
}

func TestInitEmbeddings(t *testing.T) {
	store, err := OpenShared(t.Name())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	if err := store.InitEmbeddings(ctx); err != nil {
		t.Fatalf("InitEmbeddings: %v", err)
	}
	// Should be idempotent.
	if err := store.InitEmbeddings(ctx); err != nil {
		t.Fatalf("InitEmbeddings second call: %v", err)
	}

	// Verify table exists by querying sqlite_master.
	row := store.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='problem_class_embeddings'`)
	var count int
	if err := row.Scan(&count); err != nil {
		t.Fatalf("check table exists: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected table problem_class_embeddings to exist, got count=%d", count)
	}
}

func TestStoreAndGetEmbedding(t *testing.T) {
	store, err := OpenShared(t.Name())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	if err := store.InitEmbeddings(ctx); err != nil {
		t.Fatalf("InitEmbeddings: %v", err)
	}

	// Create a problem class first.
	classID, err := store.CreateProblemClass(ctx, "test-class", "Test description")
	if err != nil {
		t.Fatalf("CreateProblemClass: %v", err)
	}

	embedding := []float64{0.1, 0.2, 0.3, 0.4, 0.5}

	// Initially no embedding.
	if _, err := store.GetEmbedding(ctx, classID); err == nil {
		t.Fatal("expected ErrNotFound before storing")
	}

	if err := store.StoreEmbedding(ctx, classID, embedding); err != nil {
		t.Fatalf("StoreEmbedding: %v", err)
	}

	// Retrieve and verify.
	got, err := store.GetEmbedding(ctx, classID)
	if err != nil {
		t.Fatalf("GetEmbedding: %v", err)
	}
	if len(got) != len(embedding) {
		t.Fatalf("expected %d dimensions, got %d", len(embedding), len(got))
	}
	for i, v := range embedding {
		if absDiff(v, got[i]) > 1e-9 {
			t.Fatalf("dim[%d]: expected %f, got %f", i, v, got[i])
		}
	}

	// HasEmbedding should return true.
	has, err := store.HasEmbedding(ctx, classID)
	if err != nil {
		t.Fatalf("HasEmbedding: %v", err)
	}
	if !has {
		t.Fatal("expected HasEmbedding to return true")
	}

	// Upsert: store should replace.
	embedding2 := []float64{0.9, 0.8, 0.7, 0.6, 0.5}
	if err := store.StoreEmbedding(ctx, classID, embedding2); err != nil {
		t.Fatalf("StoreEmbedding (upsert): %v", err)
	}
	got2, err := store.GetEmbedding(ctx, classID)
	if err != nil {
		t.Fatalf("GetEmbedding after upsert: %v", err)
	}
	for i, v := range embedding2 {
		if absDiff(v, got2[i]) > 1e-9 {
			t.Fatalf("dim[%d] after upsert: expected %f, got %f", i, v, got2[i])
		}
	}
}

func TestStoreEmbeddingValidation(t *testing.T) {
	store, err := OpenShared(t.Name())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()
	ctx := context.Background()
	store.InitEmbeddings(ctx)

	// Zero classID.
	if err := store.StoreEmbedding(ctx, 0, []float64{1.0}); err == nil {
		t.Fatal("expected error for zero classID")
	}

	// Create class for valid ID tests.
	classID, _ := store.CreateProblemClass(ctx, "validate-class", "desc")

	// Empty embedding.
	if err := store.StoreEmbedding(ctx, classID, nil); err == nil {
		t.Fatal("expected error for empty embedding")
	}
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name string
		a    []float64
		b    []float64
		want float64
	}{
		{"identical", []float64{1, 0, 0}, []float64{1, 0, 0}, 1.0},
		{"orthogonal", []float64{1, 0, 0}, []float64{0, 1, 0}, 0.0},
		{"opposite", []float64{1, 0, 0}, []float64{-1, 0, 0}, -1.0},
		{"zero vector a", []float64{0, 0, 0}, []float64{1, 0, 0}, 0.0},
		{"zero vector b", []float64{1, 0, 0}, []float64{0, 0, 0}, 0.0},
		{"different lengths", []float64{1, 0}, []float64{1, 0, 0}, 0.0},
		{"empty", nil, nil, 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cosineSimilarity(tt.a, tt.b)
			if absDiff(got, tt.want) > 1e-9 {
				t.Fatalf("cosineSimilarity() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestSimilaritySearch(t *testing.T) {
	store, err := OpenShared(t.Name())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()
	ctx := context.Background()
	store.InitEmbeddings(ctx)

	// Create 3 classes with embeddings.
	c1, _ := store.CreateProblemClass(ctx, "python-async", "Python async/await issues")
	c2, _ := store.CreateProblemClass(ctx, "go-goroutines", "Go goroutine and channel issues")
	c3, _ := store.CreateProblemClass(ctx, "rust-tokio", "Rust Tokio async runtime issues")

	// Embeddings that make c1 and c3 close (both async), c2 far.
	store.StoreEmbedding(ctx, c1, []float64{1.0, 0.8, 0.0, 0.0})
	store.StoreEmbedding(ctx, c2, []float64{0.0, 0.0, 1.0, 0.0})
	store.StoreEmbedding(ctx, c3, []float64{0.9, 1.0, 0.0, 0.0})

	// Query with a vector close to c1/c3.
	query := []float64{1.0, 1.0, 0.0, 0.0}
	hits, err := store.SimilaritySearch(ctx, query, 3)
	if err != nil {
		t.Fatalf("SimilaritySearch: %v", err)
	}
	if len(hits) != 3 {
		t.Fatalf("expected 3 hits, got %d", len(hits))
	}

	// c3 should be closest (most aligned with [1.0, 1.0, 0, 0]), then c1.
	if hits[0].Title != "rust-tokio" {
		t.Fatalf("expected rust-tokio as top hit, got %s", hits[0].Title)
	}
}

func TestFlobConversion(t *testing.T) {
	original := []float64{0.1, -0.5, 3.14159, -1e10, 0.0}
	blob := floatsToBlob(original)
	restored := blobToFloats(blob)
	if len(restored) != len(original) {
		t.Fatalf("length mismatch: %d vs %d", len(original), len(restored))
	}
	for i := range original {
		if absDiff(original[i], restored[i]) > 1e-9 {
			t.Fatalf("dim[%d]: %f vs %f", i, original[i], restored[i])
		}
	}
}

func TestOpenRouterEmbedder_HTTPMock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var body struct {
			Model string `json:"model"`
			Input string `json:"input"`
		}
		json.NewDecoder(r.Body).Decode(&body)

		// Return a mock embedding based on the model.
		resp := map[string]interface{}{
			"data": []interface{}{
				map[string]interface{}{
					"embedding": []float64{0.1, 0.2, 0.3},
					"index":     0,
				},
			},
			"model": body.Model,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	embedder := &OpenRouterEmbedder{
		APIKey:  "test-api-key",
		BaseURL: server.URL,
		Model:   "openai/text-embedding-3-small",
	}
	embedder.init()

	ctx := context.Background()
	vec, err := embedder.Embed(ctx, "test text")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vec) != 3 {
		t.Fatalf("expected 3 dims, got %d", len(vec))
	}
	if absDiff(vec[0], 0.1) > 1e-9 {
		t.Fatalf("vec[0] = %f, want 0.1", vec[0])
	}
}

func TestOpenRouterEmbedder_NoAPIKey(t *testing.T) {
	embedder := &OpenRouterEmbedder{}
	embedder.init()
	_, err := embedder.Embed(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}

func TestOpenRouterEmbedder_EmptyText(t *testing.T) {
	embedder := &OpenRouterEmbedder{APIKey: "test-key"}
	embedder.init()
	_, err := embedder.Embed(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty text")
	}
}

func TestOpenRouterEmbedder_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"message": "rate limit exceeded",
			},
		})
	}))
	defer server.Close()

	embedder := &OpenRouterEmbedder{
		APIKey:  "test-key",
		BaseURL: server.URL,
	}
	embedder.init()

	_, err := embedder.Embed(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for 429 response")
	}
}

func TestEmbedAndStoreClass(t *testing.T) {
	store, err := OpenShared(t.Name())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()
	ctx := context.Background()

	classID, _ := store.CreateProblemClass(ctx, "embed-store-test", "A class to embed")
	emb := newMockEmbedder()

	if err := store.EmbedAndStoreClass(ctx, emb, classID, "A class to embed"); err != nil {
		t.Fatalf("EmbedAndStoreClass: %v", err)
	}

	// Verify the embedding was stored.
	_, err = store.GetEmbedding(ctx, classID)
	if err != nil {
		t.Fatalf("expected embedding to be stored: %v", err)
	}
}

func TestEmbedAndStoreClass_NilEmbedder(t *testing.T) {
	store, err := OpenShared(t.Name())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()
	ctx := context.Background()

	classID, _ := store.CreateProblemClass(ctx, "nil-embed-test", "desc")
	// Should be a no-op, not crash.
	if err := store.EmbedAndStoreClass(ctx, nil, classID, "desc"); err != nil {
		t.Fatalf("EmbedAndStoreClass with nil embedder: %v", err)
	}
}

func TestSimilarClasses(t *testing.T) {
	store, err := OpenShared(t.Name())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()
	ctx := context.Background()

	emb := newMockEmbedder()
	// Create two classes and embed them.
	c1, _ := store.CreateProblemClass(ctx, "similar-1", "first test class")
	c2, _ := store.CreateProblemClass(ctx, "similar-2", "second test class")
	store.EmbedAndStoreClass(ctx, emb, c1, "first test class")
	store.EmbedAndStoreClass(ctx, emb, c2, "second test class")

	hits, err := store.SimilarClasses(ctx, emb, "first test class", 5, 0)
	if err != nil {
		t.Fatalf("SimilarClasses: %v", err)
	}
	if len(hits) < 1 {
		t.Fatal("expected at least 1 similar class")
	}
}

func TestDiscoveryWithSimilar(t *testing.T) {
	store, err := OpenShared(t.Name())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()
	ctx := context.Background()

	emb := newMockEmbedder()

	// Create problem class and embed it.
	c1, _ := store.CreateProblemClass(ctx, "discovery-sim", "discovery similarity test")
	c2, _ := store.CreateProblemClass(ctx, "discovery-sim-2", "another similar class")
	store.EmbedAndStoreClass(ctx, emb, c1, "discovery similarity test")
	store.EmbedAndStoreClass(ctx, emb, c2, "another similar class")

	// Add an answer so Discovery returns something.
	store.CreateAnswerNode(ctx, c1, 0, "linux", "go", "1.26", "solution", "evidence", "{}")

	result, err := store.DiscoveryWithSimilar(ctx, emb, "discovery-sim", "linux", "go", "1.26", false)
	if err != nil {
		t.Fatalf("DiscoveryWithSimilar: %v", err)
	}
	if result.Similar == nil {
		t.Fatal("expected Similar hits to be non-nil")
	}
	// The similar classes should include c2 (filtered from c1).
	found := false
	for _, h := range result.Similar {
		if h.Title == "discovery-sim-2" {
			found = true
		}
	}
	if !found {
		t.Logf("Similar hits: %+v", result.Similar)
		t.Fatal("expected discovery-sim-2 in similarity results")
	}
}

func TestDiscoveryWithSimilar_NilEmbedder(t *testing.T) {
	store, err := OpenShared(t.Name())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()
	ctx := context.Background()

	c1, _ := store.CreateProblemClass(ctx, "nil-disco", "nil embedder test")
	store.CreateAnswerNode(ctx, c1, 0, "linux", "go", "1.26", "sol", "ev", "{}")

	result, err := store.DiscoveryWithSimilar(ctx, nil, "nil-disco", "", "", "", false)
	if err != nil {
		t.Fatalf("DiscoveryWithSimilar nil embedder: %v", err)
	}
	if result.Similar != nil {
		t.Fatal("expected Similar to be nil when embedder is nil")
	}
}

func absDiff(a, b float64) float64 {
	if a > b {
		return a - b
	}
	return b - a
}
