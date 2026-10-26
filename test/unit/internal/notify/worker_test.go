package notify_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/mservice/gitscanner/internal/notify"
	"github.com/mservice/gitscanner/libs/entity"
	gpb "github.com/mservice/gitscanner/libs/proto/gen/gitscanner/v1"
	"github.com/mservice/gitscanner/libs/rabbit"
)

type fakeStore struct {
	on      bool
	created int
}

func (s *fakeStore) GetRepository(context.Context, entity.ID) (entity.Repository, error) {
	return entity.Repository{Owner: "a", Name: "b"}, nil
}
func (s *fakeStore) ListFindingsByIDs(context.Context, []entity.ID) ([]entity.Finding, error) {
	return []entity.Finding{{RuleID: "x", Severity: entity.SeverityHigh, FilePath: ".env"}}, nil
}
func (s *fakeStore) ListSuppressions(context.Context) ([]string, error) { return nil, nil }
func (s *fakeStore) CreateNotification(_ context.Context, n entity.Notification) (entity.Notification, error) {
	s.created++
	n.ID = "n1"
	return n, nil
}
func (s *fakeStore) SetNotificationState(context.Context, entity.ID, entity.NotificationState, string) error {
	return nil
}
func (s *fakeStore) NotificationsEnabled(context.Context) (bool, error) { return s.on, nil }

type fakeSender struct{ sent []notify.Message }

func (s *fakeSender) Send(_ context.Context, m notify.Message) error {
	s.sent = append(s.sent, m)
	return nil
}

type fakeLimiter struct{ taken int }

func (l *fakeLimiter) Take(context.Context, float64) (time.Duration, error) {
	l.taken++
	return 0, nil
}

func delivery(t *testing.T) rabbit.Delivery {
	t.Helper()
	body, err := proto.Marshal(&gpb.NotifyTask{
		RepositoryId: "01a16d46-dc0a-7979-899f-fae3af3961b2",
		Recipient:    "dev@example.com",
		FindingIds:   []string{"01a16d46-dc0a-7979-899f-fae3af3961b3"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return rabbit.Delivery{Body: body}
}

// A request queued before the switch went off is acked without a trace: no
// mail, no row, and no rate token spent.
func TestHandleDropsWhenNotificationsOff(t *testing.T) {
	st, snd, lim := &fakeStore{on: false}, &fakeSender{}, &fakeLimiter{}
	w := notify.New(st, snd, lim, notify.DisclosureInfo{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	if err := w.Handle(context.Background(), delivery(t)); err != nil {
		t.Fatalf("Handle = %v, want nil (acked)", err)
	}
	if len(snd.sent) != 0 || st.created != 0 || lim.taken != 0 {
		t.Errorf("sent=%d created=%d taken=%d, want all zero", len(snd.sent), st.created, lim.taken)
	}
}

func TestHandleSendsWhenNotificationsOn(t *testing.T) {
	st, snd, lim := &fakeStore{on: true}, &fakeSender{}, &fakeLimiter{}
	w := notify.New(st, snd, lim, notify.DisclosureInfo{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	if err := w.Handle(context.Background(), delivery(t)); err != nil {
		t.Fatal(err)
	}
	if len(snd.sent) != 1 || snd.sent[0].To != "dev@example.com" {
		t.Errorf("sent = %+v, want one mail to dev@example.com", snd.sent)
	}
}
