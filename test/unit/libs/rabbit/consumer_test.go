package rabbit_test

import (
	"github.com/mservice/gitscanner/libs/rabbit"

	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

// AttemptCount decides when a task stops retrying and is dead-lettered, so a
// misread header either retries forever or drops work on the first failure.
func TestDeathCount(t *testing.T) {
	cases := []struct {
		name    string
		headers amqp.Table
		want    int
	}{
		{"first delivery", nil, 0},
		{"malformed header", amqp.Table{"x-death": "nonsense"}, 0},
		{"empty list", amqp.Table{"x-death": []any{}}, 0},
		{"counted", amqp.Table{"x-death": []any{amqp.Table{"count": int64(3)}}}, 3},
	}

	for _, c := range cases {
		if got := rabbit.AttemptCount(c.headers); got != c.want {
			t.Errorf("%s: AttemptCount = %d, want %d", c.name, got, c.want)
		}
	}
}
