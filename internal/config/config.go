// Package config loads runtime configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime settings. Every field is sourced from the
// environment so the binary can be deployed behind a simple `env` wrapper
// or a systemd `EnvironmentFile=` without any flag plumbing.
type Config struct {
	Homeserver  string        // MATRIX_HOMESERVER, e.g. https://matrix.org
	User        string        // MATRIX_USER, e.g. @bot:matrix.org (for logging + sanity)
	Token       string        // MATRIX_TOKEN, access token from Matrix login
	Room        string        // MATRIX_ROOM, e.g. !abc123:matrix.org
	Rate        time.Duration // MATRIX_RATE_MS, min spacing between sends
	DryRun      bool          // MATRIX_DRY_RUN, log to stderr instead of sending
	MaxRetries  int           // MATRIX_MAX_RETRIES, attempts per line before giving up
	GracePeriod time.Duration // MATRIX_GRACE_MS, backoff between retries
	Debug       bool          // MATRIX_DEBUG or DEBUG, enable verbose slog output
}

// Load reads the environment and returns a validated Config. It returns a
// descriptive error when any required variable is missing or malformed so
// the caller can surface it without further interpretation.
func Load() (Config, error) {
	get := func(key string) string { return strings.TrimSpace(os.Getenv(key)) }

	cfg := Config{
		Homeserver:  get("MATRIX_HOMESERVER"),
		User:        get("MATRIX_USER"),
		Token:       get("MATRIX_TOKEN"),
		Room:        get("MATRIX_ROOM"),
		Rate:        300 * time.Millisecond,
		DryRun:      false,
		MaxRetries:  3,
		GracePeriod: 500 * time.Millisecond,
	}

	missing := make([]string, 0, 4)
	if cfg.Homeserver == "" {
		missing = append(missing, "MATRIX_HOMESERVER")
	}
	if cfg.User == "" {
		missing = append(missing, "MATRIX_USER")
	}
	if cfg.Token == "" {
		missing = append(missing, "MATRIX_TOKEN")
	}
	if cfg.Room == "" {
		missing = append(missing, "MATRIX_ROOM")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required env: %s", strings.Join(missing, ", "))
	}

	// Catch the common mistake of supplying a bare domain. gomatrix
	// would otherwise fail with a confusing error deep inside the HTTP
	// client. A scheme is required so net/http selects the right
	// transport; "matrix.org" is rejected, "https://matrix.org" is not.
	if !strings.HasPrefix(strings.ToLower(cfg.Homeserver), "http://") &&
		!strings.HasPrefix(strings.ToLower(cfg.Homeserver), "https://") {
		return Config{}, fmt.Errorf("MATRIX_HOMESERVER: missing scheme (want https://... or http://...): %q", cfg.Homeserver)
	}

	// The bridge posts to MATRIX_ROOM directly, so it must be a room ID
	// (!...:server), not a Matrix alias (#...:server). Rejecting aliases
	// up front saves a confusing 404 on the first send.
	if !strings.HasPrefix(cfg.Room, "!") {
		return Config{}, fmt.Errorf("MATRIX_ROOM: must be a room ID (start with \"!\"), got %q", cfg.Room)
	}

	if v := get("MATRIX_RATE_MS"); v != "" {
		ms, err := strconv.ParseInt(v, 10, 64)
		if err != nil || ms < 0 {
			return Config{}, fmt.Errorf("MATRIX_RATE_MS: invalid integer %q", v)
		}
		cfg.Rate = time.Duration(ms) * time.Millisecond
	}

	if v := get("MATRIX_DRY_RUN"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return Config{}, fmt.Errorf("MATRIX_DRY_RUN: invalid bool %q", v)
		}
		cfg.DryRun = b
	}

	if v := get("MATRIX_MAX_RETRIES"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return Config{}, fmt.Errorf("MATRIX_MAX_RETRIES: invalid integer %q", v)
		}
		cfg.MaxRetries = n
	}

	if v := get("MATRIX_GRACE_MS"); v != "" {
		ms, err := strconv.ParseInt(v, 10, 64)
		if err != nil || ms < 0 {
			return Config{}, fmt.Errorf("MATRIX_GRACE_MS: invalid integer %q", v)
		}
		cfg.GracePeriod = time.Duration(ms) * time.Millisecond
	}

	// DEBUG is the bridge's verbose-logging knob. MATRIX_DEBUG is the
	// namespaced form; the bare DEBUG env var is honored too so it
	// matches the convention used by the wider ecosystem (and by the
	// logging package, which falls back to DEBUG when MATRIX_DEBUG is
	// unset). Either one being truthy enables DEBUG-level slog output.
	for _, key := range []string{"MATRIX_DEBUG", "DEBUG"} {
		if v := get(key); v != "" {
			b, err := strconv.ParseBool(v)
			if err != nil {
				return Config{}, fmt.Errorf("%s: invalid bool %q", key, v)
			}
			cfg.Debug = cfg.Debug || b
		}
	}

	return cfg, nil
}
