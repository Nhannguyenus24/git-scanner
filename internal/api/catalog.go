package api

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/mservice/gitscanner/internal/store"
	"github.com/mservice/gitscanner/libs/entity"
)

// Catalog is the read side the dashboard browses, plus finding triage. It is a
// separate interface from Store so the scan-submission handlers, and the fakes
// that test them, do not have to carry the whole browsing surface.
type Catalog interface {
	ListScanJobs(ctx context.Context, limit int) ([]store.JobSummary, error)
	SearchFindings(ctx context.Context, f store.FindingFilter) ([]store.FindingRow, int, error)
	SetFindingStatus(ctx context.Context, id entity.ID, status entity.FindingStatus) error
	ListRepositories(ctx context.Context) ([]store.RepositorySummary, error)
	GetOverview(ctx context.Context) (store.Overview, error)
	ListProviderCandidates(ctx context.Context) ([]entity.ProviderCandidate, error)
	FindingsTimeline(ctx context.Context) ([]store.TimePoint, error)
	FindingsByAppType(ctx context.Context) ([]store.Count, error)
	FindingsByRule(ctx context.Context, limit int) ([]store.Count, error)
	LiveProbeCounts(ctx context.Context) ([]store.Count, error)
	NotificationCounts(ctx context.Context) ([]store.Count, error)
}

// WithCatalog enables the browsing and triage routes.
func (s *Server) WithCatalog(c Catalog) *Server {
	s.catalog = c
	return s
}

func (s *Server) catalogRoutes(mux *http.ServeMux) {
	if s.catalog == nil {
		return
	}
	mux.HandleFunc("GET /v1/overview", s.handleOverview)
	mux.HandleFunc("GET /v1/scans", s.handleListScans)
	mux.HandleFunc("GET /v1/findings", s.handleSearchFindings)
	mux.HandleFunc("PATCH /v1/findings/{id}", s.handleUpdateFinding)
	mux.HandleFunc("GET /v1/repositories", s.handleListRepositories)
	mux.HandleFunc("GET /v1/candidates", s.handleListCandidates)
	mux.HandleFunc("GET /v1/analytics", s.handleAnalytics)
}

type scanSummaryResponse struct {
	scanJobResponse
	Target     targetResponse `json:"target"`
	CreatedAt  time.Time      `json:"created_at"`
	StartedAt  *time.Time     `json:"started_at,omitempty"`
	FinishedAt *time.Time     `json:"finished_at,omitempty"`
}

type targetResponse struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

func (s *Server) handleListScans(w http.ResponseWriter, r *http.Request) {
	jobs, err := s.catalog.ListScanJobs(r.Context(), intParam(r, "limit"))
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]scanSummaryResponse, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, scanSummaryResponse{
			scanJobResponse: toScanResponse(j.ScanJob),
			Target:          targetResponse{Kind: j.TargetKind, Value: j.TargetValue},
			CreatedAt:       j.CreatedAt, StartedAt: j.StartedAt, FinishedAt: j.FinishedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"scans": out})
}

type findingResponse struct {
	ID                string            `json:"id"`
	ScanJobID         string            `json:"scan_job_id"`
	Repository        repoRef           `json:"repository"`
	RuleID            string            `json:"rule_id"`
	Provider          string            `json:"provider"`
	Severity          string            `json:"severity"`
	Status            string            `json:"status"`
	MaskedPreview     string            `json:"masked_preview"`
	FilePath          string            `json:"file_path"`
	LineNumber        int               `json:"line_number"`
	CommitSHA         string            `json:"commit_sha"`
	AuthorName        string            `json:"author_name"`
	CommittedAt       *time.Time        `json:"committed_at,omitempty"`
	ReachableFromHEAD bool              `json:"reachable_from_head"`
	DetectedAt        time.Time         `json:"detected_at"`
	Metadata          map[string]string `json:"metadata"`
	// Secret is the unmasked credential: present only while masking is off,
	// for an unmask-listed owner, when a scan kept the value.
	Secret string `json:"secret,omitempty"`
}

type repoRef struct {
	ID    string `json:"id"`
	Owner string `json:"owner"`
	Name  string `json:"name"`
	URL   string `json:"url"`
}

