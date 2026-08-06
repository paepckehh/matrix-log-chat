// Package logging configures the process-wide slog logger. The level is
// controlled by the DEBUG environment variable (or the Config.Debug field
// from internal/config): when DEBUG=1 (or a truthy value) is set, the
// logger runs at DEBUG level and emits source location, otherwise it
// runs at INFO with a concise format.
//
// Packages that want to log should pull the default logger via
// `slog.Default()` (or the `logging.L()` helper) rather than wiring their
// own — this keeps a single knob (DEBUG=1) for the whole binary.
package logging

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// L returns the process-wide logger. Equivalent to slog.Default().Info
// / slog.Default().Debug etc. but shorter for callers.
func L() *slog.Logger { return slog.Default() }

// Configure sets up the default slog logger. When debug is true (or the
// DEBUG env var is truthy), the logger emits at DEBUG level with source
// file/line; otherwise it emits at INFO with a compact text format.
// output goes to stderr so it does not pollute the syslog stream on
// stdout (the bridge reads stdin and writes chat lines to Matrix, but
// diagnostics must land somewhere a human can read).
func Configure(debug bool) {
	if !debug {
		// Allow DEBUG=1 to flip the flag even if config did not.
		if v := strings.TrimSpace(os.Getenv("DEBUG")); v != "" {
			if b, err := strconv.ParseBool(v); err == nil {
				debug = b
			}
		}
	}

	level := slog.LevelInfo
	opts := &slog.HandlerOptions{
		Level: level,
	}
	if debug {
		opts.Level = slog.LevelDebug
		opts.AddSource = true
	}

	// "matrix-log-chat" prefix keeps lines greppable among other
	// processes' stderr in a container log.
	logger := slog.New(slog.NewTextHandler(os.Stderr, opts))
	slog.SetDefault(logger)
}

// Debug reports whether the active logger is at DEBUG level. Callers
// that want to gate expensive work without forcing a slog level check
// can use this.
func Debug() bool {
	return slog.Default().Enabled(context.Background(), slog.LevelDebug)
}
