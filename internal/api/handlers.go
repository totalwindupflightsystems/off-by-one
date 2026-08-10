package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/totalwindupflightsystems/off-by-one/internal/export"
	"github.com/totalwindupflightsystems/off-by-one/internal/graph"
	importgit "github.com/totalwindupflightsystems/off-by-one/internal/import"
	"github.com/totalwindupflightsystems/off-by-one/internal/ingest"
)

// --- Request types -------------------------------------------------------

// submitProblemRequest mirrors the OpenAPI SubmitProblemRequest
// schema. Context is decoded as raw JSON so we don't pin the
// structure — the queue's Submission.Context is map[string]any.
type submitProblemRequest struct {
	ProblemClass  string         `json:"problem_class"`
	Environment   string         `json:"environment"`
	Language      string         `json:"language"`
	Version       string         `json:"version"`
	Description   string         `json:"description"`
	ErrorMessage  string         `json:"error_message"`
	StackTrace    string         `json:"stack_trace"`
	Context       map[string]any `json:"context"`
	Cadence       string         `json:"cadence"`
	RequiredTools []string       `json:"required_tools,omitempty"`
}

// submitProblemResponse mirrors SubmitProblemResponse in the spec.
type submitProblemResponse struct {
	SubmissionID      string   `json:"submission_id"`
	ProblemClass      string   `json:"problem_class"`
	Status            string   `json:"status"` // queued | deduplicated | rejected
	Position          int      `json:"position"`
	EstimatedTime     string   `json:"estimated_time"`
	ExistingSolutions int      `json:"existing_solutions"`
	RelatedProblems   []string `json:"related_problems"`
}

// discoverRequest mirrors DiscoverRequest.
type discoverRequest struct {
	ProblemClass   string `json:"problem_class"`
	Environment    string `json:"environment"`
	Language       string `json:"language"`
	Version        string `json:"version"`
	IncludeRelated *bool  `json:"include_related,omitempty"`
}

// relatedEntry matches the related entry inside DiscoverResponse and
// RelatedResponse. The field name is "problem_class" in the JSON.
type relatedEntry struct {
	ProblemClass string  `json:"problem_class"`
	Relationship string  `json:"relationship"`
	Weight       float64 `json:"weight,omitempty"`
	Relevance    float64 `json:"relevance,omitempty"`
}

// discoverResponse mirrors DiscoverResponse. Found=false signals 404.
type discoverResponse struct {
	Found           bool           `json:"found"`
	Answer          *answerWire    `json:"answer,omitempty"`
	Related         []relatedEntry `json:"related,omitempty"`
	VersionWarnings []string       `json:"version_warnings,omitempty"`
}

// answerWire is the JSON shape of an answer. Signatures is a
// json.RawMessage so the queue's string-encoded JSON passes through
// without re-marshalling.
type answerWire struct {
	ID           int64           `json:"id"`
	ProblemClass string          `json:"problem_class"`
	Env          string          `json:"env"`
	Lang         string          `json:"lang"`
	Version      string          `json:"version"`
	Solution     string          `json:"solution"`
	Evidence     string          `json:"evidence"`
	Signatures   json.RawMessage `json:"signatures"`
	Status       string          `json:"status"`
	CreatedAt    string          `json:"created_at"`
}

// problemClassWire is the JSON shape of a problem class. Matches the
// ProblemClass schema in the OpenAPI spec.
type problemClassWire struct {
	ID               int64  `json:"id"`
	Title            string `json:"title"`
	Description      string `json:"description"`
	DescriptionShort string `json:"description_short,omitempty"`
	AnswerCount      int    `json:"answer_count"`
	Status           string `json:"status"`
	HitCount         int    `json:"hit_count"`
	LastHit          string `json:"last_hit,omitempty"`
	CreatedAt        string `json:"created_at"`
}

type listProblemsResponse struct {
	Problems []problemClassWire `json:"problems"`
	Total    int                `json:"total"`
}

type listAnswersResponse struct {
	Answers []answerWire `json:"answers"`
	Total   int          `json:"total"`
}

type relatedResponse struct {
	Related []relatedEntry `json:"related"`
}

