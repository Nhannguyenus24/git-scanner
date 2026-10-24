package scan

import (
	"context"
	"fmt"
	"strconv"

	"github.com/mservice/gitscanner/internal/secrets"
	"github.com/mservice/gitscanner/libs/entity"
	"github.com/mservice/gitscanner/libs/id"
)

// Engine scans one repository: clone, walk unique blobs, detect, and hand each
// finding to a sink. It owns no I/O state, so one Engine serves every task a
// worker processes.
type Engine struct {
	cloner Cloner
	det    *Detector
	hasher *secrets.Hasher
}

// NewEngine assembles the engine from its collaborators.
func NewEngine(cloner Cloner, det *Detector, hasher *secrets.Hasher) *Engine {
	return &Engine{cloner: cloner, det: det, hasher: hasher}
}

// Sink receives findings as they are detected. Implementations must be safe to
// call once per finding and should be idempotent on the fingerprint, since a
// redelivered task replays the same findings (plan.md §6).
type Sink interface {
	Emit(ctx context.Context, f entity.Finding) error
}

// Result summarises one repository scan.
type Result struct {
	BlobsScanned  int
	FindingsCount int
	AppType       string

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
	repo, err := e.cloner.Clone(ctx, cloneURL)
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
	var pending []entity.Finding
	err = repo.Blobs(ctx, func(b Blob) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		res.BlobsScanned++
		app.observe(b.Path)

		_, reachable := headOIDs[b.OID]
		for _, m := range e.det.Detect(b.Path, b.Data) {
			pending = append(pending, e.finding(jobID, repoID, b, m, reachable))
		}
		return nil
	})
	if err != nil {
		return res, err
	}

	res.AttributionErr = attribute(ctx, repo, pending)
	if ctx.Err() != nil {
		return res, ctx.Err()
	}
	for _, f := range pending {
		if err := sink.Emit(ctx, f); err != nil {
			return res, fmt.Errorf("scan: emit finding: %w", err)
		}
		res.FindingsCount++
	}

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
