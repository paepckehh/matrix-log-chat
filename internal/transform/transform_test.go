package transform

import (
	"strings"
	"testing"

	"paepcke.de/matrix-log-chat/internal/syslog"
)

func TestToChat_RawForwardedVerbatim(t *testing.T) {
	ev := syslog.Event{Format: "raw", Message: "hello raw world"}
	if got := ToChat(ev); got != "hello raw world" {
		t.Fatalf("got %q, want verbatim", got)
	}
}

func TestToChat_3164Shape(t *testing.T) {
	ev := syslog.Event{
		Format:   "3164",
		Severity: syslog.SevCritical,
		Facility: syslog.FacAuth,
		App:      "sshd",
		PID:      "1234",
		Message:  "connection accepted",
		Hostname: "myhost",
	}
	got := ToChat(ev)
	// Severity emoji 🟥 and facility 🛡️ should lead.
	if !strings.HasPrefix(got, "🟥") {
		t.Fatalf("missing critical emoji: %q", got)
	}
	if !strings.Contains(got, "🔐") {
		t.Fatalf("missing auth emoji: %q", got)
	}
	if !strings.Contains(got, "<sshd[1234]>") {
		t.Fatalf("missing source block: %q", got)
	}
	if !strings.Contains(got, "connection accepted") {
		t.Fatalf("missing message: %q", got)
	}
	if !strings.Contains(got, "@myhost") {
		t.Fatalf("missing hostname tail: %q", got)
	}
}

func TestToChat_HostnameShownWhenNoApp(t *testing.T) {
	ev := syslog.Event{
		Format:   "3164",
		Severity: syslog.SevInfo,
		Facility: syslog.FacUser,
		Hostname: "onlyhost",
		Message:  "hi",
	}
	got := ToChat(ev)
	if !strings.Contains(got, "<onlyhost>") {
		t.Fatalf("missing hostname as source: %q", got)
	}
}

func TestToChat_NoPIDRendered(t *testing.T) {
	ev := syslog.Event{
		Format:   "3164",
		Severity: syslog.SevInfo,
		Facility: syslog.FacUser,
		App:      "cron",
		Message:  "job done",
		Hostname: "host",
	}
	got := ToChat(ev)
	if !strings.Contains(got, "<cron>") {
		t.Fatalf("missing <cron>: %q", got)
	}
	if strings.Contains(got, "[") {
		t.Fatalf("unexpected pid bracket: %q", got)
	}
}

func TestToChat_UnknownSeverityFallback(t *testing.T) {
	ev := syslog.Event{
		Format:   "3164",
		Severity: 99,
		Facility: syslog.FacUser,
		App:      "x",
		Message:  "m",
		Hostname: "h",
	}
	got := ToChat(ev)
	if !strings.HasPrefix(got, "❔ ") {
		t.Fatalf("expected ❔ fallback, got %q", got)
	}
}

func TestSummary(t *testing.T) {
	got := Summary("https://matrix.org", "@logs:matrix.org", "!room:matrix.org", false)
	if !strings.Contains(got, "[LIVE]") {
		t.Fatalf("missing LIVE tag: %q", got)
	}
	if !strings.Contains(got, "@logs:matrix.org") {
		t.Fatalf("missing user: %q", got)
	}
	gotDry := Summary("https://matrix.org", "@logs:matrix.org", "!room:matrix.org", true)
	if !strings.Contains(gotDry, "[DRY-RUN]") {
		t.Fatalf("missing DRY-RUN tag: %q", gotDry)
	}
}
