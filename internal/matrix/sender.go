// Package matrix wraps github.com/matrix-org/gomatrix with the small amount
// of behaviour the bridge needs: a one-shot auth check, rate-limited sends
// with bounded retry, and a dry-run mode that prints to stderr instead.
package matrix

import (
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/matrix-org/gomatrix"

	"paepcke.de/matrix-log-chat/internal/config"
)

// Sender posts lines to a Matrix room. It is safe for sequential use from a
// single goroutine (the main loop); the bridge does not send concurrently.
type Sender struct {
	cli        *gomatrix.Client
	room       string
	rate       time.Duration
	maxRetries int
	grace      time.Duration
	dryRun     bool
	out        io.Writer // destination for dry-run lines; os.Stderr in prod
}

// New constructs a Sender from the given Config and verifies the access
// token by querying the homeserver. In dry-run mode the homeserver is NOT
// contacted: the sender is returned without credentials so the bridge can
// be previewed offline (e.g. piped to `head` against a recorded log).
func New(cfg config.Config, out io.Writer) (*Sender, error) {
	if cfg.DryRun {
		slog.Info("dry-run mode: no network will be used", "room", cfg.Room)
		return &Sender{
			room:   cfg.Room,
			dryRun: true,
			out:    out,
		}, nil
	}

	slog.Info("connecting to homeserver",
		"homeserver", cfg.Homeserver, "user", cfg.User, "room", cfg.Room,
		"rate_ms", cfg.Rate.Milliseconds(),
		"max_retries", cfg.MaxRetries,
		"grace_ms", cfg.GracePeriod.Milliseconds(),
	)
	cli, err := gomatrix.NewClient(cfg.Homeserver, cfg.User, cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("matrix: build client: %w", err)
	}

	// Connectivity + auth sanity. GetOwnDisplayName hits /account and
	// returns 401 on a bad token, which is far more useful than failing
	// on the first real send.
	if _, err := cli.GetOwnDisplayName(); err != nil {
		slog.Error("auth check failed", "err", err)
		return nil, fmt.Errorf("matrix: auth check failed (bad token, user, or homeserver?): %w", err)
	}
	slog.Info("auth check passed", "user", cfg.User)
	return &Sender{
		cli:        cli,
		room:       cfg.Room,
		rate:       cfg.Rate,
		maxRetries: cfg.MaxRetries,
		grace:      cfg.GracePeriod,
		out:        out,
	}, nil
}

// Send posts one line. It blocks until the rate-limit window has elapsed,
// so callers can simply call it in a tight loop. A failed send is retried
// up to maxRetries times with a fixed backoff; if all attempts fail the
// line is logged and dropped (the bridge never blocks on a single bad
// line — a syslog stream must keep flowing).
func (s *Sender) Send(line string) {
	if line == "" {
		return
	}
	if s.dryRun {
		fmt.Fprintln(s.out, line)
		return
	}

	var lastErr error
	for attempt := 0; attempt <= s.maxRetries; attempt++ {
		_, err := s.cli.SendText(s.room, line)
		if err == nil {
			if attempt > 0 {
				slog.Info("send recovered after retry",
					"attempts", attempt+1, "line", truncate(line, 80))
			} else {
				slog.Debug("send ok", "line", truncate(line, 80))
			}
			lastErr = nil
			break
		}
		lastErr = err
		slog.Warn("send attempt failed",
			"attempt", attempt+1, "max", s.maxRetries+1,
			"err", err, "line", truncate(line, 80))
		if attempt < s.maxRetries {
			time.Sleep(s.grace)
		}
	}
	if lastErr != nil {
		slog.Error("send failed after all attempts — line dropped",
			"attempts", s.maxRetries+1, "err", lastErr, "line", truncate(line, 200))
	}
	s.pace()
}

// truncate clips s to n runes for log lines so a multi-kB stack trace
// does not blow up the operator's log aggregator. The trailing "…" marks
// truncation.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// pace sleeps for the configured rate window. This spaces messages apart
// without batching, which is what the Matrix homeserver rate limiter
// expects from a chatty bot.
func (s *Sender) pace() {
	if s.rate > 0 {
		time.Sleep(s.rate)
	}
}
