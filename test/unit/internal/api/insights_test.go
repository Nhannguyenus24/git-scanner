package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/mservice/gitscanner/internal/store"
)

type fakeInsights struct{ in store.Insights }

func (f fakeInsights) GetInsights(context.Context) (store.Insights, error) { return f.in, nil }

func TestInsightsIsOffUnlessConfigured(t *testing.T) {
	srv, _, _ := newServer(t)
	if rec := do(srv, http.MethodGet, "/v1/insights", ""); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 without WithInsights", rec.Code)
	}
}

func TestInsightsFoldsPathsAndOrdersExposure(t *testing.T) {
	srv, _, _ := newServer(t)
	days := 12.5
	srv.WithInsights(fakeInsights{in: store.Insights{
		FilePaths: []store.Count{
			{Key: ".env", Count: 3}, {Key: "config/.env.prod", Count: 2},
			{Key: "src/app.py", Count: 4}, {Key: ".env.example", Count: 1},
		},
		Exposure: []store.ExposureBucket{
			{Key: "over_1y", AtHead: 5}, {Key: "1d", Removed: 2},
		},
		MedianDaysToRemoval: &days,
		Reuse: store.ReuseSummary{Secrets: 1, Repos: 3, Top: []store.ReusedSecret{
			{Preview: "AKIA…ZAM2", Provider: "aws", Repos: 3, Forks: 2},
		}},
	}})

	rec := do(srv, http.MethodGet, "/v1/insights", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	var body struct {
		ByFileKind []struct {
			Key   string `json:"key"`
			Count int    `json:"count"`
		} `json:"by_file_kind"`
		Exposure []struct {
			Key     string `json:"key"`
			AtHead  int    `json:"at_head"`
			Removed int    `json:"removed"`
		} `json:"exposure"`
		MedianDaysToRemoval *float64 `json:"median_days_to_removal"`
		Disclosure          struct {
			MedianDaysToDead *float64 `json:"median_days_to_dead"`
		} `json:"disclosure"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}

	// Two env paths fold into one kind, which leads; the template is its own.
	want := map[string]int{"env": 5, "source": 4, "example": 1}
	if len(body.ByFileKind) != len(want) || body.ByFileKind[0].Key != "env" {
		t.Errorf("by_file_kind = %+v, want env first of %v", body.ByFileKind, want)
	}
	for _, k := range body.ByFileKind {
		if want[k.Key] != k.Count {
			t.Errorf("kind %s = %d, want %d", k.Key, k.Count, want[k.Key])
		}
	}

	// Every bucket, in reading order, zero-filled.
	keys := make([]string, len(body.Exposure))
	for i, b := range body.Exposure {
		keys[i] = b.Key
	}
	if got := strings.Join(keys, ","); got != "1d,7d,30d,90d,1y,over_1y,unknown" {
		t.Errorf("exposure order = %s", got)
	}
	if body.Exposure[0].Removed != 2 || body.Exposure[5].AtHead != 5 {
		t.Errorf("exposure values misplaced: %+v", body.Exposure)
	}
	if body.MedianDaysToRemoval == nil || *body.MedianDaysToRemoval != days {
		t.Errorf("median days to removal = %v", body.MedianDaysToRemoval)
	}
	// No disclosure data is null, not zero: "no rotations measured" is not
	// "rotated instantly".
	if body.Disclosure.MedianDaysToDead != nil {
		t.Errorf("median days to dead = %v, want null", *body.Disclosure.MedianDaysToDead)
	}
	if strings.Contains(rec.Body.String(), "fingerprint") {
		t.Error("insights response mentions a fingerprint")
	}
}
