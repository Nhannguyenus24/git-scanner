// Package enrich is the enricher service: it resolves a detected credential
// against the provider registry and, when nothing in the catalog claims the
// shape, records it as a candidate so the registry learns what is still
// unmapped (plan.md §5, §5.4).
//
// Enrichment never sees the secret. It works from the finding row and the
// context the scan captured alongside it, which is why the task carries the
// nearby environment variable and host rather than a blob reference.
package enrich

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"google.golang.org/protobuf/proto"

	"github.com/mservice/gitscanner/internal/store"
	"github.com/mservice/gitscanner/libs/entity"
	gpb "github.com/mservice/gitscanner/libs/proto/gen/gitscanner/v1"
	"github.com/mservice/gitscanner/libs/rabbit"
)

// Store is the database surface enrichment needs.
type Store interface {
	GetFinding(ctx context.Context, findingID entity.ID) (entity.Finding, error)
	GetProviderBySlug(ctx context.Context, slug string) (entity.Provider, error)
	UpdateFindingEnrichment(ctx context.Context, findingID entity.ID, slug string, sev entity.Severity, meta map[string]string) error
	UpsertProviderCandidate(ctx context.Context, c entity.ProviderCandidate) error
}

// Worker enriches one finding per delivery.
type Worker struct {
	store Store
	log   *slog.Logger
}

// New builds an enrich worker.
func New(st Store, log *slog.Logger) *Worker {
	return &Worker{store: st, log: log}
}

// Handle processes one EnrichTask delivery (the rabbit.Handler for
// enrich.tasks).
func (w *Worker) Handle(ctx context.Context, d rabbit.Delivery) error {
	var task gpb.EnrichTask
	if err := proto.Unmarshal(d.Body, &task); err != nil {
		return fmt.Errorf("%w: unmarshal enrich task: %v", rabbit.ErrDrop, err)
	}

	findingID := entity.ID(task.FindingId)
	finding, err := w.store.GetFinding(ctx, findingID)
	if errors.Is(err, store.ErrNotFound) {
		// The finding is gone — its job was deleted, most likely. No number of
		// retries brings it back, so this task is finished, not failed.
		return fmt.Errorf("%w: finding %s not found", rabbit.ErrDrop, task.FindingId)
	}
	if err != nil {
		return err
	}

	meta := finding.Metadata
	if meta == nil {
		meta = map[string]string{}
	}

	var category string
	switch {
	case finding.ProviderSlug != "":
		category, err = w.resolveProvider(ctx, finding.ProviderSlug, meta)
		if err != nil {
			return err
		}
	default:
		if err := w.recordCandidate(ctx, &task, finding); err != nil {
			return err
		}
	}

	severity := Rescore(finding.Severity, finding.ReachableFromHEAD, category)
	if err := w.store.UpdateFindingEnrichment(ctx, findingID, finding.ProviderSlug, severity, meta); err != nil {
		return err
	}

	w.log.Info("finding enriched",
		"job", task.JobId, "finding", task.FindingId, "rule", finding.RuleID,
		"provider", finding.ProviderSlug, "category", category,
		"severity", severity.String(), "was", finding.Severity.String())
	return nil
}

// resolveProvider looks the slug up in the catalog and writes what it learned
// into meta. A slug the catalog has not onboarded yet is not an error: the rule
// fired on a real pattern, the registry has simply not caught up.
func (w *Worker) resolveProvider(ctx context.Context, slug string, meta map[string]string) (string, error) {
	p, err := w.store.GetProviderBySlug(ctx, slug)
	if errors.Is(err, store.ErrNotFound) {
		w.log.Warn("provider not in catalog", "provider", slug)
		return "", nil
	}
	if err != nil {
		return "", err
	}

	meta["category"] = p.Category
	meta["provider_name"] = p.Name
	return p.Category, nil
}

// recordCandidate folds an unmapped credential into the backlog.
func (w *Worker) recordCandidate(ctx context.Context, task *gpb.EnrichTask, f entity.Finding) error {
	length := int(task.SecretLength)
	cand := entity.ProviderCandidate{
		Signature:    Signature(task.NearbyEnvVar, task.CharsetClass, length, task.NearbyHost),
		GuessedName:  GuessName(task.NearbyEnvVar),
		SampleMasked: f.MaskedPreview,
		CharsetClass: task.CharsetClass,
		LengthMin:    length,
		LengthMax:    length,
		Occurrences:  1,
	}
	if task.NearbyEnvVar != "" {
		cand.EnvVarNames = []string{task.NearbyEnvVar}
	}
	if task.NearbyHost != "" {
		cand.NearbyHosts = []string{task.NearbyHost}
	}

	if err := w.store.UpsertProviderCandidate(ctx, cand); err != nil {
		return err
	}

	w.log.Info("provider candidate recorded",
		"signature", cand.Signature, "guess", cand.GuessedName,
		"env_var", task.NearbyEnvVar, "host", task.NearbyHost, "length", length)
	return nil
}

// Rescore adjusts a finding's severity using what enrichment established.
//
// Two bumps, both about consequence rather than confidence: a credential still
// reachable from HEAD is live rather than merely historical, and one belonging
// to a cloud or payment provider has a blast radius — infrastructure or money —
// that the detection rule alone cannot know about.
func Rescore(base entity.Severity, reachableFromHEAD bool, category string) entity.Severity {
	sev := base
	if reachableFromHEAD {
		sev++
	}
	switch strings.ToLower(category) {
	case "cloud", "payment":
		sev++
	}
	if sev > entity.SeverityCritical {
		return entity.SeverityCritical
	}
	return sev
}

// Signature clusters credentials of a similar shape under one backlog entry.
//
// The inputs are ordered and joined with a separator that cannot appear in any
// of them, so two different splits of the same characters cannot collide. The
// length is bucketed because key lengths vary by a byte or two across accounts
// while the underlying provider is the same.
func Signature(envVar, charsetClass string, secretLength int, host string) string {
	parts := []string{
		normalizeEnvVar(envVar),
		strings.ToLower(charsetClass),
		strconv.Itoa(secretLength / 8),
		strings.ToLower(host),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])[:32]
}

// nameSuffixes are the trailing words that describe what a value is rather than
// whose it is. Longest first, so CLOUDINARY_API_SECRET loses _API_SECRET rather
// than only _SECRET.
var nameSuffixes = []string{
	"_SECRET_ACCESS_KEY",
	"_API_SECRET",
	"_SECRET_KEY",
	"_ACCESS_KEY",
	"_API_KEY",
	"_PASSWORD",
	"_SECRET",
	"_TOKEN",
	"_PASS",
	"_KEY",
}

// GuessName derives a provider name from the environment variable beside the
// secret. In practice that variable names the missing provider outright:
// CLOUDINARY_API_SECRET, AZURE_STORAGE_KEY, MAILGUN_API_KEY (plan.md §5.4).
func GuessName(envVar string) string {
	v := normalizeEnvVar(envVar)
	if v == "" {
		return ""
	}
	for _, suffix := range nameSuffixes {
		if trimmed, ok := strings.CutSuffix(v, suffix); ok && trimmed != "" {
			v = trimmed
			break
		}
	}
	return strings.ToLower(strings.Trim(v, "_"))
}

func normalizeEnvVar(v string) string {
	return strings.ToUpper(strings.TrimSpace(v))
}
