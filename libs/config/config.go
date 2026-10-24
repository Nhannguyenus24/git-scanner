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

	OTLPEndpoint    string // empty disables tracing export
	ShutdownTimeout time.Duration

	// FingerprintKey is the HMAC key for secret fingerprints (plan.md §6). Only
	// the workers that handle raw secrets (scan-worker, enricher) require it; it
	// is read here so a single place validates its length.
	FingerprintKey []byte
	// ScratchDir is where scan-worker clones repositories; empty uses the OS
	// temp dir. In Kubernetes it is the size-limited /scratch volume.
	ScratchDir string

	// ProbeSecrets turns on live credential validation in the scan worker: a
	// single identity/liveness call per finding, to tell a user a leaked key is
	// still active. It authenticates to third-party providers with found
	// credentials, so it is off unless explicitly enabled, and must only be
	// enabled for targets the operator owns or is authorized to test.
	ProbeSecrets bool
}

// LoadCommon reads Common from the environment.
//
// serviceName is passed in rather than read from the environment so that a
// misconfigured deployment cannot make two different workloads report the same
// identity in traces and metrics.
func LoadCommon(serviceName string) (Common, error) {
	c := Common{
		ServiceName:  serviceName,
		Env:          getDefault("ENV", "dev"),
		LogLevel:     getDefault("LOG_LEVEL", "info"),
		HTTPAddr:     getDefault("HTTP_ADDR", ":8080"),
		PostgresURL:  os.Getenv("POSTGRES_URL"),
		RabbitURL:    os.Getenv("RABBITMQ_URL"),
		RedisURL:     os.Getenv("REDIS_URL"),
		OTLPEndpoint: os.Getenv("OTLP_ENDPOINT"),
		ScratchDir:   os.Getenv("SCRATCH_DIR"),
		ProbeSecrets: GetBool("PROBE_SECRETS", false),
	}

	if raw := os.Getenv("FINGERPRINT_KEY"); raw != "" {
		c.FingerprintKey = []byte(raw)
	} else if c.Env == "dev" {
		c.FingerprintKey = []byte("dev-insecure-fingerprint-key-32bytes")
	}

	timeout, err := GetDuration("SHUTDOWN_TIMEOUT", 30*time.Second)
	if err != nil {
		return Common{}, err
	}
	c.ShutdownTimeout = timeout

	return c, c.Validate()
}

// Validate reports every problem at once. Fixing configuration one error per
// restart is needlessly slow when the whole set is knowable up front.
//
// Which backend URLs are required depends on the service, so presence of those
// is checked by internal/app against the service's declared needs.
func (c Common) Validate() error {
	var errs []error

	if c.ServiceName == "" {
		errs = append(errs, errors.New("service name is empty"))
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

// GetBool reads a boolean environment variable. Only an explicit true value
// (1, t, true, yes, on; case-insensitive) is true; unset or anything else is
// the fallback, so a security-sensitive flag cannot be switched on by a typo.
func GetBool(key string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "t", "true", "yes", "on":
		return true
	case "0", "f", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

// GetOr returns the environment variable or fallback when it is unset or empty.
func GetOr(key, fallback string) string { return getDefault(key, fallback) }
