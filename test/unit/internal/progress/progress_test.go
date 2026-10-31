package progress_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mservice/gitscanner/internal/progress"
)

type fakeNotifier struct {
	mu       sync.Mutex
	channel  string
	payloads []string
	err      error
}

func (n *fakeNotifier) Notify(_ context.Context, channel, payload string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.channel = channel
	n.payloads = append(n.payloads, payload)
	return n.err
}

func TestPublisherSendsClippedJSON(t *testing.T) {
	n := &fakeNotifier{}
	p := progress.NewPublisher(n, slog.New(slog.DiscardHandler))
	p.Publish(context.Background(), progress.Event{
		JobID: "j1", Stage: progress.StageWalk, Status: progress.StatusProgress,
		Path: strings.Repeat("a/", 1000), Seq: 42,
	})
	if n.channel != progress.Channel || len(n.payloads) != 1 {
		t.Fatalf("channel %q, %d payloads", n.channel, len(n.payloads))
	}
	// A NOTIFY payload is capped at 8000 bytes; one long path must not lose
	// the whole event.
	if len(n.payloads[0]) > 1000 {
		t.Errorf("payload is %d bytes", len(n.payloads[0]))
	}
	var ev progress.Event
	if err := json.Unmarshal([]byte(n.payloads[0]), &ev); err != nil {
		t.Fatal(err)
	}
	if ev.At.IsZero() || ev.Seq != 0 || ev.JobID != "j1" {
		t.Errorf("event = %+v: want a timestamp, no sequence and the job id", ev)
	}
}

// A live view going quiet is never a reason to fail a scan.
func TestPublisherIsBestEffort(t *testing.T) {
	var nilPub *progress.Publisher
	nilPub.Publish(context.Background(), progress.Event{JobID: "j"})

	n := &fakeNotifier{err: errors.New("connection refused")}
	progress.NewPublisher(n, slog.New(slog.DiscardHandler)).Publish(context.Background(), progress.Event{JobID: "j"})
}

func TestHubReplaysAndStreamsPerJob(t *testing.T) {
	h := progress.NewHub(slog.New(slog.DiscardHandler))
	h.Add(progress.Event{JobID: "a", Stage: "clone", Status: "start"})
	h.Add(progress.Event{JobID: "b", Stage: "clone", Status: "start"})
	h.Add(progress.Event{JobID: "a", Stage: "clone", Status: "done"})

	replay, events, stop := h.Subscribe("a", 0)
	defer stop()
	if len(replay) != 2 || replay[0].Seq >= replay[1].Seq {
		t.Fatalf("replay = %+v, want job a's two events in order", replay)
	}

	// Resuming after the first event replays only the second.
	again, _, stop2 := h.Subscribe("a", replay[0].Seq)
	stop2()
	if len(again) != 1 || again[0].Status != "done" {
		t.Errorf("resumed replay = %+v", again)
	}

	h.Add(progress.Event{JobID: "b", Stage: "walk", Status: "start"})
	h.Add(progress.Event{JobID: "a", Stage: "walk", Status: "start"})
	select {
	case ev := <-events:
		if ev.JobID != "a" || ev.Stage != "walk" {
			t.Errorf("got %+v, want job a's walk", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("no live event")
	}
}

// A stream that stops reading loses events; it never blocks the hub.
func TestHubDoesNotWaitForASlowStream(t *testing.T) {
	h := progress.NewHub(slog.New(slog.DiscardHandler))
	_, _, stop := h.Subscribe("a", 0)
	defer stop()
	done := make(chan struct{})
	go func() {
		for i := 0; i < 10_000; i++ {
			h.Add(progress.Event{JobID: "a", Stage: "walk", Status: "progress"})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("hub blocked on a stream that is not reading")
	}
	replay, _, stop2 := h.Subscribe("a", 0)
	stop2()
	if len(replay) == 0 || len(replay) > 1000 {
		t.Errorf("replay holds %d events, want a bounded recent window", len(replay))
	}
}

type fakeListener struct{ payloads []string }

func (l fakeListener) Listen(ctx context.Context, _ string, fn func(string)) error {
	for _, p := range l.payloads {
		fn(p)
	}
	<-ctx.Done()
	return ctx.Err()
}

func TestHubRunDeliversAndClosesDone(t *testing.T) {
	h := progress.NewHub(slog.New(slog.DiscardHandler))
	ctx, cancel := context.WithCancel(context.Background())
	go h.Run(ctx, fakeListener{payloads: []string{
		`{"job_id":"a","stage":"clone","status":"start"}`,
		`not json`,
		`{"stage":"clone","status":"start"}`,
	}})
	deadline := time.Now().Add(time.Second)
	for {
		replay, _, stop := h.Subscribe("a", 0)
		stop()
		if len(replay) == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("replay = %+v, want the one valid event", replay)
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case <-h.Done():
	case <-time.After(time.Second):
		t.Fatal("Done not closed after Run returned")
	}
}
