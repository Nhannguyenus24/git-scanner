package api

import (
	"context"
	"net/http"
	"sort"

	"github.com/mservice/gitscanner/internal/scan"
	"github.com/mservice/gitscanner/internal/store"
)

// InsightsSource is the analyst read model behind GET /v1/insights.
type InsightsSource interface {
	GetInsights(ctx context.Context) (store.Insights, error)
}

// WithInsights enables GET /v1/insights.
func (s *Server) WithInsights(i InsightsSource) *Server {
	s.insights = i
	return s
}

// exposureOrder is the order the exposure buckets read in, shortest first.
var exposureOrder = []string{"1d", "7d", "30d", "90d", "1y", "over_1y", "unknown"}

type activityResponse struct {
	Key      string `json:"key"`
	Repos    int    `json:"repos"`
	Findings int    `json:"findings"`
	AtHead   int    `json:"at_head"`
}

type ruleTriageResponse struct {
	Rule          string `json:"rule"`
	Total         int    `json:"total"`
	Open          int    `json:"open"`
	Rotated       int    `json:"rotated"`
	Ignored       int    `json:"ignored"`
	FalsePositive int    `json:"false_positive"`
}

type reusedSecretResponse struct {
	Preview  string `json:"preview"`
	Provider string `json:"provider"`
	Repos    int    `json:"repos"`
	Forks    int    `json:"forks"`
}

type keyTypeResponse struct {
	Provider string `json:"provider"`
	KeyType  string `json:"key_type"`
	Count    int    `json:"count"`
}

type exposureResponse struct {
	Key     string `json:"key"`
	AtHead  int    `json:"at_head"`
	Removed int    `json:"removed"`
}

func (s *Server) handleInsights(w http.ResponseWriter, r *http.Request) {
	in, err := s.insights.GetInsights(r.Context())
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	activity := make([]activityResponse, 0, len(in.RepoActivity))
	for _, b := range in.RepoActivity {
		activity = append(activity, activityResponse{Key: b.Key, Repos: b.Repos, Findings: b.Findings, AtHead: b.AtHead})
	}
	precision := make([]ruleTriageResponse, 0, len(in.RulePrecision))
	for _, t := range in.RulePrecision {
		precision = append(precision, ruleTriageResponse{
			Rule: t.Rule, Total: t.Total, Open: t.Open, Rotated: t.Rotated, Ignored: t.Ignored, FalsePositive: t.FalsePos,
		})
	}
	reused := make([]reusedSecretResponse, 0, len(in.Reuse.Top))
	for _, x := range in.Reuse.Top {
		reused = append(reused, reusedSecretResponse{Preview: x.Preview, Provider: x.Provider, Repos: x.Repos, Forks: x.Forks})
	}
	keyTypes := make([]keyTypeResponse, 0, len(in.KeyTypes))
	for _, k := range in.KeyTypes {
		keyTypes = append(keyTypes, keyTypeResponse{Provider: k.Provider, KeyType: k.KeyType, Count: k.Count})
	}

	counters := make(map[string]int, len(in.DetectionStats))
	for _, c := range in.DetectionStats {
		counters[c.Key] = c.Count
	}
	d := in.Disclosure

	writeJSON(w, http.StatusOK, map[string]any{
		"attributes":     map[string]int{"known": in.AttributesKnown, "repositories": in.Repositories},
		"by_language":    countList(in.ByLanguage),
		"by_owner_type":  countList(in.ByOwnerType),
		"repo_activity":  activity,
		"by_file_kind":   fileKinds(in.FilePaths),
		"rule_precision": precision,
		"reuse": map[string]any{
			"secrets": in.Reuse.Secrets, "repos": in.Reuse.Repos, "top": reused,
		},
		"key_types":              keyTypes,
		"modes":                  countList(in.Modes),
		"exposure":               exposure(in.Exposure),
		"median_days_to_removal": in.MedianDaysToRemoval,
		"removed_but_open":       in.RemovedButOpen,
		"commit_context":         countList(in.CommitContext),
		"disclosure": map[string]any{
			"repos": d.Repos, "findings": d.Findings, "live_at_disclosure": d.LiveAtDisclosure,
			"went_dead": d.WentDead, "still_live": d.StillLive, "rotated": d.Rotated,
			"median_days_to_dead": d.MedianDaysToDead,
		},
		"detection": map[string]any{
			"repositories": in.DetectionScannedRepo, "counters": counters,
		},
	})
}

// fileKinds folds per-path counts into per-kind counts, largest first.
func fileKinds(paths []store.Count) []countResponse {
	byKind := map[string]int{}
	for _, p := range paths {
		byKind[scan.FileKind(p.Key)] += p.Count
	}
	out := make([]countResponse, 0, len(byKind))
	for k, n := range byKind {
		out = append(out, countResponse{Key: k, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// exposure returns every bucket in reading order, zero-filled, so the chart's
// axis does not change shape as data arrives.
func exposure(bs []store.ExposureBucket) []exposureResponse {
	have := make(map[string]store.ExposureBucket, len(bs))
	for _, b := range bs {
		have[b.Key] = b
	}
	out := make([]exposureResponse, 0, len(exposureOrder))
	for _, k := range exposureOrder {
		b := have[k]
		out = append(out, exposureResponse{Key: k, AtHead: b.AtHead, Removed: b.Removed})
	}
	return out
}

func countList(cs []store.Count) []countResponse {
	out := make([]countResponse, 0, len(cs))
	for _, c := range cs {
		out = append(out, countResponse{Key: c.Key, Count: c.Count})
	}
	return out
}
