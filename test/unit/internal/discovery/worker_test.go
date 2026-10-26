package discovery_test

import (
	"context"
	"log/slog"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/mservice/gitscanner/internal/discovery"
	"github.com/mservice/gitscanner/internal/dispatch"
	"github.com/mservice/gitscanner/internal/github"
	"github.com/mservice/gitscanner/libs/entity"
	gpb "github.com/mservice/gitscanner/libs/proto/gen/gitscanner/v1"
	"github.com/mservice/gitscanner/libs/rabbit"
)

// fakeLister returns two pages for a user, the second short (end of listing).
type fakeLister struct {
	calls  []string
	random []github.Repo // one page returned by ListRandomRepos
}

func (f *fakeLister) ListUserRepos(_ context.Context, user, cursor string) (github.Page, error) {
	f.calls = append(f.calls, cursor)
	switch cursor {
	case "":
		return github.Page{
			Repos: []github.Repo{
				{Owner: user, Name: "a", CloneURL: "https://github.com/" + user + "/a.git"},
				{Owner: user, Name: "forked", IsFork: true, CloneURL: "https://github.com/" + user + "/forked.git"},
			},
			NextCursor: "2",
		}, nil
	default:
		return github.Page{Repos: []github.Repo{
			{Owner: user, Name: "b", CloneURL: "https://github.com/" + user + "/b.git"},
		}}, nil
	}
}
func (f *fakeLister) ListOrgRepos(context.Context, string, string) (github.Page, error) {
	return github.Page{}, nil
}

func (f *fakeLister) ListRandomRepos(context.Context) (github.Page, error) {
	return github.Page{Repos: f.random}, nil
}

type fakeStore struct {
	repos    int
	total    int
	cursor   string
	state    entity.ScanState
	existing map[string]struct{} // repos ExistingRepoFullNames reports as present
}

func (s *fakeStore) UpsertRepository(_ context.Context, r entity.Repository) (entity.Repository, error) {
	s.repos++
	r.ID = entity.ID("repo-" + r.Name)
	return r, nil
}
func (s *fakeStore) AddReposTotal(_ context.Context, _ entity.ID, n int) error {
	s.total += n
	return nil
}
func (s *fakeStore) SetScanCursor(_ context.Context, _ entity.ID, c string) error {
	s.cursor = c
	return nil
}
func (s *fakeStore) SetScanState(_ context.Context, _ entity.ID, to entity.ScanState, _ string) error {
	s.state = to
	return nil
}
func (s *fakeStore) ExistingRepoFullNames(_ context.Context, names []string) (map[string]struct{}, error) {
	out := map[string]struct{}{}
	for _, n := range names {
		if _, ok := s.existing[n]; ok {
			out[n] = struct{}{}
		}
	}
	return out, nil
}

type fakeDisp struct {
	scans       int
	discoveries []string
}

func (d *fakeDisp) EnqueueScan(context.Context, entity.ScanJob, entity.Repository) error {
	d.scans++
	return nil
}
func (d *fakeDisp) EnqueueDiscovery(_ context.Context, req dispatch.DiscoveryRequest) error {
	d.discoveries = append(d.discoveries, req.Cursor)
	return nil
}

type fakePause struct{ paused bool }

func (p *fakePause) PauseRequested(context.Context, entity.ID) (bool, error) { return p.paused, nil }

func deliver(t *testing.T, task *gpb.DiscoveryTask) rabbit.Delivery {
	t.Helper()
	body, err := proto.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	return rabbit.Delivery{Body: body}
}

func TestDiscoverFirstPageEnqueuesAndContinues(t *testing.T) {
	gh, st, disp, pause := &fakeLister{}, &fakeStore{}, &fakeDisp{}, &fakePause{}
	w := discovery.New(gh, st, disp, pause, slog.New(slog.DiscardHandler))

	err := w.Handle(context.Background(), deliver(t, &gpb.DiscoveryTask{
		JobId: "job", TargetId: "tgt", Kind: "user", Value: "octocat"}))
	if err != nil {
		t.Fatal(err)
	}

	// The fork is skipped (IncludeForks false), so one scan is enqueued.
	if disp.scans != 1 {
		t.Errorf("scans = %d, want 1 (fork skipped)", disp.scans)
	}
	if st.total != 1 {
		t.Errorf("repos total += %d, want 1", st.total)
	}
	// There is a next page, so a continuation is scheduled with cursor "2".
	if len(disp.discoveries) != 1 || disp.discoveries[0] != "2" {
		t.Errorf("continuation = %v, want [2]", disp.discoveries)
	}
	if st.cursor != "2" {
		t.Errorf("checkpoint cursor = %q, want 2", st.cursor)
	}
}

