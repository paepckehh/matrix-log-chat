package matrix

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"paepcke.de/matrix-log-chat/internal/config"
)

// Dry-run is the only path that does not require a live homeserver, so it
// is the only thing that can be exercised as a unit test without a fake
// Matrix server. It must print the rendered line verbatim to the writer
// and must NOT pace (sleep) — keep the dry-run path side-effect-free.
func TestSender_DryRunPrintsVerbatim(t *testing.T) {
	cfg := config.Config{
		Homeserver: "https://matrix.example.com",
		User:       "@logs:matrix.example.com",
		Token:      "ignored",
		Room:       "!room:matrix.example.com",
		Rate:       1000 * time.Millisecond, // would slow the test if dry-run slept
		DryRun:     true,
	}
	var buf bytes.Buffer
	s, err := New(cfg, &buf)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	s.Send("🟥 🛡️ <sshd[1]> hi @host · 12:00:00")
	got := buf.String()
	if !strings.HasSuffix(got, "\n") {
		t.Fatalf("dry-run output not newline-terminated: %q", got)
	}
	if !strings.Contains(got, "<sshd[1]> hi") {
		t.Fatalf("dry-run mangled the line: %q", got)
	}
}

func TestSender_DryRunSkipsEmpty(t *testing.T) {
	cfg := config.Config{DryRun: true, Room: "!r:s"}
	var buf bytes.Buffer
	s, _ := New(cfg, &buf)
	s.Send("")
	if buf.Len() != 0 {
		t.Fatalf("empty line should be skipped, got %q", buf.String())
	}
}
