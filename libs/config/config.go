// Package config loads and validates service configuration from the environment.
//
// Every value is read once at startup and validated eagerly: a service with a bad
// configuration must fail before it reports ready, not on the first request.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Common holds the configuration every service needs. Services embed it and add
// their own fields.
type Common struct {
	ServiceName string
	Env         string // dev | staging | prod
	LogLevel    string // debug | info | warn | error
	HTTPAddr    string // health and metrics listener

	PostgresURL string
	RabbitURL   string
	RedisURL    string
	KafkaBroker []string

	OTLPEndpoint    string // empty disables tracing export
	ShutdownTimeout time.Duration
}

// LoadCommon reads Common from the environment.
//
// serviceName is passed in rather than read from the environment so that a
// misconfigured deployment cannot make two different workloads report the same
// identity in traces and metrics.
func LoadCommon(serviceName string) (Common, error) {
	c := Common{
		ServiceName:     serviceName,
		Env:             getDefault("ENV", "dev"),
		LogLevel:        getDefault("LOG_LEVEL", "info"),
		HTTPAddr:        getDefault("HTTP_ADDR", ":8080"),
		PostgresURL:     os.Getenv("POSTGRES_URL"),
		RabbitURL:       os.Getenv("RABBITMQ_URL"),
		RedisURL:        os.Getenv("REDIS_URL"),
		KafkaBroker:     splitList(getDefault("KAFKA_BOOTSTRAP_SERVERS", "")),
		OTLPEndpoint:    os.Getenv("OTLP_ENDPOINT"),
		ShutdownTimeout: 30 * time.Second,
	}

	if d, ok := os.LookupEnv("SHUTDOWN_TIMEOUT"); ok {
		parsed, err := time.ParseDuration(d)
		if err != nil {
			return Common{}, fmt.Errorf("SHUTDOWN_TIMEOUT: %w", err)
		}
		c.ShutdownTimeout = parsed
	}

	return c, c.Validate()
}

// Validate reports every problem at once. Fixing configuration one error per
// restart is needlessly slow when the whole set is knowable up front.
func (c Common) Validate() error {
	var errs []error

	if c.ServiceName == "" {
		errs = append(errs, errors.New("service name is empty"))
	}
	if c.PostgresURL == "" {
		errs = append(errs, errors.New("POSTGRES_URL is required"))
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		errs = append(errs, fmt.Errorf("LOG_LEVEL %q must be debug, info, warn or error", c.LogLevel))
	}
	switch c.Env {
	case "dev", "staging", "prod":
	default:
		errs = append(errs, fmt.Errorf("ENV %q must be dev, staging or prod", c.Env))
	}

	return errors.Join(errs...)
}

// Get returns the environment variable or an error when it is unset or empty.
func Get(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return v, nil
}

func getDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// GetInt reads an integer environment variable, returning fallback when unset.
func GetInt(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return n, nil
}

// GetDuration reads a duration environment variable, returning fallback when unset.
func GetDuration(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return d, nil
}

func splitList(v string) []string {
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
