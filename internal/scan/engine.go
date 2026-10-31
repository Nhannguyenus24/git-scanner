package scan

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/mservice/gitscanner/internal/progress"
	"github.com/mservice/gitscanner/internal/secrets"
	"github.com/mservice/gitscanner/libs/entity"
	"github.com/mservice/gitscanner/libs/id"
)

// probeTimeout bounds a single liveness probe. It is short: the probe makes one
// identity call, and a slow provider must never hold up a scan.
const probeTimeout = 8 * time.Second

// Engine scans one repository: clone, walk unique blobs, detect, and hand each
// finding to a sink. It owns no I/O state, so one Engine serves every task a
// worker processes.
type Engine struct {
	cloner Cloner
	det    *Detector
	hasher *secrets.Hasher
	prober Prober // nil unless live probing is enabled (off by default)
}

// NewEngine assembles the engine from its collaborators.
func NewEngine(cloner Cloner, det *Detector, hasher *secrets.Hasher) *Engine {
	return &Engine{cloner: cloner, det: det, hasher: hasher}
}

// WithProber enables live credential validation. The caller passes a prober
// only when probing is configured on and the target is one the operator owns;
// with none, Scan does offline extraction only and never touches the network.
func (e *Engine) WithProber(p Prober) *Engine {
	e.prober = p
	return e
}

// Sink receives findings as they are detected. Implementations must be safe to
// call once per finding and should be idempotent on the fingerprint, since a
// redelivered task replays the same findings (plan.md §6).
type Sink interface {
	Emit(ctx context.Context, f entity.Finding) error
}

// SecretKeeper is implemented by a sink that keeps the credentials it is given,
// sealed, for the unmask-list test flow. The engine hands over the raw value
// only when KeepsSecrets reports true for this scan, and only after sealing
// is the sink's job — the engine never stores anything itself.
type SecretKeeper interface {
	KeepsSecrets() bool
	KeepSecret(ctx context.Context, f entity.Finding, secret string) error
}

// Progress is one step of a scan as the engine sees it. Stage and Status take
// the progress package's values; the counts are cumulative for the repository.
type Progress struct {
	Stage    string
	Status   string
	Blobs    int
	Matches  int
	Findings int
	Live     int
	Path     string
	Elapsed  time.Duration
	Err      error
}

// ProgressReporter is implemented by a sink that wants to show the scan as it
// runs. The engine reports each stage's start and end and, during the walk, at
// most one tick per walkTick. A reporter must not block: it is called on the
// scan's own goroutine.
type ProgressReporter interface {
	ScanProgress(ctx context.Context, p Progress)
}

// walkTick spaces the walk's progress reports. A repository has up to millions
// of blobs; one report each would cost more than reading them.
const walkTick = 750 * time.Millisecond

type noProgress struct{}

func (noProgress) ScanProgress(context.Context, Progress) {}

// Result summarises one repository scan.
type Result struct {
	BlobsScanned  int
	FindingsCount int
	AppType       string

	// ProbesLive counts findings a liveness probe confirmed are still active.
	ProbesLive int

	// AttributionErr is set when the findings could not be traced to the
	// commits that introduced them. They are still emitted, just without commit,
	// author and date: a finding missing its provenance is worth far more than
	// no finding, so this is reported rather than failing the scan.
	AttributionErr error
}

