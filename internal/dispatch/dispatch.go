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
