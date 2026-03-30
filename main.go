package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/matrix-org/gomatrix"
	"github.com/nxadm/tail"
)

func main() {
	// ── CLI flags ──────────────────────────────────────────────────────────────
	filePath   := flag.String("file",       "",            "Path to syslog file to follow (required)")
	homeserver := flag.String("homeserver", "",            "Matrix homeserver URL, e.g. https://matrix.org (required)")
	token      := flag.String("token",      "",            "Matrix access token (required)")
	roomID     := flag.String("room",       "",            "Matrix room ID, e.g. !abc123:matrix.org (required)")
	rateMs     := flag.Int64 ("rate",       300,           "Minimum ms between messages to avoid flooding (default 300)")
	fromStart  := flag.Bool  ("from-start", false,         "Replay existing file content before following new lines")
	flag.Parse()

	// ── Validate required flags ────────────────────────────────────────────────
	missing := []string{}
	if *filePath   == "" { missing = append(missing, "--file") }
	if *homeserver == "" { missing = append(missing, "--homeserver") }
	if *token      == "" { missing = append(missing, "--token") }
	if *roomID     == "" { missing = append(missing, "--room") }
	if len(missing) > 0 {
		fmt.Fprintf(os.Stderr, "Error: missing required flags: %v\n\n", missing)
		flag.Usage()
		os.Exit(1)
	}

	rateLimit := time.Duration(*rateMs) * time.Millisecond

	// ── Matrix client ──────────────────────────────────────────────────────────
	cli, err := gomatrix.NewClient(*homeserver, "", *token)
	if err != nil {
		log.Fatalf("matrix: failed to create client: %v", err)
	}

	// Quick connectivity check — whoami
	resp, err := cli.Whoami()
	if err != nil {
		log.Fatalf("matrix: auth check failed (bad token or homeserver?): %v", err)
	}
	log.Printf("matrix: authenticated as %s", resp.UserID)

	// ── Tail config ────────────────────────────────────────────────────────────
	seekInfo := &tail.SeekInfo{Offset: 0, Whence: io.SeekEnd} // only new lines by default
	if *fromStart {
		seekInfo = &tail.SeekInfo{Offset: 0, Whence: io.SeekStart}
	}

	t, err := tail.TailFile(*filePath, tail.Config{
		Follow:    true,
		ReOpen:    true, // handles log rotation
		MustExist: true,
		Location:  seekInfo,
		Logger:    tail.DiscardingLogger,
	})
	if err != nil {
		log.Fatalf("tail: failed to open %q: %v", *filePath, err)
	}
	defer t.Cleanup()

	log.Printf("following %s → Matrix room %s (rate limit: %s)", *filePath, *roomID, rateLimit)

	// ── Graceful shutdown ──────────────────────────────────────────────────────
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		log.Println("shutting down…")
		t.Stop()
	}()

	// ── Main loop ──────────────────────────────────────────────────────────────
	for line := range t.Lines {
		if line.Err != nil {
			log.Printf("tail error: %v", line.Err)
			continue
		}
		text := line.Text
		if text == "" {
			continue
		}

		if _, err := cli.SendText(*roomID, text); err != nil {
			log.Printf("matrix: send failed: %v — line dropped: %s", err, text)
		}

		time.Sleep(rateLimit) // avoid flooding the room
	}
}
