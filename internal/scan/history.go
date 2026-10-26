package scan

import (
	"context"
	"fmt"
	"maps"
	"regexp"
	"time"

	"github.com/mservice/gitscanner/libs/entity"
)

// RemovalFinder is implemented by a Repo that can name the commit which took a
// blob out of the default branch. It is a separate, optional interface so an
// in-memory Repo used in tests need not implement it; without it, findings
// simply carry no removal.
type RemovalFinder interface {
	Removals(ctx context.Context, oids map[string]struct{}) (map[string]Commit, error)
}

// SkipReporter is implemented by a Repo that can say how many blobs its walk
// skipped without handing them to the detector, by reason.
type SkipReporter interface {
	SkippedBlobs() map[string]int
}

// initialSubject matches the messages of a project's first commit. A root
// commit is initial by definition; a non-root commit saying so is usually a
// fresh import squashed onto an empty branch, which is the same story: the
// credential arrived with the code, not in a later change.
var initialSubject = regexp.MustCompile(`(?i)^\s*(initial|first|init)\b`)

// introKind classifies the commit that introduced a credential.
func introKind(c Commit) string {
	switch {
	case c.Root || initialSubject.MatchString(c.Subject):
		return "initial"
	case mergeSubject.MatchString(c.Subject):
		return "merge"
	}
	return "normal"
}

var mergeSubject = regexp.MustCompile(`^Merge (branch|pull request|remote-tracking branch|tag) `)

// cleanupSubject matches a commit message that talks about a secret. On the
// commit that removed a credential, it means the owner knew it had leaked —
// and, since the value is still in history, that deleting the file was taken
// for the fix. A removal verb alone is not enough: "remove unused import"
// removes plenty of blobs that held a key by coincidence.
var cleanupSubject = regexp.MustCompile(`(?i)(secret|credential|passw|api[ _-]?key|access[ _-]?key|private[ _-]?key|token|\.env\b|leak|sensitive|oops)`)

// removalKind classifies the commit that removed a credential from HEAD.
func removalKind(c Commit) string {
	switch {
	case cleanupSubject.MatchString(c.Subject):
		return "cleanup"
	case mergeSubject.MatchString(c.Subject):
		return "merge"
	}
	return "normal"
}

// heldFinding is a finding awaiting attribution, with the plaintext it came
// from still in scope for an optional liveness probe.
type heldFinding struct {
	f        entity.Finding
	secret   string
	probeURL string // when set, validate this instead of secret (split-var DB creds)
}

// consolidate merges the findings that share a fingerprint into one.
//
// A credential usually sits in several blobs — each version of a file that kept
// it — and the walk reports it once per blob. The store keeps one row per
// (fingerprint, repository), so before this the row described whichever copy
// was emitted first while every later copy rewrote its time-varying state: a
// key still at HEAD in its newest version could be marked gone by an older one,
// and a removal could be paired with an introduction that came after it. The
// merged finding answers for the credential rather than for one copy:
//
//   - detection fields come from the first copy a named rule claimed, else the
//     first copy: a named rule outranks the generic pass, as in the store;
//   - location and introduction come from the copy introduced earliest, so the
//     commit, path and line agree, and name the first time it was committed;
//   - it is reachable from HEAD if any copy is;
//   - if none is, its removal is the latest copy's: the commit after which no
//     copy was left on the default branch.
//
// Output order is the order each fingerprint was first seen, so it is
// deterministic.
func consolidate(hs []heldFinding) []heldFinding {
	var order []string
	groups := map[string][]int{}
	for i, h := range hs {
		fp := h.f.Fingerprint
		if _, seen := groups[fp]; !seen {
			order = append(order, fp)
		}
		groups[fp] = append(groups[fp], i)
	}

	out := make([]heldFinding, 0, len(order))
	for _, fp := range order {
		idx := groups[fp]
		if len(idx) == 1 {
			out = append(out, hs[idx[0]])
			continue
		}

		merged := hs[idx[0]]
		for _, i := range idx {
			if hs[i].f.ProviderSlug != "" {
				merged = hs[i]
				break
			}
		}
		f := &merged.f
		meta := maps.Clone(f.Metadata)
		if meta == nil {
			meta = map[string]string{}
		}
		f.Metadata = meta

		loc := *f
		for _, i := range idx {
			c := hs[i].f
			if c.CommitSHA != "" && (loc.CommitSHA == "" || c.CommittedAt.Before(loc.CommittedAt)) {
				loc = c
			}
		}
		f.BlobOID, f.FilePath, f.LineNumber = loc.BlobOID, loc.FilePath, loc.LineNumber
		f.CommitSHA, f.AuthorName, f.AuthorEmail, f.CommittedAt = loc.CommitSHA, loc.AuthorName, loc.AuthorEmail, loc.CommittedAt
		delete(meta, "intro_commit_kind")
		if k, ok := loc.Metadata["intro_commit_kind"]; ok {
			meta["intro_commit_kind"] = k
		}

		f.ReachableFromHEAD = false
		f.RemovedCommitSHA, f.RemovedAt = "", time.Time{}
		delete(meta, "removal_commit_kind")
		var last *entity.Finding
		for _, i := range idx {
			c := hs[i].f
			if c.ReachableFromHEAD {
				f.ReachableFromHEAD = true
			}
			if c.RemovedCommitSHA != "" && (last == nil || c.RemovedAt.After(last.RemovedAt)) {
				last = &c
			}
		}
		if !f.ReachableFromHEAD && last != nil {
			f.RemovedCommitSHA, f.RemovedAt = last.RemovedCommitSHA, last.RemovedAt
			if k, ok := last.Metadata["removal_commit_kind"]; ok {
				meta["removal_commit_kind"] = k
			}
		}
		out = append(out, merged)
	}
	return out
}

// trackRemovals stamps each finding that is no longer reachable from HEAD with
// the commit that removed it. Like attribution it is best-effort: a lookup that
// fails leaves the findings without a removal, and the error is returned for
// the caller to report rather than to fail the scan on.
//
// hasHead is false for a repository with an unborn or empty HEAD, where every
// finding is "not at HEAD" without anything having removed it.
func trackRemovals(ctx context.Context, repo Repo, fs []entity.Finding, hasHead bool) error {
	rf, ok := repo.(RemovalFinder)
	if !ok || !hasHead {
		return nil
	}
	gone := map[string]struct{}{}
	for _, f := range fs {
		if !f.ReachableFromHEAD {
			gone[f.BlobOID] = struct{}{}
		}
	}
	if len(gone) == 0 {
		return nil
	}
	removed, err := rf.Removals(ctx, gone)
	for i := range fs {
		c, ok := removed[fs[i].BlobOID]
		if !ok || fs[i].ReachableFromHEAD {
			continue
		}
		fs[i].RemovedCommitSHA = c.SHA
		fs[i].RemovedAt = c.When
		if fs[i].Metadata == nil {
			fs[i].Metadata = map[string]string{}
		}
		fs[i].Metadata["removal_commit_kind"] = removalKind(c)
	}
	if err != nil {
		return fmt.Errorf("scan: track removals: %w", err)
	}
	return nil
}
