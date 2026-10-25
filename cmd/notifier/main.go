// Command notifier sends responsible-disclosure mail under a shared rate cap (plan.md §0.3).
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/mservice/gitscanner/internal/app"
	"github.com/mservice/gitscanner/internal/notify"
	"github.com/mservice/gitscanner/libs/config"
	"github.com/mservice/gitscanner/libs/rabbit"
)

// Prefetch is modest and is not the thing that bounds the send rate: one
// delivery is one mail, and the Redis token bucket — shared by every replica —
// is what actually caps how fast mail leaves (plan.md §13.5). Prefetch only
// decides how many requests this pod holds while waiting for tokens.
const (
	notifyPrefetch = 4
	notifyMaxRetry = 5

	defaultRateCap    = 30
	defaultRateWindow = time.Hour
)

func main() {
	app.Main(app.Spec{
		Name:  "notifier",
		Needs: app.Needs{Postgres: true, Rabbit: true, Redis: true},
		Build: build,
	})
}

func build(_ context.Context, d *app.Deps) (app.Parts, error) {
	cap, err := config.GetInt("NOTIFY_RATE_CAP", defaultRateCap)
	if err != nil {
		return app.Parts{}, fmt.Errorf("notifier: %w", err)
	}
	window, err := config.GetDuration("NOTIFY_RATE_WINDOW", defaultRateWindow)
	if err != nil {
		return app.Parts{}, fmt.Errorf("notifier: %w", err)
	}

	// One key for every replica, so scaling out raises throughput but never the
	// rate at which strangers receive mail.
	bucket := d.Redis.NewBucket("notify:mail", float64(cap), window)

	sender := newSender(d)
	info := notify.DisclosureInfo{
		Project:   config.GetOr("DISCLOSURE_PROJECT", ""),
		Signature: config.GetOr("DISCLOSURE_SIGNATURE", ""),
		InfoURL:   config.GetOr("DISCLOSURE_INFO_URL", ""),
	}
	worker := notify.New(d.DB, sender, bucket, info, d.Log)

	consumer, err := d.Rabbit.NewConsumer(rabbit.QueueNotify, notifyPrefetch, notifyMaxRetry)
	if err != nil {
		return app.Parts{}, fmt.Errorf("notifier: consumer: %w", err)
	}
	d.Log.Info("notifier ready", "rate_cap", cap, "rate_window", window)

	return app.Parts{Worker: app.NewConsumerWorker(consumer, worker.Handle)}, nil
}

// newSender returns the SMTP sender when a relay is configured and the
// log-only sender otherwise, so a dev or CI run exercises the whole path
// without mailing anyone.
func newSender(d *app.Deps) notify.Sender {
	addr := config.GetOr("SMTP_ADDR", "")
	if addr == "" {
		d.Log.Warn("SMTP_ADDR is not set: disclosure mail will be logged, not sent")
		return notify.NewLogSender(d.Log)
	}
	return notify.NewSMTPSender(
		addr,
		config.GetOr("SMTP_FROM", "security@localhost"),
		config.GetOr("SMTP_USER", ""),
		config.GetOr("SMTP_PASSWORD", ""),
	)
}
