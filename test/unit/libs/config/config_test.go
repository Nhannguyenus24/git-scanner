package config_test

import (
	"github.com/mservice/gitscanner/libs/config"

	"strings"
	"testing"
	"time"
)

func TestLoadCommonDefaults(t *testing.T) {
	for _, k := range []string{"ENV", "LOG_LEVEL", "HTTP_ADDR", "SHUTDOWN_TIMEOUT", "KAFKA_BOOTSTRAP_SERVERS"} {
		t.Setenv(k, "")
	}

	c, err := config.LoadCommon("svc")
	if err != nil {
		t.Fatalf("LoadCommon: %v", err)
	}
	if c.Env != "dev" || c.LogLevel != "info" || c.HTTPAddr != ":8080" || c.ShutdownTimeout != 30*time.Second {
		t.Errorf("unexpected defaults: %+v", c)
	}
}

func TestLoadCommonReportsEveryProblem(t *testing.T) {
	t.Setenv("ENV", "qa")
	t.Setenv("LOG_LEVEL", "verbose")

	_, err := config.LoadCommon("svc")
	if err == nil {
		t.Fatal("invalid ENV and LOG_LEVEL were accepted")
	}
	for _, want := range []string{"ENV", "LOG_LEVEL"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %s", err, want)
		}
	}
}

func TestKafkaBrokerList(t *testing.T) {
	t.Setenv("KAFKA_BOOTSTRAP_SERVERS", " a:9092, ,b:9092 ")
	c, err := config.LoadCommon("svc")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.KafkaBroker) != 2 || c.KafkaBroker[0] != "a:9092" || c.KafkaBroker[1] != "b:9092" {
		t.Errorf("KafkaBroker = %q", c.KafkaBroker)
	}
}
