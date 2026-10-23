// Package dispatch turns domain work into queue messages. It is the one place
// that knows the proto wire shape of a task and which queue it goes on, so
// handlers and the API never build raw messages themselves.
package dispatch

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mservice/gitscanner/libs/entity"
	gpb "github.com/mservice/gitscanner/libs/proto/gen/gitscanner/v1"
	"github.com/mservice/gitscanner/libs/rabbit"
)

// Publisher is the rabbit publishing surface dispatch needs.
type Publisher interface {
	Publish(ctx context.Context, queue string, body []byte, headers map[string]any) error
}

// Dispatcher publishes tasks onto their queues.
type Dispatcher struct {
	pub Publisher
}

// New builds a dispatcher over a rabbit publisher.
func New(pub Publisher) *Dispatcher { return &Dispatcher{pub: pub} }

// EnqueueScan publishes one scan task for a repository (plan.md §8).
func (d *Dispatcher) EnqueueScan(ctx context.Context, job entity.ScanJob, repo entity.Repository) error {
	task := &gpb.ScanTask{
		JobId:           job.ID.String(),
		RepositoryId:    repo.ID.String(),
		CloneUrl:        repo.CloneURL,
		DefaultBranch:   repo.DefaultBranch,
		RegistryVersion: job.RegistryVer,
	}
	body, err := proto.Marshal(task)
	if err != nil {
		return fmt.Errorf("dispatch: marshal scan task: %w", err)
	}
	if err := d.pub.Publish(ctx, rabbit.QueueScan, body, nil); err != nil {
		return fmt.Errorf("dispatch: enqueue scan: %w", err)
	}
	return nil
}

// EnrichRequest carries the context the enricher needs to classify a finding
// without re-reading the blob it came from (plan.md §5).
type EnrichRequest struct {
	FindingID entity.ID
	JobID     entity.ID
	RuleID    string

	FilePath     string
	NearbyEnvVar string
	NearbyHost   string
	CharsetClass string
	SecretLength int
}

// EnqueueEnrich publishes one enrichment task per newly stored finding.
func (d *Dispatcher) EnqueueEnrich(ctx context.Context, req EnrichRequest) error {
	task := &gpb.EnrichTask{
		FindingId:    req.FindingID.String(),
		JobId:        req.JobID.String(),
		RuleId:       req.RuleID,
		FilePath:     req.FilePath,
		NearbyEnvVar: req.NearbyEnvVar,
		NearbyHost:   req.NearbyHost,
		CharsetClass: req.CharsetClass,
		SecretLength: int32(req.SecretLength),
	}
	body, err := proto.Marshal(task)
	if err != nil {
		return fmt.Errorf("dispatch: marshal enrich task: %w", err)
	}
	if err := d.pub.Publish(ctx, rabbit.QueueEnrich, body, nil); err != nil {
		return fmt.Errorf("dispatch: enqueue enrich: %w", err)
	}
	return nil
}

// NotifyRequest asks for one responsible-disclosure mail about a repository.
type NotifyRequest struct {
	RepositoryID entity.ID
	Recipient    string
	FindingIDs   []entity.ID
}

// EnqueueNotify publishes one disclosure task. The notifier, not this call,
// enforces the one-mail-per-repository cap (plan.md §0.3).
func (d *Dispatcher) EnqueueNotify(ctx context.Context, req NotifyRequest) error {
	ids := make([]string, 0, len(req.FindingIDs))
	for _, id := range req.FindingIDs {
		ids = append(ids, id.String())
	}

	task := &gpb.NotifyTask{
		RepositoryId: req.RepositoryID.String(),
		Recipient:    req.Recipient,
		FindingIds:   ids,
		RequestedAt:  timestamppb.Now(),
	}
	body, err := proto.Marshal(task)
	if err != nil {
		return fmt.Errorf("dispatch: marshal notify task: %w", err)
	}
	if err := d.pub.Publish(ctx, rabbit.QueueNotify, body, nil); err != nil {
		return fmt.Errorf("dispatch: enqueue notify: %w", err)
	}
	return nil
}

// DiscoveryRequest describes a target to expand into repositories. Cursor is
// non-empty when resuming a paused discovery (plan.md §8).
type DiscoveryRequest struct {
	JobID        entity.ID
	TargetID     entity.ID
	Kind         string // user | org | allowlist
	Value        string
	Allowlist    []string
	IncludeForks bool
	Cursor       string
}

// EnqueueDiscovery publishes a discovery task. The discovery worker republishes
// one for itself with an advanced cursor to continue after a pause or to
// paginate without holding a prefetch slot for the whole walk.
func (d *Dispatcher) EnqueueDiscovery(ctx context.Context, req DiscoveryRequest) error {
	task := &gpb.DiscoveryTask{
		JobId:        req.JobID.String(),
		TargetId:     req.TargetID.String(),
		Kind:         req.Kind,
		Value:        req.Value,
		Allowlist:    req.Allowlist,
		IncludeForks: req.IncludeForks,
		Cursor:       req.Cursor,
	}
	body, err := proto.Marshal(task)
	if err != nil {
		return fmt.Errorf("dispatch: marshal discovery task: %w", err)
	}
	if err := d.pub.Publish(ctx, rabbit.QueueDiscovery, body, nil); err != nil {
		return fmt.Errorf("dispatch: enqueue discovery: %w", err)
	}
	return nil
}
