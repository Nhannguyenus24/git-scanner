package lifecycle_test

import (
	"github.com/mservice/gitscanner/libs/lifecycle"

	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"testing"
	"time"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func TestShutdownRunsInOrderAndCountsFailures(t *testing.T) {
	var order []string
	step := func(name string, err error) lifecycle.Component {
		return lifecycle.Component{Name: name, Stop: func(context.Context) error {
			order = append(order, name)
			return err
		}}
	}

	failed := lifecycle.Shutdown(quiet, time.Second,
		step("intake", nil),
		step("flush", errors.New("broker gone")),
		lifecycle.Component{Name: "nil-stop"},
		step("store", nil),
	)

	if want := []string{"intake", "flush", "store"}; !reflect.DeepEqual(order, want) {
		t.Errorf("order = %v, want %v", order, want)
	}
	if failed != 1 {
		t.Errorf("failed = %d, want 1", failed)
	}
}

func TestShutdownSkipsRemainingAfterDeadline(t *testing.T) {
	var storeClosed bool
	failed := lifecycle.Shutdown(quiet, 10*time.Millisecond,
		lifecycle.Component{Name: "slow", Stop: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		}},
		lifecycle.Component{Name: "store", Stop: func(context.Context) error {
			storeClosed = true
			return nil
		}},
	)

	if storeClosed {
		t.Error("components after an exceeded deadline must be skipped")
	}
	if failed != 2 {
		t.Errorf("failed = %d, want 2 (the slow step and the skipped one)", failed)
	}
}