type queueEntryWire struct {
	SubmissionID  string `json:"submission_id"`
	ProblemClass  string `json:"problem_class"`
	Status        string `json:"status"`
	Stage         string `json:"stage"`
	Position      int    `json:"position"`
	EstimatedTime string `json:"estimated_time"`
	StartedAt     string `json:"started_at,omitempty"`
	CompletedAt   string `json:"completed_at,omitempty"`
}

type queueListResponse struct {
	Entries []queueEntryWire `json:"entries"`
	Total   int              `json:"total"`
}

// taxonomyNode is the recursive shape of the taxonomy tree.
type taxonomyNode struct {
	Title       string         `json:"title"`
	Description string         `json:"description,omitempty"`
	Children    []taxonomyNode `json:"children,omitempty"`
	Answers     []answerWire   `json:"answers,omitempty"`
}

// --- Export/Import request/response types --------------------------------

// exportRequest mirrors ExportRequest from the OpenAPI spec.
type exportRequest struct {
	TargetRepo    string  `json:"target_repo"`
	AnswerIDs     []int64 `json:"answer_ids"`
	Branch        string  `json:"branch,omitempty"`
	CommitMessage string  `json:"commit_message,omitempty"`
}

// exportResponse mirrors ExportResponse from the OpenAPI spec.
type exportResponse struct {
	CommitSHA    string `json:"commit_sha"`
	PRURL        string `json:"pr_url,omitempty"`
	FilesChanged int    `json:"files_changed"`
}

// importRequest mirrors ImportRequest from the OpenAPI spec.
type importRequest struct {
	SourceRepo       string `json:"source_repo"`
	Branch           string `json:"branch,omitempty"`
	ConflictStrategy string `json:"conflict_strategy,omitempty"`
}

// importResponse mirrors ImportResponse from the OpenAPI spec.
type importResponse struct {
	Added      int `json:"added"`
	Updated    int `json:"updated"`
	Skipped    int `json:"skipped"`
	Conflicted int `json:"conflicted"`
}

// --- Handlers ------------------------------------------------------------