// Scan clones cloneURL, walks its history and emits a finding per detected
// credential. repoID and jobID are stamped onto every finding so results are
// attributable without re-reading the repository.
//
// The clone is always removed, on every exit path, so a failed or cancelled
// scan cannot leak a repository onto the scratch disk.
func (e *Engine) Scan(ctx context.Context, jobID, repoID, cloneURL string, sink Sink) (Result, error) {
	var report ProgressReporter = noProgress{}
	if r, ok := sink.(ProgressReporter); ok {
		report = r
	}
	stage := func(name string) func(p Progress) {
		start := time.Now()
		report.ScanProgress(ctx, Progress{Stage: name, Status: progress.StatusStart})
		return func(p Progress) {
			p.Stage, p.Elapsed = name, time.Since(start)
			if p.Status == "" {
				p.Status = progress.StatusDone
				if p.Err != nil {
					p.Status = progress.StatusFailed
				}
			}
			report.ScanProgress(ctx, p)
		}
	}

	cloned := stage(progress.StageClone)
	repo, err := e.cloner.Clone(ctx, cloneURL)
	cloned(Progress{Err: err})
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = repo.Close() }()

	headOIDs, err := repo.HeadOIDs(ctx)
	if err != nil {
		return Result{}, err
	}

	var res Result
	app := newAppTypeDetector()

	// Findings are held until the walk ends, because attributing them needs a
	// pass over history that is only worth making once, for every blob that
	// produced one. They are few — the secret itself is already reduced to a
	// fingerprint — so holding them costs little.
	type held struct {
		f        entity.Finding
		secret   string
		probeURL string // when set, validate this instead of secret (split-var DB creds)
	}
	var pending []held
	findings := func() []entity.Finding {
		out := make([]entity.Finding, len(pending))
		for i := range pending {
			out[i] = pending[i].f
		}
		return out
	}

	walked := stage(progress.StageWalk)
	lastTick := time.Now()
	err = repo.Blobs(ctx, func(b Blob) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		res.BlobsScanned++
		app.observe(b.Path)

		_, reachable := headOIDs[b.OID]
		for _, m := range e.det.Detect(b.Path, b.Data) {
			pending = append(pending, held{f: e.finding(jobID, repoID, b, m, reachable), secret: m.Secret, probeURL: m.ProbeURL})
		}
		if now := time.Now(); now.Sub(lastTick) >= walkTick {
			lastTick = now
			report.ScanProgress(ctx, Progress{
				Stage: progress.StageWalk, Status: progress.StatusProgress,
				Blobs: res.BlobsScanned, Matches: len(pending), Path: b.Path,
			})
		}
		return nil
	})
	walked(Progress{Blobs: res.BlobsScanned, Matches: len(pending), Err: err})
	if err != nil {
		return res, err
	}

	// Attribution works on the finding rows; it does not need the secret.
	attrs := findings()
	var attributed func(Progress)
	if len(attrs) > 0 {
		attributed = stage(progress.StageAttribute)
	}
	res.AttributionErr = attribute(ctx, repo, attrs)
	if attributed != nil {
		// A failed attribution does not fail the scan, so it is reported done
		// with the error alongside rather than as a failed stage.
		attributed(Progress{Status: progress.StatusDone, Matches: len(attrs), Err: res.AttributionErr})
	}
	for i := range pending {
		pending[i].f = attrs[i]
	}
	if ctx.Err() != nil {
		return res, ctx.Err()
	}

	// Live probing, if enabled, is the last thing done with the secret in scope;
	// only its non-secret result is written back into the finding.
	if e.prober != nil && len(pending) > 0 {
		probed := stage(progress.StageProbe)
		for i := range pending {
			if pending[i].f.ProviderSlug == "" {
				continue // the generic pass names no provider to probe against
			}
			probeInput := pending[i].secret
			if pending[i].probeURL != "" {
				probeInput = pending[i].probeURL // a URL assembled from split DB settings
			}
			if e.probe(ctx, &pending[i].f, probeInput) {
				res.ProbesLive++
			}
		}
		probed(Progress{Matches: len(pending), Live: res.ProbesLive})
	}

	keeper, keep := sink.(SecretKeeper)
	keep = keep && keeper.KeepsSecrets()
	// Like attribution, storing is a stage only when there is something to
	// store; most repositories have nothing, and two empty steps each would
	// double the events a bulk scan sends for no information.
	stored := func(Progress) {}
	if len(pending) > 0 {
		stored = stage(progress.StageStore)
	}
	for _, pf := range pending {
		if err := sink.Emit(ctx, pf.f); err != nil {
			stored(Progress{Matches: len(pending), Err: err})
			return res, fmt.Errorf("scan: emit finding: %w", err)
		}
		res.FindingsCount++
		if keep {
			if err := keeper.KeepSecret(ctx, pf.f, pf.secret); err != nil {
				return res, fmt.Errorf("scan: keep secret: %w", err)
			}
		}
	}

	stored(Progress{Matches: len(pending)})

	res.AppType = app.result()
	return res, nil
}