func (s *Server) handleSearchFindings(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := store.FindingFilter{
		Severity:     q.Get("severity"),
		Provider:     q.Get("provider"),
		Status:       q.Get("status"),
		RepositoryID: entity.ID(q.Get("repository_id")),
		ScanJobID:    entity.ID(q.Get("scan_id")),
		Query:        q.Get("q"),
		Limit:        intParam(r, "limit"),
		Offset:       intParam(r, "offset"),
	}
	// Reject bad enum values here: they would otherwise reach the query as a
	// filter that silently matches nothing.
	if filter.Severity != "" {
		if _, err := entity.ParseSeverity(filter.Severity); err != nil {
			s.writeError(w, http.StatusBadRequest, err)
			return
		}
	}
	if filter.Status != "" && !validFindingStatus(entity.FindingStatus(filter.Status)) {
		s.writeError(w, http.StatusBadRequest, errors.New("unknown status "+strconv.Quote(filter.Status)))
		return
	}
	for _, id := range []entity.ID{filter.RepositoryID, filter.ScanJobID} {
		if id != "" && !validID(id) {
			s.writeError(w, http.StatusBadRequest, errors.New("invalid id "+strconv.Quote(string(id))))
			return
		}
	}

	if s.sealer != nil {
		st, err := s.store.GetSettings(r.Context())
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err)
			return
		}
		filter.Reveal = !st.MaskingEnabled
	}

	rows, total, err := s.catalog.SearchFindings(r.Context(), filter)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]findingResponse, 0, len(rows))
	for _, f := range rows {
		var secret string
		if f.Sealed != nil && s.sealer != nil {
			// A value that does not open (the key was rotated) falls back to
			// the masked preview rather than failing the whole page.
			if v, err := s.sealer.Open(f.Sealed, store.SealedAAD(f.RepositoryID, f.Fingerprint)); err == nil {
				secret = v
			}
		}
		out = append(out, findingResponse{
			Secret: secret,
			ID:     f.ID.String(), ScanJobID: f.ScanJobID.String(),
			Repository: repoRef{ID: f.RepositoryID.String(), Owner: f.RepoOwner, Name: f.RepoName, URL: f.RepoURL},
			RuleID:     f.RuleID, Provider: f.Provider, Severity: f.Severity, Status: f.Status,
			MaskedPreview: f.MaskedPreview, FilePath: f.FilePath, LineNumber: f.LineNumber,
			CommitSHA: f.CommitSHA, AuthorName: f.AuthorName, CommittedAt: f.CommittedAt,
			ReachableFromHEAD: f.ReachableFromHEAD, DetectedAt: f.DetectedAt,
			Metadata: nonNilMap(f.Metadata),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"findings": out, "total": total})
}

type updateFindingRequest struct {
	Status string `json:"status"`
}

func (s *Server) handleUpdateFinding(w http.ResponseWriter, r *http.Request) {
	id := entity.ID(r.PathValue("id"))
	if !validID(id) {
		s.writeError(w, http.StatusBadRequest, errors.New("invalid id"))
		return
	}
	var req updateFindingRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	status := entity.FindingStatus(req.Status)
	if !validFindingStatus(status) {
		s.writeError(w, http.StatusBadRequest, errors.New("unknown status "+strconv.Quote(req.Status)))
		return
	}

	err := s.catalog.SetFindingStatus(r.Context(), id, status)
	if errors.Is(err, store.ErrNotFound) {
		s.writeError(w, http.StatusNotFound, errors.New("finding not found"))
		return
	}
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.log.Info("finding triaged", "finding", id.String(), "status", req.Status)
	writeJSON(w, http.StatusOK, map[string]string{"id": id.String(), "status": req.Status})
}

type repositoryResponse struct {
	ID            string     `json:"id"`
	Owner         string     `json:"owner"`
	Name          string     `json:"name"`
	URL           string     `json:"url"`
	AppType       string     `json:"app_type"`
	Stars         int        `json:"stars"`
	IsFork        bool       `json:"is_fork"`
	LastScannedAt *time.Time `json:"last_scanned_at,omitempty"`
	Findings      int        `json:"findings"`
	Open          int        `json:"open"`
	Critical      int        `json:"critical"`
	High          int        `json:"high"`
	Medium        int        `json:"medium"`
	Low           int        `json:"low"`
	// Notification is the latest disclosure; absent when never contacted.
	Notification *noticeResponse `json:"notification,omitempty"`
}

