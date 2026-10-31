package api_test

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mservice/gitscanner/internal/progress"
	"github.com/mservice/gitscanner/libs/entity"
)

type fakeHub struct {
	replay []progress.Event
	events chan progress.Event
	done   chan struct{}
	after  int64
}

func newFakeHub(replay ...progress.Event) *fakeHub {
	return &fakeHub{replay: replay, events: make(chan progress.Event, 8), done: make(chan struct{})}
}

func (h *fakeHub) Subscribe(_ string, after int64) ([]progress.Event, <-chan progress.Event, func()) {
	h.after = after
	return h.replay, h.events, func() {}
}
func (h *fakeHub) Done() <-chan struct{} { return h.done }

// readStream collects the SSE frames until the server closes the stream or the
// deadline passes.
func readStream(t *testing.T, url string, header http.Header) (frames []string, status int, contentType string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	for k, v := range header {
		req.Header[k] = v
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	sc := bufio.NewScanner(res.Body)
	var cur []string
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			if len(cur) > 0 {
				frames = append(frames, strings.Join(cur, "\n"))
			}
			cur = nil
			continue
		}
		cur = append(cur, line)
	}
	return frames, res.StatusCode, res.Header.Get("Content-Type")
}

func seedJob(st *fakeStore, state entity.ScanState) entity.ScanJob {
	tgt, _ := st.CreateTarget(context.Background(), entity.Target{Kind: entity.TargetUser, Value: "octocat"})
	job, _ := st.CreateScanJob(context.Background(), tgt.ID)
	job.State = state
	st.jobs[job.ID] = job
	return job
}

// A finished job's stream replays what the hub still holds and ends at once.
func TestScanEventsReplaysThenEndsForAFinishedJob(t *testing.T) {
	srv, st, _ := newServer(t)
	job := seedJob(st, entity.ScanCompleted)
	hub := newFakeHub(progress.Event{Seq: 7, JobID: job.ID.String(), Stage: "repo", Status: "done", Findings: 2})
	srv.WithProgress(hub, nil)
	ts := httptest.NewServer(srv.Routes())
	defer ts.Close()

	frames, status, ctype := readStream(t, ts.URL+"/v1/scans/"+job.ID.String()+"/events", http.Header{"Last-Event-Id": {"5"}})
	if status != http.StatusOK || ctype != "text/event-stream" {
		t.Fatalf("status %d, content type %q", status, ctype)
	}
	if hub.after != 5 {
		t.Errorf("resumed after %d, want the Last-Event-ID 5", hub.after)
	}
	if len(frames) != 3 || frames[0] != "retry: 3000" ||
		!strings.HasPrefix(frames[1], "id: 7\ndata: {") || !strings.Contains(frames[1], `"findings":2`) ||
		frames[2] != "event: end\ndata: {\"state\":\"completed\"}" {
		t.Errorf("frames = %q", frames)
	}
}

// A running job's stream carries live events and ends when the job does.
func TestScanEventsStreamsUntilTheJobEnds(t *testing.T) {
	srv, st, _ := newServer(t)
	job := seedJob(st, entity.ScanRunning)
	hub := newFakeHub()
	srv.WithProgress(hub, nil)
	ts := httptest.NewServer(srv.Routes())
	defer ts.Close()

	id := job.ID.String()
	hub.events <- progress.Event{Seq: 1, JobID: id, Stage: "clone", Status: "start", Repo: "octocat/demo"}
	hub.events <- progress.Event{Seq: 2, JobID: id, Stage: "job", Status: "done"}

	frames, _, _ := readStream(t, ts.URL+"/v1/scans/"+id+"/events", nil)
	if len(frames) != 4 || !strings.Contains(frames[1], `"repo":"octocat/demo"`) ||
		!strings.Contains(frames[2], `"stage":"job"`) || frames[3] != "event: end\ndata: {\"state\":\"done\"}" {
		t.Errorf("frames = %q", frames)
	}
}

// On shutdown the hub's Done closes every open stream, so they do not hold the
// HTTP server's drain until its deadline.
func TestScanEventsEndOnShutdown(t *testing.T) {
	srv, st, _ := newServer(t)
	job := seedJob(st, entity.ScanRunning)
	hub := newFakeHub()
	srv.WithProgress(hub, nil)
	ts := httptest.NewServer(srv.Routes())
	defer ts.Close()

	close(hub.done)
	start := time.Now()
	readStream(t, ts.URL+"/v1/scans/"+job.ID.String()+"/events", nil)
	if time.Since(start) > 2*time.Second {
		t.Error("stream outlived the hub")
	}
}

func TestScanEventsNotFound(t *testing.T) {
	srv, st, _ := newServer(t)
	job := seedJob(st, entity.ScanRunning)
	if rec := do(srv, http.MethodGet, "/v1/scans/"+job.ID.String()+"/events", ""); rec.Code != http.StatusNotFound {
		t.Errorf("without a hub: status %d, want 404", rec.Code)
	}
	srv.WithProgress(newFakeHub(), nil)
	if rec := do(srv, http.MethodGet, "/v1/scans/00000000-0000-0000-0000-000000000000/events", ""); rec.Code != http.StatusNotFound {
		t.Errorf("unknown job: status %d, want 404", rec.Code)
	}
}
