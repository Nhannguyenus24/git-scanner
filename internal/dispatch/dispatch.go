// Package dispatch turns domain work into queue messages. It is the one place
// that knows the proto wire shape of a task and which queue it goes on, so
// handlers and the API never build raw messages themselves.
package dispatch

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/proto"

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