type noticeResponse struct {
	State     string     `json:"state"`
	Recipient string     `json:"recipient"`
	Error     string     `json:"error,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	SentAt    *time.Time `json:"sent_at,omitempty"`
}

func (s *Server) handleListRepositories(w http.ResponseWriter, r *http.Request) {
	repos, err := s.catalog.ListRepositories(r.Context())
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]repositoryResponse, 0, len(repos))
	for _, x := range repos {
		resp := repositoryResponse{
			ID: x.ID.String(), Owner: x.Owner, Name: x.Name, URL: x.CloneURL,
			AppType: x.AppType, Stars: x.Stars, IsFork: x.IsFork, LastScannedAt: x.LastScannedAt,
			Findings: x.Findings, Open: x.Open,
			Critical: x.Critical, High: x.High, Medium: x.Medium, Low: x.Low,
		}
		if n := x.Notice; n != nil {
			resp.Notification = &noticeResponse{
				State: n.State, Recipient: n.Recipient, Error: n.Error, CreatedAt: n.CreatedAt, SentAt: n.SentAt,
			}
		}
		out = append(out, resp)
	}
	writeJSON(w, http.StatusOK, map[string]any{"repositories": out})
}

type countResponse struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	o, err := s.catalog.GetOverview(r.Context())
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	counts := func(cs []store.Count) []countResponse {
		out := make([]countResponse, 0, len(cs))
		for _, c := range cs {
			out = append(out, countResponse{Key: c.Key, Count: c.Count})
		}
		return out
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"findings":            o.Findings,
		"open_findings":       o.OpenFindings,
		"reachable_from_head": o.ReachableFromHEAD,
		"repositories":        o.Repositories,
		"exposed_repos":       o.ExposedRepos,
		"active_scans":        o.ActiveJobs,
		"candidates":          o.Candidates,
		"notified_repos":      o.NotifiedRepos,
		"awaiting_notice":     o.AwaitingNotice,
		"by_severity":         counts(o.BySeverity),
		"by_provider":         counts(o.ByProvider),
		"by_status":           counts(o.ByStatus),
		"by_scan_state":       counts(o.ByState),
	})
}

type candidateResponse struct {
	ID           string    `json:"id"`
	GuessedName  string    `json:"guessed_name"`
	EnvVarNames  []string  `json:"env_var_names"`
	NearbyHosts  []string  `json:"nearby_hosts"`
	SampleMasked string    `json:"sample_masked"`
	CharsetClass string    `json:"charset_class"`
	LengthMin    int       `json:"length_min"`
	LengthMax    int       `json:"length_max"`
	Occurrences  int       `json:"occurrences"`
	RepoCount    int       `json:"repo_count"`
	Status       string    `json:"status"`
	FirstSeenAt  time.Time `json:"first_seen_at"`
	LastSeenAt   time.Time `json:"last_seen_at"`
}

func (s *Server) handleListCandidates(w http.ResponseWriter, r *http.Request) {
	cs, err := s.catalog.ListProviderCandidates(r.Context())
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]candidateResponse, 0, len(cs))
	for _, c := range cs {
		out = append(out, candidateResponse{
			ID: c.ID.String(), GuessedName: c.GuessedName,
			EnvVarNames: nonNil(c.EnvVarNames), NearbyHosts: nonNil(c.NearbyHosts),
			SampleMasked: c.SampleMasked, CharsetClass: c.CharsetClass,
			LengthMin: c.LengthMin, LengthMax: c.LengthMax,
			Occurrences: c.Occurrences, RepoCount: c.RepoCount, Status: string(c.Status),
			FirstSeenAt: c.FirstSeenAt, LastSeenAt: c.LastSeenAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"candidates": out})
}

// uuidShape is checked before an id reaches a uuid column, where a malformed
// value is a Postgres error and would surface as a 500 instead of a 400.
var uuidShape = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func validID(id entity.ID) bool { return uuidShape.MatchString(string(id)) }

func (s *Server) handleAnalytics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	timeline, err := s.catalog.FindingsTimeline(ctx)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	byApp, err := s.catalog.FindingsByAppType(ctx)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	byRule, err := s.catalog.FindingsByRule(ctx, intParam(r, "rules"))
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	probe, err := s.catalog.LiveProbeCounts(ctx)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	notices, err := s.catalog.NotificationCounts(ctx)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	points := make([]map[string]any, 0, len(timeline))
	for _, p := range timeline {
		points = append(points, map[string]any{"period": p.Period, "findings": p.Findings})
	}
	counts := func(cs []store.Count) []countResponse {
		out := make([]countResponse, 0, len(cs))
		for _, c := range cs {
			out = append(out, countResponse{Key: c.Key, Count: c.Count})
		}
		return out
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"timeline":      points,
		"by_app_type":   counts(byApp),
		"by_rule":       counts(byRule),
		"live_probes":   counts(probe),
		"notifications": counts(notices),
	})
}

func nonNilMap(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

func validFindingStatus(s entity.FindingStatus) bool {
	switch s {
	case entity.FindingOpen, entity.FindingRotated, entity.FindingFalsePositive, entity.FindingIgnored:
		return true
	}
	return false
}

// intParam reads a non-negative integer query parameter; anything else is 0,
// which the store treats as its default.
func intParam(r *http.Request, name string) int {
	n, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// nonNil keeps an empty list as [] in JSON rather than null, so the client can
// iterate it without a guard.
func nonNil(xs []string) []string {
	if xs == nil {
		return []string{}
	}
	return xs
}
