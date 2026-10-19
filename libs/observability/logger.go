package observability

import (
	"log/slog"
	"os"
)

// NewLogger returns a JSON logger at the given level, tagged with the service
// name so logs from six workloads remain separable in one stream.
func NewLogger(serviceName, level string) *slog.Logger {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parseLevel(level)})
	return slog.New(h).With("service", serviceName)
}

// parseLevel maps the configured level name. Unknown names fall back to info;
// libs/config has already rejected them, so this only guards direct callers.
func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