// handleSubmitProblem accepts either JSON or multipart/form-data. JSON is the
// standard flow. Multipart adds file attachments: the "data" field contains
// the JSON body, and any additional file parts are stored in AttachmentsDir
// and their paths added to the submission Context.
func (s *Server) handleSubmitProblem(w http.ResponseWriter, r *http.Request) {
	var req submitProblemRequest
	var attachmentPaths []string

	ct := r.Header.Get("Content-Type")
	mediaType, _, _ := mime.ParseMediaType(ct)

	if mediaType == "multipart/form-data" {
		// 10 MB max for multipart bodies.
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "failed to parse multipart form: "+err.Error())
			return
		}

		// Extract the JSON data from the "data" field.
		dataField := r.FormValue("data")
		if dataField == "" {
			writeError(w, http.StatusBadRequest, "invalid_request", "multipart form requires a \"data\" field with JSON body")
			return
		}
		if err := json.Unmarshal([]byte(dataField), &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON in \"data\" field: "+err.Error())
			return
		}

		// Save uploaded files to AttachmentsDir.
		if s.AttachmentsDir != "" {
			if err := os.MkdirAll(s.AttachmentsDir, 0o755); err != nil {
				writeError(w, http.StatusInternalServerError, "internal_error", "failed to create attachments dir: "+err.Error())
				return
			}
			for _, fhs := range r.MultipartForm.File {
				for _, fh := range fhs {
					path, err := saveUpload(s.AttachmentsDir, fh)
					if err != nil {
						writeError(w, http.StatusInternalServerError, "internal_error", "failed to save attachment: "+err.Error())
						return
					}
					attachmentPaths = append(attachmentPaths, path)
				}
			}
		}
	} else {
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
	}

	if strings.TrimSpace(req.ProblemClass) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "problem_class is required")
		return
	}
	slug := ingest.SanitizeForID(req.ProblemClass)

	// Merge attachment paths into the context map.
	if len(attachmentPaths) > 0 {
		if req.Context == nil {
			req.Context = map[string]any{}
		}
		req.Context["attachments"] = attachmentPaths
	}

	sub := ingest.Submission{
		ProblemClass:  slug,
		Environment:   req.Environment,
		Language:      req.Language,
		Version:       req.Version,
		Description:   req.Description,
		ErrorMessage:  req.ErrorMessage,
		StackTrace:    req.StackTrace,
		Context:       req.Context,
		Cadence:       req.Cadence,
		RequiredTools: req.RequiredTools,
	}
	id, existing, err := s.Queue.Submit(r.Context(), sub)
	if err != nil {
		status := ingest.StatusForHTTP(err)
		if errors.Is(err, ingest.ErrDuplicate) {
			resp := submitProblemResponse{
				ProblemClass: slug,
				Status:       "deduplicated",
			}
			if existing != nil {
				resp.SubmissionID = existing.ID
				resp.Position = s.queuePosition(r, existing)
			}
			resp.ExistingSolutions = s.countAnswersFor(r, slug)
			resp.RelatedProblems = s.relatedFor(r, slug)
			writeJSON(w, status, resp)
			return
		}
		if errors.Is(err, ingest.ErrInvalidCadence) || errors.Is(err, ingest.ErrEmptyProblemClass) {
			writeError(w, status, "invalid_request", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	depth, _ := s.Queue.Depth(r.Context())
	resp := submitProblemResponse{
		SubmissionID:      id,
		ProblemClass:      slug,
		Status:            "queued",
		Position:          depth,
		EstimatedTime:     estimateTime(depth),
		ExistingSolutions: s.countAnswersFor(r, slug),
		RelatedProblems:   s.relatedFor(r, slug),
	}
	writeJSON(w, http.StatusOK, resp)
}

// saveUpload persists a multipart.FileHeader to disk under dir/ and
// returns the absolute path. The file is saved with its original name;
// a random suffix is appended on collision.
func saveUpload(dir string, fh *multipart.FileHeader) (string, error) {
	src, err := fh.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	// Sanitize filename — use base name only, no path traversal.
	name := filepath.Base(fh.Filename)
	if name == "." || name == "/" || name == "" {
		name = "upload"
	}
	dstPath := filepath.Join(dir, name)

	// Avoid overwriting: append a suffix if the file exists.
	for i := 1; fileExists(dstPath); i++ {
		ext := filepath.Ext(name)
		base := name[:len(name)-len(ext)]
		dstPath = filepath.Join(dir, fmt.Sprintf("%s_%d%s", base, i, ext))
	}

	dst, err := os.Create(dstPath)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", err
	}
	return dstPath, nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// handleDiscover runs the graph discovery query and returns either
// the best-matching answer + related edges, or 404 when no answer
// exists for the class.
func (s *Server) handleDiscover(w http.ResponseWriter, r *http.Request) {
	var req discoverRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if strings.TrimSpace(req.ProblemClass) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "problem_class is required")
		return
	}
	slug := ingest.SanitizeForID(req.ProblemClass)
	includeRelated := true
	if req.IncludeRelated != nil {
		includeRelated = *req.IncludeRelated
	}
	res, err := s.Store.Discovery(r.Context(), slug, req.Environment, req.Language, req.Version, includeRelated)
	if err != nil {
		if errors.Is(err, graph.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "problem class not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	out := discoverResponse{Found: res.Exact != nil}
	if res.Exact != nil {
		out.Answer = answerToWire(res.Exact, slug)
	}
	for _, re := range res.Related {
		out.Related = append(out.Related, relatedEntry{
			ProblemClass: re.TargetTitle,
			Relationship: re.Relationship,
			Weight:       re.Weight,
			Relevance:    re.Weight,
		})
	}
	out.VersionWarnings = res.VersionWarnings
	writeJSON(w, http.StatusOK, out)
}

// handleListProblems supports ?q=, ?env=, ?lang=, ?status=, ?limit=,
// ?offset=. When q is set we use the FTS5 Search; otherwise the
// ListProblemClassesWithCounts with a status-derived filter.
func (s *Server) handleListProblems(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := parseIntDefault(q.Get("limit"), 20, 1, 100)
	offset := parseIntDefault(q.Get("offset"), 0, 0, 1<<20)
	status := q.Get("status")
	env := q.Get("env")
	lang := q.Get("lang")
	search := strings.TrimSpace(q.Get("q"))

	if search != "" {
		hits, err := s.Store.Search(r.Context(), search, env, lang, status, limit, offset)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		total, err := s.Store.SearchCount(r.Context(), search, env, lang, status)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		out := listProblemsResponse{Total: total}
		for _, h := range hits {
			cnt, _ := s.Store.AnswerCount(r.Context(), h.ClassID)
			out.Problems = append(out.Problems, problemClassWire{
				ID:          h.ClassID,
				Title:       h.Title,
				AnswerCount: cnt,
				Status:      statusOrDefault(status),
			})
		}
		writeJSON(w, http.StatusOK, out)
		return
	}

	rows, err := s.Store.ListProblemClassesWithCountsFiltered(r.Context(), status, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	total, err := s.Store.CountProblemClasses(r.Context(), status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	out := listProblemsResponse{Total: total}
	for _, p := range rows {
		out.Problems = append(out.Problems, problemClassWire{
			ID:          p.ID,
			Title:       p.Title,
			Description: p.Description,
			AnswerCount: p.AnswerCount,
			Status:      p.Status,
			CreatedAt:   formatTimeRFC3339(p.CreatedAt),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// handleGetProblemClass returns a single problem class detail.
func (s *Server) handleGetProblemClass(w http.ResponseWriter, r *http.Request) {
	class := r.PathValue("class")
	slug := ingest.SanitizeForID(class)
	pc, err := s.Store.GetProblemClassByTitle(r.Context(), slug)
	if err != nil {
		if errors.Is(err, graph.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "problem class not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	cnt, _ := s.Store.AnswerCount(r.Context(), pc.ID)
	status, err := s.Store.GetProblemClassStatus(r.Context(), pc.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, problemClassWire{
		ID:          pc.ID,
		Title:       pc.Title,
		Description: pc.Description,
		AnswerCount: cnt,
		Status:      status,
		CreatedAt:   formatTimeRFC3339(pc.CreatedAt),
	})
}

// handleListAnswers returns answers for a problem class.
func (s *Server) handleListAnswers(w http.ResponseWriter, r *http.Request) {
	class := r.PathValue("class")
	slug := ingest.SanitizeForID(class)
	pc, err := s.Store.GetProblemClassByTitle(r.Context(), slug)
	if err != nil {
		if errors.Is(err, graph.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "problem class not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	q := r.URL.Query()
	limit := parseIntDefault(q.Get("limit"), 20, 1, 100)
	offset := parseIntDefault(q.Get("offset"), 0, 0, 1<<20)
	all, err := s.Store.ListAnswers(r.Context(), pc.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	out := listAnswersResponse{Total: len(all)}
	for i, a := range all {
		if i < offset {
			continue
		}
		if len(out.Answers) >= limit {
			break
		}
		out.Answers = append(out.Answers, *answerToWire(&a, slug))
	}
	writeJSON(w, http.StatusOK, out)
}

// handleGetAnswer returns one answer by ID within a problem class.
func (s *Server) handleGetAnswer(w http.ResponseWriter, r *http.Request) {
	class := r.PathValue("class")
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "id must be an integer")
		return
	}
	slug := ingest.SanitizeForID(class)
	a, err := s.Store.GetAnswerNode(r.Context(), id)
	if err != nil {
		if errors.Is(err, graph.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "answer not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	// Verify the answer belongs to the requested class — prevents
	// a 200 for a cross-class lookup.
	pc, err := s.Store.GetProblemClass(r.Context(), a.ClassID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "answer not found")
		return
	}
	if pc.Title != slug {
		writeError(w, http.StatusNotFound, "not_found", "answer not found in this class")
		return
	}
	writeJSON(w, http.StatusOK, answerToWire(a, slug))
}

// handleGetRelated returns the related problem classes (graph edges).
func (s *Server) handleGetRelated(w http.ResponseWriter, r *http.Request) {
	class := r.PathValue("class")
	slug := ingest.SanitizeForID(class)
	pc, err := s.Store.GetProblemClassByTitle(r.Context(), slug)
	if err != nil {
		if errors.Is(err, graph.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "problem class not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	edges, err := s.Store.ListEdgesFrom(r.Context(), pc.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	out := relatedResponse{}
	for _, e := range edges {
		// Resolve target title for human-readable output.
		target, _ := s.Store.GetProblemClass(r.Context(), e.TargetID)
		title := ""
		if target != nil {
			title = target.Title
		}
		out.Related = append(out.Related, relatedEntry{
			ProblemClass: title,
			Relationship: e.Relationship,
			Weight:       e.Weight,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// handleListQueue returns the queue contents filtered by status.
func (s *Server) handleListQueue(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	status := q.Get("status")
	limit := parseIntDefault(q.Get("limit"), 100, 1, 1000)
	offset := parseIntDefault(q.Get("offset"), 0, 0, 1<<20)
	entries, err := s.Queue.List(r.Context(), status, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	out := queueListResponse{Total: len(entries)}
	for i, e := range entries {
		wire := entryToWire(&e)
		wire.Position = offset + i + 1
		out.Entries = append(out.Entries, wire)
	}
	writeJSON(w, http.StatusOK, out)
}

// handleGetQueueStatus returns one queue entry by submission ID.
func (s *Server) handleGetQueueStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("submission_id")
	e, err := s.Queue.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, ingest.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "submission not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, entryToWire(e))
}

// handleTaxonomy returns the full problem-class tree. The current
// implementation flattens to a 1-deep list (problem_class → answer)
// because there's no parent_id on problem_classes yet. Future
// commits can introduce hierarchy without changing the response
// shape.
func (s *Server) handleTaxonomy(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Store.ListProblemClassesWithCounts(r.Context(), 1000, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	var tree []taxonomyNode = []taxonomyNode{}
	for _, p := range rows {
		node := taxonomyNode{
			Title:       p.Title,
			Description: p.Description,
		}
		answers, err := s.Store.ListAnswers(r.Context(), p.ID)
		if err == nil {
			for _, a := range answers {
				node.Answers = append(node.Answers, *answerToWire(&a, p.Title))
			}
		}
		tree = append(tree, node)
	}
	writeJSON(w, http.StatusOK, map[string]any{"tree": tree})
}

// handleStats returns the aggregate counters. The queue depth is
// added on top of the graph-only Stats because the queue is in
// a separate table.
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	st, err := s.Store.Stats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	depth, _ := s.Queue.Depth(r.Context())
	st.QueueDepth = depth
	// Expose read-only mode so the UI can disable the AI chat panel,
	// and solver availability so users can tell why submissions sit queued.
	writeJSON(w, http.StatusOK, struct {
		*graph.Stats
		ReadOnly        bool `json:"readonly"`
		SolverAvailable bool `json:"solver_available"`
	}{Stats: st, ReadOnly: s.ReadOnly, SolverAvailable: s.SolverAvailable})
}

// --- Wire-format helpers -------------------------------------------------

// answerToWire converts a graph.AnswerNode into the API's JSON shape.
// The signatures column is a string of JSON; we pass it through as
// RawMessage so the response is valid JSON, not a string-of-JSON.
func answerToWire(a *graph.AnswerNode, problemClass string) *answerWire {
	sig := json.RawMessage(a.Signatures)
	if len(sig) == 0 {
		sig = json.RawMessage("{}")
	}
	return &answerWire{
		ID:           a.ID,
		ProblemClass: problemClass,
		Env:          a.Env,
		Lang:         a.Lang,
		Version:      a.Version,
		Solution:     a.Solution,
		Evidence:     a.Evidence,
		Signatures:   sig,
		Status:       a.Status,
		CreatedAt:    formatTimeRFC3339(a.CreatedAt),
	}
}

// entryToWire converts a queue.Entry into the API's JSON shape.
// StartedAt/CompletedAt are sql.NullString in the store; we extract
// the inner string when valid.
func entryToWire(e *ingest.Entry) queueEntryWire {
	w := queueEntryWire{
		SubmissionID: e.ID,
		ProblemClass: e.ProblemClass,
		Status:       e.Status,
		Stage:        e.Stage,
	}
	if e.StartedAt.Valid {
		w.StartedAt = e.StartedAt.String
	}
	if e.CompletedAt.Valid {
		w.CompletedAt = e.CompletedAt.String
	}
	return w
}

// queuePosition returns 1-based position of e in the pending queue.
// Used by SubmitProblemResponse so the user knows how many problems
// are ahead. Falls back to 1 if the queue is empty.
func (s *Server) queuePosition(r *http.Request, e *ingest.Entry) int {
	entries, err := s.Queue.List(r.Context(), ingest.StatusPending, 1000, 0)
	if err != nil {
		return 1
	}
	for i, other := range entries {
		if other.ID == e.ID {
			return i + 1
		}
	}
	return 1
}

// countAnswersFor returns the number of answers for a problem class.
// Used by SubmitProblemResponse.existing_solutions. Errors degrade
// to 0 (the field is informational).
func (s *Server) countAnswersFor(r *http.Request, slug string) int {
	pc, err := s.Store.GetProblemClassByTitle(r.Context(), slug)
	if err != nil {
		return 0
	}
	n, err := s.Store.AnswerCount(r.Context(), pc.ID)
	if err != nil {
		return 0
	}
	return n
}

// relatedFor returns the titles of related problem classes for a
// given slug. Errors degrade to nil.
func (s *Server) relatedFor(r *http.Request, slug string) []string {
	pc, err := s.Store.GetProblemClassByTitle(r.Context(), slug)
	if err != nil {
		return nil
	}
	titles, err := s.Store.RelatedTitles(r.Context(), pc.ID)
	if err != nil {
		return nil
	}
	return titles
}

// --- Export/Import handlers ----------------------------------------------

// handleExport accepts an ExportRequest, constructs an export.Engine,
// runs the export, and returns an ExportResponse. When ExportLocalDir
// is empty, the handler returns 501 (export not configured).
func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	if s.ExportLocalDir == "" {
		writeError(w, http.StatusNotImplemented, "not_configured", "export directory not configured")
		return
	}
	var req exportRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if strings.TrimSpace(req.TargetRepo) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "target_repo is required")
		return
	}
	if len(req.AnswerIDs) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "answer_ids must be non-empty")
		return
	}

	items := make([]export.ExportItem, len(req.AnswerIDs))
	for i, id := range req.AnswerIDs {
		items[i] = export.ExportItem{AnswerID: id}
	}

	engine := export.NewEngine(export.Config{
		RepoURL:       req.TargetRepo,
		Branch:        req.Branch,
		LocalDir:      s.ExportLocalDir,
		SubtreePrefix: "pre-solve-answers",
		Push:          true,
		GitPath:       "git",
	}, s.Store)

	result, err := engine.Export(r.Context(), items)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "export_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, exportResponse{
		CommitSHA:    result.CommitSHA,
		FilesChanged: len(result.FilesWritten),
	})
}

// handleImport accepts an ImportRequest, constructs an import.Engine,
// runs the import, and returns an ImportResponse. When ImportLocalDir
// is empty, the handler returns 501 (import not configured).
func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	if s.ImportLocalDir == "" {
		writeError(w, http.StatusNotImplemented, "not_configured", "import directory not configured")
		return
	}
	var req importRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if strings.TrimSpace(req.SourceRepo) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "source_repo is required")
		return
	}

	engine := importgit.NewEngine(importgit.Config{
		RepoURL:       req.SourceRepo,
		Branch:        req.Branch,
		LocalDir:      s.ImportLocalDir,
		SubtreePrefix: "pre-solve-answers",
		GitPath:       "git",
	}, s.Store)

	result, err := engine.Import(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "import_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, importResponse{
		Added:      result.Added,
		Updated:    result.Updated,
		Skipped:    result.Skipped,
		Conflicted: result.Conflicted,
	})
}

// parseIntDefault returns the parsed int or def if invalid. Bounds
// are inclusive; out-of-range values are clamped to def.
func parseIntDefault(s string, def, min, max int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	if n < min {
		return def
	}
	if n > max {
		return def
	}
	return n
}

// statusOrDefault returns the status if non-empty, else "pending".
func statusOrDefault(s string) string {
	if s == "" {
		return "pending"
	}
	return s
}

// formatTimeRFC3339 formats a time.Time as RFC3339 or returns "" for
// the zero value.
func formatTimeRFC3339(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// estimateTime is a placeholder ETA: 30s per queued entry, capped
// at 10 minutes. The real value will be computed from historical
// solve times once the cron loop starts producing them.
func estimateTime(depth int) string {
	if depth <= 0 {
		return "0s"
	}
	secs := depth * 30
	if secs > 600 {
		secs = 600
	}
	return (time.Duration(secs) * time.Second).String()
}

// compile-time check: sql is used by the package (e.g. NullString
// referenced in entryToWire). Avoids the unused import when the
// file is read in isolation.
var _ = sql.NullString{}
