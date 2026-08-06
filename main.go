// Command matrix-log-chat reads syslog lines from stdin, translates each
// line into an emoji-decorated chat message, and forwards it to a Matrix
// room. Configuration is environment-only; run `MATRIX_DRY_RUN=true` to
// preview the rendered messages on stderr without posting anything.
// Set DEBUG=1 (or MATRIX_DEBUG=1) to enable verbose slog output.
package main

import (
	"bufio"
	"log"
	"os"
	"os/signal"
	"syscall"

	"paepcke.de/matrix-log-chat/internal/config"
	"paepcke.de/matrix-log-chat/internal/logging"
	"paepcke.de/matrix-log-chat/internal/matrix"
	"paepcke.de/matrix-log-chat/internal/syslog"
	"paepcke.de/matrix-log-chat/internal/transform"
	"paepcke.de/matrix-log-chat/version"
)

func main() {
	log.SetFlags(log.Ltime | log.Lmsgprefix)
	log.SetPrefix("matrix-log-chat: ")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	// Wire the logger now that we know the DEBUG flag. This must run
	// before any package logs anything meaningful.
	logging.Configure(cfg.Debug)

	logging.L().Info("starting",
		"version", version.Version, "build", version.Build,
		"mode", mode(cfg.DryRun), "debug", cfg.Debug,
	)
	log.Println(transform.Summary(cfg.Homeserver, cfg.User, cfg.Room, cfg.DryRun))

	sender, err := matrix.New(cfg, os.Stderr)
	if err != nil {
		log.Fatalf("startup: %v", err)
	}

	// SIGINT/SIGTERM: stop after the in-flight line finishes. Scanner
	// stops on stdin EOF naturally; the signal path ensures a piped
	// producer that never closes is still interruptible.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-stop
		logging.L().Info("signal received, shutting down", "signal", sig)
		log.Println("shutting down…")
		_ = os.Stdin.Close()
	}()

	scanner := bufio.NewScanner(os.Stdin)
	// Syslog lines can be long (multi-kB stack traces, NDJSON, etc.).
	// Raise the per-line limit well above the default 64 KiB so we do
	// not silently truncate.
	const maxLine = 256 * 1024
	scanner.Buffer(make([]byte, 0, maxLine), maxLine)

	lines := 0
	for scanner.Scan() {
		lines++
		ev := syslog.Parse(scanner.Text())
		logging.L().Debug("parsed",
			"format", ev.Format,
			"facility", ev.FacilityName(),
			"severity", ev.SeverityName(),
			"app", ev.App,
			"host", ev.Hostname,
		)
		sender.Send(transform.ToChat(ev))
	}
	if err := scanner.Err(); err != nil {
		logging.L().Error("scanner error", "err", err)
		log.Printf("stdin: %v", err)
	}
	logging.L().Info("exit", "lines", lines)
}

func mode(dryRun bool) string {
	if dryRun {
		return "dry-run"
	}
	return "live"
}
