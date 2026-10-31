package progress

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"
)

// Listener delivers every payload sent on channel to fn until ctx ends or the
// connection fails. Satisfied by *store.DB.
type Listener interface {
	Listen(ctx context.Context, channel string, fn func(payload string)) error
}

const (
	// replaySize is how many recent events a job keeps for a stream that opens
	// mid-scan, so it starts with the current step of every repository in
	// flight instead of a blank panel.
	replaySize = 300
	// idleAfter drops a job's replay once nothing has arrived for this long.
	idleAfter = 30 * time.Minute
	// subBuffer is how far one stream may fall behind before it starts losing
	// events. A slow browser loses lines; it never slows the hub or another
	// stream.
	subBuffer = 128
)

// Hub fans events from the listener out to every open stream for their job.
type Hub struct {
	log *slog.Logger
	now func() time.Time

	mu   sync.Mutex
	seq  int64
	jobs map[string]*jobLog
	done chan struct{}
}

type jobLog struct {
	events []Event
	subs   map[chan Event]struct{}
	last   time.Time
}

// NewHub builds an idle hub; Run starts it listening.
func NewHub(log *slog.Logger) *Hub {
	return &Hub{log: log, now: time.Now, jobs: map[string]*jobLog{}, done: make(chan struct{})}
}

// Done is closed when Run returns. A stream selects on it so it ends when the
// service shuts down: an open event stream would otherwise hold the HTTP
// server's graceful shutdown until its deadline.
func (h *Hub) Done() <-chan struct{} { return h.done }

// Run listens until ctx ends, reconnecting with backoff when the connection
// drops. Events sent while it is reconnecting are lost; the counters on the
// job row are unaffected.
func (h *Hub) Run(ctx context.Context, l Listener) {
	defer close(h.done)
	backoff := time.Second
	for {
		err := l.Listen(ctx, Channel, h.deliver)
		if ctx.Err() != nil {
			return
		}
		h.log.Warn("progress listener stopped, reconnecting", "error", err, "in", backoff.String())
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, 30*time.Second)
	}
}

func (h *Hub) deliver(payload string) {
	var ev Event
	if err := json.Unmarshal([]byte(payload), &ev); err != nil || ev.JobID == "" {
		h.log.Debug("progress payload ignored", "error", err)
		return
	}
	h.Add(ev)
}

// Add records ev and hands it to the job's open streams.
func (h *Hub) Add(ev Event) {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := h.now()
	h.seq++
	ev.Seq = h.seq

	jl := h.jobs[ev.JobID]
	if jl == nil {
		h.pruneLocked(now) // before the new job joins, so it cannot prune itself
		jl = &jobLog{subs: map[chan Event]struct{}{}}
		h.jobs[ev.JobID] = jl
	}
	jl.last = now
	jl.events = append(jl.events, ev)
	if over := len(jl.events) - replaySize; over > 0 {
		jl.events = append(jl.events[:0], jl.events[over:]...)
	}
	for ch := range jl.subs {
		select {
		case ch <- ev:
		default: // this stream is behind; it loses the line, nobody waits
		}
	}
}

// pruneLocked forgets jobs that have gone quiet and have no open stream. It runs
// when a new job appears, which bounds the map by the number of jobs active in
// the last idleAfter.
func (h *Hub) pruneLocked(now time.Time) {
	for id, jl := range h.jobs {
		if len(jl.subs) == 0 && now.Sub(jl.last) > idleAfter {
			delete(h.jobs, id)
		}
	}
}

// Subscribe returns the job's recent events newer than after (0 for all), and a
// channel carrying each event that arrives from now on. stop must be called
// when the stream ends.
func (h *Hub) Subscribe(jobID string, after int64) (replay []Event, events <-chan Event, stop func()) {
	h.mu.Lock()
	defer h.mu.Unlock()

	jl := h.jobs[jobID]
	if jl == nil {
		jl = &jobLog{subs: map[chan Event]struct{}{}, last: h.now()}
		h.jobs[jobID] = jl
	}
	for _, ev := range jl.events {
		if ev.Seq > after {
			replay = append(replay, ev)
		}
	}
	ch := make(chan Event, subBuffer)
	jl.subs[ch] = struct{}{}
	return replay, ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		delete(jl.subs, ch)
	}
}