// attribute stamps each finding with the commit that introduced its blob. It
// fills in what it can even when the lookup fails part-way.
func attribute(ctx context.Context, repo Repo, fs []entity.Finding) error {
	if len(fs) == 0 {
		return nil
	}
	want := make(map[string]struct{}, len(fs))
	for _, f := range fs {
		want[f.BlobOID] = struct{}{}
	}
	intro, err := repo.Introductions(ctx, want)
	for i := range fs {
		c, ok := intro[fs[i].BlobOID]
		if !ok {
			continue
		}
		fs[i].CommitSHA = c.SHA
		fs[i].AuthorName = c.AuthorName
		fs[i].AuthorEmail = c.AuthorEmail
		fs[i].CommittedAt = c.When
		// The walk names a blob by any one path it appears under; the
		// introducing commit's own path is the one that exists at that commit,
		// so a link to the file at that commit resolves.
		if c.Path != "" {
			fs[i].FilePath = c.Path
		}
	}
	if err != nil {
		return fmt.Errorf("scan: attribute findings: %w", err)
	}
	return nil
}

// finding builds the storable finding from a match. The raw secret is consumed
// here into a fingerprint and a masked preview and is not retained.
func (e *Engine) finding(jobID, repoID string, b Blob, m Match, reachable bool) entity.Finding {
	// The shape fields are derived here because this is the last place the secret
	// is in scope. A length and a character class describe the format a provider
	// issues rather than the value, so unlike the secret they are safe to carry
	// forward — and without them the enricher's candidate clustering has only the
	// nearby variable name to go on (plan.md §5.4).
	meta := map[string]string{
		"secret_length": strconv.Itoa(len(m.Secret)),
		"charset_class": charsetClass(m.Secret),
	}
	if m.NearbyEnvVar != "" {
		meta["env_var"] = m.NearbyEnvVar
	}
	if m.NearbyHost != "" {
		meta["host"] = m.NearbyHost
	}
	// Offline extraction: facts the credential encodes in itself, no network.
	for k, v := range extractOffline(m.Provider, m.Secret) {
		meta[k] = v
	}

	sev, err := entity.ParseSeverity(m.Severity)
	if err != nil {
		sev = entity.SeverityMedium
	}

	return entity.Finding{
		ID:                id.New(),
		ScanJobID:         entity.ID(jobID),
		RepositoryID:      entity.ID(repoID),
		RuleID:            m.RuleID,
		ProviderSlug:      m.Provider,
		Fingerprint:       e.hasher.Fingerprint(m.Secret),
		MaskedPreview:     secrets.Mask(m.Secret),
		BlobOID:           b.OID,
		FilePath:          b.Path,
		LineNumber:        m.Line,
		ReachableFromHEAD: reachable,
		Severity:          sev,
		Status:            entity.FindingOpen,
		Metadata:          meta,
	}
}

// probe validates one finding's credential and records the non-secret result in
// its metadata. It reports whether the credential is live. A probe never fails
// the scan: a timeout or transport error is recorded as an error outcome and
// the finding is kept.
func (e *Engine) probe(ctx context.Context, f *entity.Finding, secret string) bool {
	pctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	res := e.prober.Probe(pctx, f.ProviderSlug, secret)
	if res.Outcome == ProbeUnverified {
		return false // no prober for this provider; leave metadata unmarked
	}
	if f.Metadata == nil {
		f.Metadata = map[string]string{}
	}
	f.Metadata["probe_status"] = string(res.Outcome)
	for k, v := range res.Identity {
		f.Metadata["probe_"+k] = v
	}
	return res.Outcome == ProbeLive
}
