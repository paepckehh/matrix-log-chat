// Package syslog parses RFC 3164 (BSD) and RFC 5424 (modern) syslog lines
// received from stdin into structured Events. Lines that do not match either
// format are treated as unstructured user-level notices so the bridge keeps
// working with arbitrary piped text.
package syslog

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Facility codes per RFC 5424 §6.2.1.
const (
	FacKern = iota
	FacUser
	FacMail
	FacDaemon
	FacAuth
	FacSyslog
	FacLPR
	FacNews
	FacUUCP
	FacClock
	FacAuthPriv
	FacFTP
	FacNTP
	FacLogAudit
	FacLogAlert
	FacClock2
	FacLocal0
	FacLocal1
	FacLocal2
	FacLocal3
	FacLocal4
	FacLocal5
	FacLocal6
	FacLocal7
)

// Severity codes per RFC 5424 §6.2.2.
const (
	SevEmergency = iota
	SevAlert
	SevCritical
	SevError
	SevWarning
	SevNotice
	SevInfo
	SevDebug
)

var facilityNames = map[int]string{
	FacKern: "kern", FacUser: "user", FacMail: "mail", FacDaemon: "daemon",
	FacAuth: "auth", FacSyslog: "syslog", FacLPR: "lpr", FacNews: "news",
	FacUUCP: "uucp", FacClock: "clock", FacAuthPriv: "authpriv", FacFTP: "ftp",
	FacNTP: "ntp", FacLogAudit: "logaudit", FacLogAlert: "logalert",
	FacClock2: "clock2", FacLocal0: "local0", FacLocal1: "local1",
	FacLocal2: "local2", FacLocal3: "local3", FacLocal4: "local4",
	FacLocal5: "local5", FacLocal6: "local6", FacLocal7: "local7",
}

var severityNames = map[int]string{
	SevEmergency: "emerg", SevAlert: "alert", SevCritical: "crit",
	SevError: "error", SevWarning: "warning", SevNotice: "notice",
	SevInfo: "info", SevDebug: "debug",
}

// Event is the normalized representation of a single syslog line.
type Event struct {
	Facility   int
	Severity   int
	Timestamp  time.Time // zero value if the line carried no timestamp
	Hostname   string
	App        string
	PID        string
	MsgID      string
	Message    string
	Structured string // RFC 5424 structured data, verbatim (may be "-" for none)
	Format     string // "3164", "5424", or "raw"
}

// FacilityName returns the canonical short name (kern, user, daemon, ...) or
// "localN" for the local-use facilities.
func (e Event) FacilityName() string {
	if n, ok := facilityNames[e.Facility]; ok {
		return n
	}
	return "unknown"
}

// SeverityName returns the canonical short name (emerg, alert, ...).
func (e Event) SeverityName() string {
	if n, ok := severityNames[e.Severity]; ok {
		return n
	}
	return "unknown"
}

// 5424 is detected by a version digit immediately after the priority:
//
//	<34>1 2003-10-11T22:14:15.003Z host app procid msgid [SD] msg
var rfc5424Re = regexp.MustCompile(
	`^<(\d{1,3})>1\s+` + // PRI + version
		`(\S+)\s+` + // timestamp
		`(\S+)\s+` + // hostname
		`(\S+)\s+` + // app
		`(\S+)\s+` + // procid
		`(\S+)\s+` + // msgid
		`(-|\[.*\])\s+` + // structured data
		`(.*)$`, // message
)

// RFC 3164 has no version and a fixed month-day-time stamp:
//
//	<34>Oct 11 22:14:15 host app[pid]: message
var rfc3164Re = regexp.MustCompile(
	`^<(\d{1,3})>` +
		`([A-Z][a-z]{2})\s+(\d{1,2})\s+(\d{2}:\d{2}:\d{2})\s+` + // timestamp
		`(\S+)\s+` + // hostname
		`(.*)$`, // tag + message
)

var tagRe = regexp.MustCompile(`^([^\s\[\]:]+)(?:\[(\d+)\])?:\s*(.*)$`)

// Parse decodes a single line. It never returns an error: any line that does
// not look like syslog becomes a raw user/notice Event so the caller can
// forward it unchanged. The caller is therefore free to range over stdin
// without per-line branching.
func Parse(line string) Event {
	line = strings.TrimRight(line, "\r\n")
	if line == "" {
		return Event{Facility: FacUser, Severity: SevNotice, Message: "", Format: "raw"}
	}

	if m := rfc5424Re.FindStringSubmatch(line); m != nil {
		return parse5424(m)
	}
	if m := rfc3164Re.FindStringSubmatch(line); m != nil {
		return parse3164(m)
	}
	return Event{
		Facility: FacUser,
		Severity: SevNotice,
		Message:  line,
		Format:   "raw",
	}
}

func parse5424(m []string) Event {
	pri, _ := strconv.Atoi(m[1])
	fac, sev := pri/8, pri%8
	ts := parseTimestamp5424(m[2])
	sd := m[7]
	if sd == "-" {
		sd = ""
	}
	return Event{
		Facility:   fac,
		Severity:   sev,
		Timestamp:  ts,
		Hostname:   nilIfDash(m[3]),
		App:        nilIfDash(m[4]),
		PID:        nilIfDash(m[5]),
		MsgID:      nilIfDash(m[6]),
		Structured: sd,
		Message:    m[8],
		Format:     "5424",
	}
}

func parse3164(m []string) Event {
	pri, _ := strconv.Atoi(m[1])
	fac, sev := pri/8, pri%8
	ts := parseTimestamp3164(m[2], m[3], m[4])
	host := m[5]
	rest := m[6]

	app, pid, msg := rest, "", rest
	if t := tagRe.FindStringSubmatch(rest); t != nil {
		app = t[1]
		pid = t[2]
		msg = t[3]
	}

	return Event{
		Facility:  fac,
		Severity:  sev,
		Timestamp: ts,
		Hostname:  host,
		App:       app,
		PID:       pid,
		Message:   msg,
		Format:    "3164",
	}
}

func nilIfDash(s string) string {
	if s == "-" {
		return ""
	}
	return s
}

func parseTimestamp5424(s string) time.Time {
	// RFC 3339-ish; the spec allows a bare "Z" or an offset, and allows
	// omitting fractional seconds. time.Parse handles the common cases;
	// we fall back to leaving a zero value for anything unparseable.
	for _, layout := range []string{
		"2006-01-02T15:04:05.000000Z",
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000000-07:00",
		"2006-01-02T15:04:05.000-07:00",
		"2006-01-02T15:04:05-07:00",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func parseTimestamp3164(month, day, clock string) time.Time {
	// RFC 3164 timestamps carry no year. Assume the current year and,
	// if that lands in the future, roll back one year (syslog lines are
	// never from the future).
	now := time.Now()
	year := now.Year()
	// Normalise day spacing: "Oct  3" -> "Oct 3".
	stamp := strings.TrimSpace(month + " " + day + " " + clock)
	t, err := time.Parse("Jan 2 15:04:05", stamp)
	if err != nil {
		return time.Time{}
	}
	t = t.AddDate(year, 0, 0)
	if t.After(now.Add(24 * time.Hour)) {
		t = t.AddDate(-1, 0, 0)
	}
	return t
}
