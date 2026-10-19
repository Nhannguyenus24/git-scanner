package scan

import (
	"context"
	"fmt"

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

	err = repo.Blobs(ctx, func(b Blob) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		res.BlobsScanned++
		app.observe(b.Path)

		_, reachable := headOIDs[b.OID]
		for _, m := range e.det.Detect(b.Path, b.Data) {
			f := e.finding(jobID, repoID, b, m, reachable)
			if err := sink.Emit(ctx, f); err != nil {
				return fmt.Errorf("scan: emit finding: %w", err)
			}
			res.FindingsCount++
		}
		return nil
	})
	if err != nil {
		return res, err
	}

	res.AppType = app.result()
	return res, nil
}

// finding builds the storable finding from a match. The raw secret is consumed
// here into a fingerprint and a masked preview and is not retained.
func (e *Engine) finding(jobID, repoID string, b Blob, m Match, reachable bool) entity.Finding {
	meta := map[string]string{}
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