func TestDiscoverLastPageStops(t *testing.T) {
	gh, st, disp, pause := &fakeLister{}, &fakeStore{}, &fakeDisp{}, &fakePause{}
	w := discovery.New(gh, st, disp, pause, slog.New(slog.DiscardHandler))

	err := w.Handle(context.Background(), deliver(t, &gpb.DiscoveryTask{
		JobId: "job", TargetId: "tgt", Kind: "user", Value: "octocat", Cursor: "2"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(disp.discoveries) != 0 {
		t.Errorf("a short page must not schedule a continuation, got %v", disp.discoveries)
	}
	if st.cursor != "" {
		t.Errorf("cursor at end = %q, want empty", st.cursor)
	}
}

func TestDiscoverPausesBeforeWork(t *testing.T) {
	gh, st, disp, pause := &fakeLister{}, &fakeStore{}, &fakeDisp{}, &fakePause{paused: true}
	w := discovery.New(gh, st, disp, pause, slog.New(slog.DiscardHandler))

	err := w.Handle(context.Background(), deliver(t, &gpb.DiscoveryTask{
		JobId: "job", TargetId: "tgt", Kind: "user", Value: "octocat", Cursor: "3"}))
	if err != nil {
		t.Fatal(err)
	}
	if st.state != entity.ScanPaused {
		t.Errorf("state = %s, want paused", st.state)
	}
	if disp.scans != 0 || len(gh.calls) != 0 {
		t.Error("a paused job must do no listing or enqueuing")
	}
	if st.cursor != "3" {
		t.Errorf("paused cursor = %q, want 3 (resume point)", st.cursor)
	}
}

func TestDiscoverAllowlist(t *testing.T) {
	gh, st, disp, pause := &fakeLister{}, &fakeStore{}, &fakeDisp{}, &fakePause{}
	w := discovery.New(gh, st, disp, pause, slog.New(slog.DiscardHandler))

	err := w.Handle(context.Background(), deliver(t, &gpb.DiscoveryTask{
		JobId: "job", TargetId: "tgt", Kind: "allowlist",
		Allowlist: []string{"https://github.com/a/b.git", "https://github.com/c/d.git"}}))
	if err != nil {
		t.Fatal(err)
	}
	if disp.scans != 2 || st.total != 2 {
		t.Errorf("allowlist: scans=%d total=%d, want 2/2", disp.scans, st.total)
	}
}

// randomRepos builds n distinct random-sample repos.
func randomRepos(n int) []github.Repo {
	repos := make([]github.Repo, n)
	for i := 0; i < n; i++ {
		name := string(rune('a' + i))
		repos[i] = github.Repo{Owner: "rnd", Name: name, CloneURL: "https://github.com/rnd/" + name + ".git"}
	}
	return repos
}

func TestDiscoverRandomCapsToCountAndStops(t *testing.T) {
	gh := &fakeLister{random: randomRepos(5)}
	st, disp, pause := &fakeStore{}, &fakeDisp{}, &fakePause{}
	w := discovery.New(gh, st, disp, pause, slog.New(slog.DiscardHandler))

	// Count is 3; the page offers 5 fresh repos, so only 3 are enqueued.
	err := w.Handle(context.Background(), deliver(t, &gpb.DiscoveryTask{
		JobId: "job", TargetId: "tgt", Kind: "random", Value: "3"}))
	if err != nil {
		t.Fatal(err)
	}
	if disp.scans != 3 || st.total != 3 {
		t.Errorf("scans=%d total=%d, want 3/3", disp.scans, st.total)
	}
	if len(disp.discoveries) != 0 {
		t.Errorf("reaching the count must stop, got continuation %v", disp.discoveries)
	}
	if st.cursor != "3" {
		t.Errorf("cursor = %q, want 3", st.cursor)
	}
}

func TestDiscoverRandomSkipsDuplicates(t *testing.T) {
	gh := &fakeLister{random: randomRepos(3)} // rnd/a, rnd/b, rnd/c
	st := &fakeStore{existing: map[string]struct{}{"rnd/a": {}, "rnd/b": {}}}
	disp, pause := &fakeDisp{}, &fakePause{}
	w := discovery.New(gh, st, disp, pause, slog.New(slog.DiscardHandler))

	// Forever (count 0): only the one not-yet-seen repo is enqueued, and the walk
	// continues for another page.
	err := w.Handle(context.Background(), deliver(t, &gpb.DiscoveryTask{
		JobId: "job", TargetId: "tgt", Kind: "random", Value: "0"}))
	if err != nil {
		t.Fatal(err)
	}
	if disp.scans != 1 || st.total != 1 {
		t.Errorf("scans=%d total=%d, want 1/1 (two duplicates skipped)", disp.scans, st.total)
	}
	if len(disp.discoveries) != 1 {
		t.Errorf("forever must schedule a continuation, got %v", disp.discoveries)
	}
}

func TestDiscoverRandomForeverContinues(t *testing.T) {
	gh := &fakeLister{random: randomRepos(2)}
	st, disp, pause := &fakeStore{}, &fakeDisp{}, &fakePause{}
	w := discovery.New(gh, st, disp, pause, slog.New(slog.DiscardHandler))

	err := w.Handle(context.Background(), deliver(t, &gpb.DiscoveryTask{
		JobId: "job", TargetId: "tgt", Kind: "random", Value: "0", Cursor: "10"}))
	if err != nil {
		t.Fatal(err)
	}
	if disp.scans != 2 {
		t.Errorf("scans = %d, want 2", disp.scans)
	}
	// Cursor accumulates: 10 already done + 2 this page.
	if st.cursor != "12" || len(disp.discoveries) != 1 || disp.discoveries[0] != "12" {
		t.Errorf("cursor=%q continuation=%v, want 12/[12]", st.cursor, disp.discoveries)
	}
}
