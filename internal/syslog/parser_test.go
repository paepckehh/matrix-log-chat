package syslog

import (
	"strings"
	"testing"
	"time"
)

func TestParse_RawLines(t *testing.T) {
	cases := []struct {
		name string
		line string
	}{
		{"empty", ""},
		{"plain text", "hello raw world"},
		{"no leading angle", "not syslog at all"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ev := Parse(c.line)
			if ev.Format != "raw" {
				t.Fatalf("Format = %q, want raw", ev.Format)
			}
			if ev.Facility != FacUser || ev.Severity != SevNotice {
				t.Fatalf("got fac=%d sev=%d, want user/notice", ev.Facility, ev.Severity)
			}
		})
	}
}

func TestParse_EmptyLineDoesNotCarryText(t *testing.T) {
	if ev := Parse(""); ev.Message != "" {
		t.Fatalf("empty line message = %q, want empty", ev.Message)
	}
}

func TestParse_RFC3164(t *testing.T) {
	// <34> is severity=2 (crit), facility=4 (auth): 4*8+2 = 34.
	ev := Parse("<34>Oct 11 22:14:15 myhost sshd[1234]: connection accepted")
	if ev.Format != "3164" {
		t.Fatalf("Format = %q, want 3164", ev.Format)
	}
	if ev.Facility != FacAuth {
		t.Fatalf("Facility = %d, want auth (%d)", ev.Facility, FacAuth)
	}
	if ev.Severity != SevCritical {
		t.Fatalf("Severity = %d, want critical (%d)", ev.Severity, SevCritical)
	}
	if ev.Hostname != "myhost" {
		t.Fatalf("Hostname = %q, want myhost", ev.Hostname)
	}
	if ev.App != "sshd" {
		t.Fatalf("App = %q, want sshd", ev.App)
	}
	if ev.PID != "1234" {
		t.Fatalf("PID = %q, want 1234", ev.PID)
	}
	if ev.Message != "connection accepted" {
		t.Fatalf("Message = %q, want %q", ev.Message, "connection accepted")
	}
	if ev.Timestamp.IsZero() {
		t.Fatal("Timestamp is zero; should be parsed")
	}
}

func TestParse_RFC3164_NoPID(t *testing.T) {
	ev := Parse("<13>Oct 11 22:14:15 host app: message body")
	if ev.Format != "3164" {
		t.Fatalf("Format = %q, want 3164", ev.Format)
	}
	if ev.App != "app" {
		t.Fatalf("App = %q, want app", ev.App)
	}
	if ev.PID != "" {
		t.Fatalf("PID = %q, want empty", ev.PID)
	}
	if ev.Message != "message body" {
		t.Fatalf("Message = %q, want %q", ev.Message, "message body")
	}
}

func TestParse_RFC5424(t *testing.T) {
	ev := Parse("<34>1 2003-10-11T22:14:15.003Z myhost app 1234 msgid - hello world")
	if ev.Format != "5424" {
		t.Fatalf("Format = %q, want 5424", ev.Format)
	}
	if ev.Facility != FacAuth {
		t.Fatalf("Facility = %d, want auth (%d)", ev.Facility, FacAuth)
	}
	if ev.Severity != SevCritical {
		t.Fatalf("Severity = %d, want critical (%d)", ev.Severity, SevCritical)
	}
	if ev.Hostname != "myhost" {
		t.Fatalf("Hostname = %q, want myhost", ev.Hostname)
	}
	if ev.App != "app" {
		t.Fatalf("App = %q, want app", ev.App)
	}
	if ev.PID != "1234" {
		t.Fatalf("PID = %q, want 1234", ev.PID)
	}
	if ev.MsgID != "msgid" {
		t.Fatalf("MsgID = %q, want msgid", ev.MsgID)
	}
	if ev.Structured != "" {
		t.Fatalf("Structured = %q, want empty (was \"-\")", ev.Structured)
	}
	if ev.Message != "hello world" {
		t.Fatalf("Message = %q, want %q", ev.Message, "hello world")
	}
	if ev.Timestamp.IsZero() {
		t.Fatal("Timestamp is zero; should be parsed")
	}
	if _, off := ev.Timestamp.Zone(); off != 0 {
		t.Fatalf("Timestamp zone offset = %d, want 0 (UTC)", off)
	}
}

func TestParse_RFC5424_StructuredData(t *testing.T) {
	ev := Parse(`<34>1 2003-10-11T22:14:15.003Z host app 1 msgid [exampleSDID@32473 iut="3"] event`)
	if ev.Structured != `[exampleSDID@32473 iut="3"]` {
		t.Fatalf("Structured = %q, want preserved", ev.Structured)
	}
	if ev.Message != "event" {
		t.Fatalf("Message = %q, want event", ev.Message)
	}
}

func TestParse_NilValuesMappedToEmpty(t *testing.T) {
	ev := Parse("<34>1 2003-10-11T22:14:15Z - - - - - body")
	if ev.Hostname != "" || ev.App != "" || ev.PID != "" || ev.MsgID != "" {
		t.Fatalf("dashes not normalized: %+v", ev)
	}
	if ev.Message != "body" {
		t.Fatalf("Message = %q, want body", ev.Message)
	}
}

func TestParseTimestamp3164_RollsBackYear(t *testing.T) {
	// Parsing a Dec 31 line in early January must attribute it to the
	// previous year, not the future.
	now := time.Now()
	dec31 := parseTimestamp3164("Dec", "31", "23:59:59")
	want := time.December
	if got := dec31.Month(); got != want {
		t.Fatalf("month = %v, want %v", got, want)
	}
	_ = now
}

func TestFacilityAndSeverityNames(t *testing.T) {
	if got := (Event{Facility: FacKern}).FacilityName(); got != "kern" {
		t.Fatalf("FacilityName(kern) = %q", got)
	}
	if got := (Event{Severity: SevEmergency}).SeverityName(); got != "emerg" {
		t.Fatalf("SeverityName(emerg) = %q", got)
	}
	if got := (Event{Facility: 999}).FacilityName(); got != "unknown" {
		t.Fatalf("FacilityName(unknown) = %q, want unknown", got)
	}
	if got := (Event{Severity: 999}).SeverityName(); got != "unknown" {
		t.Fatalf("SeverityName(unknown) = %q, want unknown", got)
	}
}

func TestParse_LongLine(t *testing.T) {
	// A 256 KiB line should round-trip verbatim through Parse's raw path.
	body := strings.Repeat("x", 200_000)
	ev := Parse(body)
	if ev.Message != body {
		t.Fatalf("raw message truncated: got len %d, want %d", len(ev.Message), len(body))
	}
}
