package ingest

import (
	"context"
	"testing"
)

// --- decodeJSON ---

func TestDecodeJSON(t *testing.T) {
	var got struct {
		Name string `json:"name"`
	}
	if err := decodeJSON([]byte(`{"name":"test"}`), &got); err != nil {
		t.Fatalf("decodeJSON: %v", err)
	}
	if got.Name != "test" {
		t.Errorf("Name = %q, want test", got.Name)
	}

	// Invalid JSON returns error.
	if err := decodeJSON([]byte(`{not json}`), &got); err == nil {
		t.Error("decodeJSON with invalid JSON should return error")
	}

	// Empty body returns error.
	if err := decodeJSON([]byte(``), &got); err == nil {
		t.Error("decodeJSON with empty body should return error")
	}
}

// --- Store accessor ---

func TestQueue_Store(t *testing.T) {
	q, store := newTestQueue(t)
	got := q.Store()
	if got == nil {
		t.Fatal("Store() returned nil")
	}
	// Verify it's the same store we passed in.
	if got != store {
		t.Error("Store() returned a different *graph.Store than expected")
	}
}

// --- SetStage ---

func TestQueue_SetStage(t *testing.T) {
	q, _ := newTestQueue(t)
	ctx := context.Background()

	// Submit a problem so we have a queue entry.
	id, _, err := q.Submit(ctx, Submission{
		ProblemClass: "test-stage",
		Environment:  "linux",
		Language:     "go",
		Version:      "1.0",
		Cadence:      "post-debug",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	// Update the stage.
	if err := q.SetStage(ctx, id, "sandbox_prepare"); err != nil {
		t.Fatalf("SetStage: %v", err)
	}

	// Verify the stage was persisted.
	entry, err := q.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if entry.Stage != "sandbox_prepare" {
		t.Errorf("Stage = %q, want sandbox_prepare", entry.Stage)
	}

	// Update again.
	if err := q.SetStage(ctx, id, "sandbox_solve"); err != nil {
		t.Fatalf("SetStage(2): %v", err)
	}
	entry2, err := q.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get(2): %v", err)
	}
	if entry2.Stage != "sandbox_solve" {
		t.Errorf("Stage = %q, want sandbox_solve", entry2.Stage)
	}
}

// --- SubmitFromHTTPRequest ---

func TestSubmitFromHTTPRequest(t *testing.T) {
	q, _ := newTestQueue(t)
	ctx := context.Background()

	// Valid JSON body.
	body := []byte(`{
		"problem_class": "http-submit-test",
		"environment": "linux",
		"language": "go",
		"version": "1.0",
		"description": "test problem",
		"error_message": "nil pointer",
		"cadence": "post-debug"
	}`)
	id, _, err := SubmitFromHTTPRequest(ctx, q, body)
	if err != nil {
		t.Fatalf("SubmitFromHTTPRequest: %v", err)
	}
	if id == "" {
		t.Error("returned empty ID")
	}

	// Verify the entry exists.
	entry, err := q.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if entry.ProblemClass != "http-submit-test" {
		t.Errorf("ProblemClass = %q, want http-submit-test", entry.ProblemClass)
	}

	// Invalid JSON returns error.
	_, _, err = SubmitFromHTTPRequest(ctx, q, []byte(`{not json}`))
	if err == nil {
		t.Error("SubmitFromHTTPRequest with invalid JSON should return error")
	}

	// Missing required field (empty problem class) returns validation error.
	_, _, err = SubmitFromHTTPRequest(ctx, q, []byte(`{
		"problem_class": "",
		"environment": "linux",
		"language": "go",
		"version": "1.0",
		"cadence": "post-debug"
	}`))
	if err == nil {
		t.Error("SubmitFromHTTPRequest with empty problem_class should return error")
	}
}
