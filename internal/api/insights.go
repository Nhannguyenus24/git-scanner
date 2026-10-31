package api

import (
	"context"
	"net/http"

	"github.com/mservice/gitscanner/internal/store"
)

// Insights is the read side behind the dashboard's cross-repository views —
// questions no single finding row can answer. Satisfied by *store.DB. It is its
// own interface, like ShareStore, so the catalog fakes do not grow a method for
// every new view.
type Insights interface {
	SharedKeys(ctx context.Context, minRepos, limit int) ([]store.SharedKey, int, error)
	LeakCalendar(ctx context.Context) ([]store.LeakDay, error)
}

// WithInsights enables the cross-repository views.
func (s *Server) WithInsights(i Insights) *Server {
	s.insights = i
	return s
}

func (s *Server) insightRoutes(mux *http.ServeMux) {
	if s.insights == nil {
		return
	}
	mux.HandleFunc("GET /v1/keys/shared", s.handleSharedKeys)
	mux.HandleFunc("GET /v1/analytics/calendar", s.handleLeakCalendar)
}

type sharedKeyResponse struct {
	KeyID         string                  `json:"key_id"`
	Provider      string                  `json:"provider"`
	RuleID        string                  `json:"rule_id"`
	Severity      string                  `json:"severity"`
	MaskedPreview string                  `json:"masked_preview"`
	Repos         []sharedKeyRepoResponse `json:"repositories"`
}

type sharedKeyRepoResponse struct {
	FindingID string `json:"finding_id"`
	ID        string `json:"id"`
	Owner     string `json:"owner"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	AtHead    bool   `json:"reachable_from_head"`
}

// handleSharedKeys lists credentials found in more than one repository. Only
// the masked preview and a fingerprint prefix describe a key — the same forms
// every other view uses — so the response never carries a value, sealed or not,
// whatever the masking setting says.
func (s *Server) handleSharedKeys(w http.ResponseWriter, r *http.Request) {
	keys, total, err := s.insights.SharedKeys(r.Context(), intParam(r, "min_repos"), intParam(r, "limit"))
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]sharedKeyResponse, 0, len(keys))
	for _, k := range keys {
		repos := make([]sharedKeyRepoResponse, 0, len(k.Repos))
		for _, r := range k.Repos {
			repos = append(repos, sharedKeyRepoResponse{
				FindingID: r.FindingID, ID: r.RepoID, Owner: r.Owner, Name: r.Name,
				Status: r.Status, AtHead: r.AtHead,
			})
		}
		out = append(out, sharedKeyResponse{
			KeyID: k.KeyID, Provider: k.Provider, RuleID: k.RuleID, Severity: k.Severity,
			MaskedPreview: k.MaskedPreview, Repos: repos,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"keys": out, "total": total})
}

type leakDayResponse struct {
	Day      string `json:"day"`
	Findings int    `json:"findings"`
	Critical int    `json:"critical"`
}

// handleLeakCalendar returns findings per UTC day of commit, all time. The
// dashboard picks the year to draw; the payload is one row per day that had a
// leak, which stays small however large the corpus grows.
func (s *Server) handleLeakCalendar(w http.ResponseWriter, r *http.Request) {
	days, err := s.insights.LeakCalendar(r.Context())
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]leakDayResponse, 0, len(days))
	for _, d := range days {
		out = append(out, leakDayResponse{Day: d.Day, Findings: d.Findings, Critical: d.Critical})
	}
	writeJSON(w, http.StatusOK, map[string]any{"days": out})
}
