// Package progress carries live scan progress — which step each repository is
// in right now — from the workers to the dashboard.
//
// It is telemetry, not a record. The job row's counters stay the system of
// record and the dashboard keeps reading them; an event lost here costs a line
// in a live view, never a wrong number. That is why the transport is Postgres
// LISTEN/NOTIFY rather than a RabbitMQ queue: it is at-most-once, needs no
// queue, retry or dead-letter topology that would then have to be kept from
// retrying a stale progress tick, and every api replica that listens receives
// every event, which is the fan-out a live view needs.
//
// Workers publish through Publisher; the api runs a Hub that listens, keeps a
// short replay per job and hands events to each open stream (GET
// /v1/scans/{id}/events).
package progress

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"
	"unicode/utf8"
)

// Channel is the Postgres NOTIFY channel the events travel on.
const Channel = "scan_progress"

// Stages, in the order one repository passes through them. discover is the
// job-level enumeration; the rest are per repository.
const (
	StageDiscover  = "discover"
	StageQueue     = "queue"     // a scan task was picked up
	StageClone     = "clone"     // git clone --bare
	StageWalk      = "walk"      // reading every unique blob and running detection
	StageAttribute = "attribute" // tracing findings to the commit that introduced them
	StageProbe     = "probe"     // live probes, only when enabled
	StageStore     = "store"     // writing findings and queueing enrichment
	StageRepo      = "repo"      // the repository as a whole
	StageJob       = "job"       // the job as a whole
)

// Statuses an event reports for its stage.
const (
	StatusStart     = "start"
	StatusProgress  = "progress"
	StatusDone      = "done"
	StatusFailed    = "failed"   // this attempt failed; a retry may follow
	StatusSkipped   = "skipped"  // given up on, or the job has ended
	StatusDeferred  = "deferred" // waiting on a rate limit, not counted as a failure
	StatusPaused    = "paused"
	StatusCancelled = "cancelled" // the operator ended the job
)

// Event is one step of one scan. Fields that do not apply are left zero and
// omitted from the JSON.
//
// It never carries a credential: the engine reports counts, a file path and
// durations, and the value it found has already become a fingerprint before
// anything here sees it.
type Event struct {
	Seq    int64     `json:"seq,omitempty"` // assigned by the Hub, per process
	JobID  string    `json:"job_id"`
	RepoID string    `json:"repo_id,omitempty"`
	Repo   string    `json:"repo,omitempty"` // owner/name
	Stage  string    `json:"stage"`
	Status string    `json:"status"`
	At     time.Time `json:"at"`

	ElapsedMs    int64  `json:"elapsed_ms,omitempty"`
	Attempt      int    `json:"attempt,omitempty"`
	Blobs        int    `json:"blobs,omitempty"`
	Matches      int    `json:"matches,omitempty"`  // detector hits so far, before storage dedupe
	Findings     int    `json:"findings,omitempty"` // new findings stored
	Reclassified int    `json:"reclassified,omitempty"`
	Live         int    `json:"live,omitempty"` // probes that found a live credential
	Page         int    `json:"page,omitempty"`
	Repos        int    `json:"repos,omitempty"` // repositories enqueued by discovery so far
	Path         string `json:"path,omitempty"`  // the file being read, during the walk
	AppType      string `json:"app_type,omitempty"`
	Error        string `json:"error,omitempty"`
}

// maxText bounds the free-text fields. A NOTIFY payload is limited to 8000
// bytes; a long git error or a deep path must not push an event past it and
// lose the whole event.
const maxText = 300

func clip(s string) string {
	if len(s) <= maxText {
		return s
	}
	cut := maxText
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "…"
}

// Notifier sends one payload on a channel. Satisfied by *store.DB.
type Notifier interface {
	Notify(ctx context.Context, channel, payload string) error
}

// Publisher sends events. It is best-effort by design: a failure is logged at
// debug and swallowed, because a live view going quiet is never a reason to
// fail a scan. A nil *Publisher is valid and publishes nothing, so workers can
// hold one unconditionally.
type Publisher struct {
	n   Notifier
	log *slog.Logger
	now func() time.Time
}

// NewPublisher builds a publisher over n.
func NewPublisher(n Notifier, log *slog.Logger) *Publisher {
	return &Publisher{n: n, log: log, now: time.Now}
}

// Publish stamps and sends ev.
func (p *Publisher) Publish(ctx context.Context, ev Event) {
	if p == nil || p.n == nil {
		return
	}
	if ev.At.IsZero() {
		ev.At = p.now().UTC()
	}
	ev.Path = clip(ev.Path)
	ev.Error = clip(ev.Error)
	ev.Seq = 0 // assigned by whoever receives it

	b, err := json.Marshal(ev)
	if err != nil {
		return
	}
	// Detached from the task's context: a cancelled scan still reports that it
	// stopped, and the notify is one short statement.
	nctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	if err := p.n.Notify(nctx, Channel, string(b)); err != nil && p.log != nil {
		p.log.Debug("progress event not sent", "job", ev.JobID, "stage", ev.Stage, "error", err)
	}
}
